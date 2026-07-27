package mcp_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/mcp"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRelations_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/project/:id/issue/:issueId/relation", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idIssueRelation":1,"relationType":"hierarchy"}]`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project/1/issue/5/relation",
		Bearer:   "Bearer tok",
		ToolName: "list_relations",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Contains(t, string(resp.Body), "hierarchy")
}

func TestRemoveRelation_Returns204AsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.DELETE("/api/private/project/:id/issue/:issueId/relation/:relId", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "DELETE",
		Path:     "/api/private/project/1/issue/5/relation/10",
		Bearer:   "Bearer tok",
		ToolName: "remove_relation",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestAddRelation_409_CycleDetected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/private/project/:id/issue/:issueId/relation", func(c *gin.Context) {
		c.String(http.StatusConflict, `{"error":"cycle detected"}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "POST",
		Path:     "/api/private/project/1/issue/5/relation",
		Body:     map[string]any{"idIssuePublicTo": 3, "relationType": "hierarchy"},
		Bearer:   "Bearer tok",
		ToolName: "add_relation",
	})
	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.ErrorMessage, "conflict")
	assert.Contains(t, resp.ErrorMessage, "cycle detected")
}
