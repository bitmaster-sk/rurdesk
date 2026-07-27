package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/mcp"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListIssueMessages_UsesCorrectRecipientType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedRecipient, capturedType string
	engine.GET("/api/private/message", func(c *gin.Context) {
		capturedRecipient = c.Query("idRecipient")
		capturedType = c.Query("idMessageRecipientType")
		c.String(http.StatusOK, `[]`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/api/private/message?idRecipient=7&idMessageRecipientType=4",
		Bearer:   "Bearer tok",
		ToolName: "list_issue_messages",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "7", capturedRecipient)
	assert.Equal(t, "4", capturedType)
}

func TestPostIssueMessage_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/private/message", func(c *gin.Context) {
		c.String(http.StatusCreated, `{"idMessage":1,"message":"hello"}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "POST",
		Path:     "/api/private/message",
		Body:     map[string]any{"idRecipient": 7, "idMessageRecipientType": 4, "message": "hello"},
		Bearer:   "Bearer tok",
		ToolName: "post_issue_message",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Contains(t, string(resp.Body), "hello")
}

func TestPostIssueMessage_WithAnchor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedBody map[string]any
	engine.POST("/api/private/message", func(c *gin.Context) {
		json.NewDecoder(c.Request.Body).Decode(&capturedBody)
		c.String(http.StatusOK, `{"idMessage":42,"message":"reply"}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method: "POST",
		Path:   "/api/private/message",
		Body: map[string]any{
			"idRecipient":            7,
			"idMessageRecipientType": 4,
			"message":                "reply",
			"idParentMessage":        5,
			"anchorLineStart":        2,
			"anchorLineEnd":          3,
		},
		Bearer:   "Bearer tok",
		ToolName: "post_issue_message",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, float64(5), capturedBody["idParentMessage"])
	assert.Equal(t, float64(2), capturedBody["anchorLineStart"])
	assert.Equal(t, float64(3), capturedBody["anchorLineEnd"])
}

func TestPostIssueMessage_PartialAnchorRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	httpCalled := false
	engine.POST("/api/private/message", func(c *gin.Context) {
		httpCalled = true
		c.String(http.StatusBadRequest, `{"error":"invalid anchor"}`)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method: "POST",
		Path:   "/api/private/message",
		Body: map[string]any{
			"idRecipient":            7,
			"idMessageRecipientType": 4,
			"message":                "reply",
			"idParentMessage":        5,
		},
		Bearer:   "Bearer tok",
		ToolName: "post_issue_message",
	})
	require.NoError(t, err)
	// Validation happens at the controller, so the endpoint is still called;
	// only the response reports the error.
	assert.True(t, httpCalled)
	assert.True(t, resp.IsError)
}

func TestUpdateIssueMessage_403_NotAuthor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.PATCH("/api/private/message/:id", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "PATCH",
		Path:     "/api/private/message/99",
		Body:     map[string]any{"message": "edit"},
		Bearer:   "Bearer tok",
		ToolName: "update_issue_message",
	})
	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Equal(t, "forbidden", resp.ErrorMessage)
}
