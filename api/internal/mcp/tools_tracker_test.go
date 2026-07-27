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

func TestListTrackRecords_NoFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/track", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idTrack":1}]`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/track",
		Bearer:   "Bearer tok",
		ToolName: "list_track_records",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Contains(t, string(resp.Body), "idTrack")
}

func TestListTrackRecords_WithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedProject, capturedIssue string
	engine.GET("/api/private/track", func(c *gin.Context) {
		capturedProject = c.Query("idProject")
		capturedIssue = c.Query("idIssue")
		c.String(http.StatusOK, `[]`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/track?idProject=5&idIssue=10",
		Bearer:   "Bearer tok",
		ToolName: "list_track_records",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "5", capturedProject)
	assert.Equal(t, "10", capturedIssue)
}

func TestDeleteTrack_Returns204AsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.DELETE("/api/private/track/:id", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "DELETE",
		Path:     "/api/private/track/7",
		Bearer:   "Bearer tok",
		ToolName: "delete_track",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestCreateTrack_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/private/track", func(c *gin.Context) {
		c.String(http.StatusCreated, `{"idTrack":42,"tracked":3600}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "POST",
		Path:     "/api/private/track",
		Body:     map[string]any{"idIssue": 5, "tracked": 3600},
		Bearer:   "Bearer tok",
		ToolName: "create_track",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Contains(t, string(resp.Body), "idTrack")
}
