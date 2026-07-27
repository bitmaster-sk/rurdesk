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

func newTestEngine(status int, body string, extraHeaders map[string]string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/test", func(c *gin.Context) {
		for k, v := range extraHeaders {
			c.Header(k, v)
		}
		if body != "" {
			c.String(status, body)
		} else {
			c.Status(status)
		}
	})
	engine.POST("/test", func(c *gin.Context) {
		c.Status(status)
	})
	return engine
}

func TestDispatcher_Success200(t *testing.T) {
	engine := newTestEngine(http.StatusOK, `{"id":1}`, nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/test",
		Bearer:   "Bearer tok123",
		ToolName: "test_tool",
	})

	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"id":1}`, string(resp.Body))
}

// Strips the optional "Bearer " prefix: auth middleware matches keys by their
// 64-hex shape, not "Bearer <key>", so the forwarded header must carry the raw key.
func TestDispatcher_BearerPrefixStripped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedBearer string
	engine.GET("/test", func(c *gin.Context) {
		capturedBearer = c.GetHeader("Authorization")
		c.Status(http.StatusOK)
	})
	d := mcp.NewDispatcher(engine)

	_, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/test",
		Bearer:   "Bearer my-api-key",
		ToolName: "test_tool",
	})

	require.NoError(t, err)
	assert.Equal(t, "my-api-key", capturedBearer)
}

func TestDispatcher_MCPHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var capturedTool, capturedOrigin string
	engine.GET("/test", func(c *gin.Context) {
		capturedTool = c.GetHeader("X-MCP-Tool")
		capturedOrigin = c.GetHeader("X-MCP-Origin")
		c.Status(http.StatusOK)
	})
	d := mcp.NewDispatcher(engine)

	_, err := d.Request(context.Background(), mcp.RequestOpts{
		Method:   "GET",
		Path:     "/test",
		Bearer:   "Bearer tok",
		ToolName: "list_issues",
	})

	require.NoError(t, err)
	assert.Equal(t, "list_issues", capturedTool)
	assert.Equal(t, "mcp", capturedOrigin)
}

func TestDispatcher_400_InvalidRequest(t *testing.T) {
	engine := newTestEngine(http.StatusBadRequest, `{"error":"title required"}`, nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.False(t, resp.IsAuthError)
	assert.Contains(t, resp.ErrorMessage, "invalid request")
}

func TestDispatcher_401_AuthenticationFailed(t *testing.T) {
	engine := newTestEngine(http.StatusUnauthorized, "", nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.True(t, resp.IsAuthError)
	assert.Equal(t, "authentication failed", resp.ErrorMessage)
}

func TestDispatcher_403_Forbidden(t *testing.T) {
	engine := newTestEngine(http.StatusForbidden, "", nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Equal(t, "forbidden", resp.ErrorMessage)
}

func TestDispatcher_404_NotFound(t *testing.T) {
	engine := newTestEngine(http.StatusNotFound, "", nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Equal(t, "not found", resp.ErrorMessage)
}

func TestDispatcher_409_Conflict(t *testing.T) {
	engine := newTestEngine(http.StatusConflict, `{"error":"duplicate key"}`, nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.ErrorMessage, "conflict")
}

func TestDispatcher_422_Unprocessable(t *testing.T) {
	engine := newTestEngine(http.StatusUnprocessableEntity, `{"error":"cycle detected"}`, nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.ErrorMessage, "unprocessable")
}

func TestDispatcher_429_RateLimited(t *testing.T) {
	engine := newTestEngine(http.StatusTooManyRequests, "", map[string]string{"Retry-After": "60"})
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.ErrorMessage, "rate limited")
	assert.Contains(t, resp.ErrorMessage, "60")
}

func TestDispatcher_500_ServerError(t *testing.T) {
	engine := newTestEngine(http.StatusInternalServerError, "", nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Contains(t, resp.ErrorMessage, "server error")
	assert.Contains(t, resp.ErrorMessage, "500")
}

func TestDispatcher_EmptyBody_ReturnsNull(t *testing.T) {
	engine := newTestEngine(http.StatusOK, "", nil)
	d := mcp.NewDispatcher(engine)

	resp, err := d.Request(context.Background(), mcp.RequestOpts{Method: "GET", Path: "/test", Bearer: "Bearer tok", ToolName: "t"})

	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "null", string(resp.Body))
}
