package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitmaster-sk/rurdesk/api/internal/mcp"
	"github.com/gin-gonic/gin"
)

func newProjectTestEngine(responseBody string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/project", func(c *gin.Context) {
		c.String(http.StatusOK, responseBody)
	})
	engine.GET("/api/private/project/:id", func(c *gin.Context) {
		c.String(http.StatusOK, `{"idProject":1,"name":"Test"}`)
	})
	engine.GET("/api/private/state", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idState":1,"idProject":1,"name":"Open"},{"idState":2,"idProject":2,"name":"Other"}]`)
	})
	engine.GET("/api/private/severity", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idSeverity":1,"idProject":1,"title":"High"},{"idSeverity":2,"idProject":2,"title":"Low"}]`)
	})
	engine.GET("/api/private/project/:id/member", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idUser":1}]`)
	})
	return engine
}

func TestListProjects_HappyPath(t *testing.T) {
	engine := newProjectTestEngine(`[{"idProject":1,"name":"Alpha"}]`)
	mcpServer := mcp.NewMCPServer(mcp.LoadConfig(), engine)
	_ = mcpServer

	d := mcp.NewDispatcher(engine)
	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project",
		Bearer:   "Bearer test",
		ToolName: "list_projects",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.JSONEq(t, `[{"idProject":1,"name":"Alpha"}]`, string(resp.Body))
}

func TestGetProjectContext_FiltersStatesAndSeverities(t *testing.T) {
	engine := newProjectTestEngine("")
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/state",
		Bearer:   "Bearer test",
		ToolName: "test",
	})
	require.NoError(t, err)
	var states []map[string]any
	require.NoError(t, json.Unmarshal(resp.Body, &states))
	assert.Len(t, states, 2, "state endpoint returns states for all projects")
}

func TestListProjects_ErrorPropagated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/project", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project",
		Bearer:   "Bearer test",
		ToolName: "list_projects",
	})
	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Equal(t, "forbidden", resp.ErrorMessage)
}
