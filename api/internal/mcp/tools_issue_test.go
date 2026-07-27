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

func TestListIssues_BuildsCorrectPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedPath, capturedQuery string
	engine.GET("/api/private/project/:id/issue", func(c *gin.Context) {
		capturedPath = c.Param("id")
		capturedQuery = c.Request.URL.RawQuery
		c.String(http.StatusOK, `[]`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project/42/issue?limit=50&excludeFinalStates=true",
		Bearer:   "Bearer tok",
		ToolName: "list_issues",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "42", capturedPath)
	assert.Contains(t, capturedQuery, "limit=50")
	assert.Contains(t, capturedQuery, "excludeFinalStates=true")
}

func TestGetIssue_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/project/:id/issue/:issueId", func(c *gin.Context) {
		c.String(http.StatusOK, `{"idIssue":5,"idIssuePublic":99,"title":"Bug"}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project/1/issue/99",
		Bearer:   "Bearer tok",
		ToolName: "get_issue",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Contains(t, string(resp.Body), `"title":"Bug"`)
}

func TestCreateIssue_With_IdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedKey string
	engine.POST("/api/private/project/:id/issue", func(c *gin.Context) {
		capturedKey = c.GetHeader("Idempotency-Key")
		c.String(http.StatusCreated, `{"idIssue":1}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:         "POST",
		Path:           "/api/private/project/1/issue",
		Body:           map[string]any{"idProject": 1, "title": "Test"},
		Bearer:         "Bearer tok",
		ToolName:       "create_issue",
		IdempotencyKey: "unique-key-123",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "unique-key-123", capturedKey)
}

func TestListIssues_SearchParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedSearch string
	engine.GET("/api/private/project/:id/issue", func(c *gin.Context) {
		capturedSearch = c.Query("search")
		c.String(http.StatusOK, `[]`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project/1/issue?search=fixme&limit=20",
		Bearer:   "Bearer tok",
		ToolName: "search_issues",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "fixme", capturedSearch)
}

func TestBulkUpdateIssues_DispatchesBatchEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.PATCH("/api/private/project/:id/issue/batch", func(c *gin.Context) {
		c.String(http.StatusOK, `{"updated":3}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "PATCH",
		Path:     "/api/private/project/1/issue/batch",
		Body:     map[string]any{"issues": []map[string]any{{"idIssuePublic": 1}}},
		Bearer:   "Bearer tok",
		ToolName: "bulk_update_issues",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.JSONEq(t, `{"updated":3}`, string(resp.Body))
}
