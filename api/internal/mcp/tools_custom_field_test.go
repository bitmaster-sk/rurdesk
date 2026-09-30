package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolRequest(name string, arguments map[string]any) mcpgo.CallToolRequest {
	req := mcpgo.CallToolRequest{Header: http.Header{}}
	req.Header.Set("Authorization", "Bearer tok")
	req.Params.Name = name
	req.Params.Arguments = arguments
	return req
}

func TestListCustomFields_CallsDefinitionEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/private/custom-field", func(c *gin.Context) {
		c.String(http.StatusOK, `[{"idCustomField":1,"key":"impact","fieldType":"number"}]`)
	})

	result, err := handleListCustomFields(context.Background(),
		toolRequest("list_custom_fields", map[string]any{}), NewDispatcher(engine))

	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, resultText(t, result), `"key":"impact"`)
}

func TestUpdateIssue_ForwardsCustomFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var captured map[string]any
	engine.PATCH("/api/private/project/:id/issue/:issueId", func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		require.NoError(t, json.Unmarshal(raw, &captured))
		c.String(http.StatusOK, `{"idIssue":5}`)
	})

	_, err := handleUpdateIssue(context.Background(), toolRequest("update_issue", map[string]any{
		"project_id":    float64(1),
		"issue_id":      float64(99),
		"custom_fields": map[string]any{"impact": float64(7)},
	}), NewDispatcher(engine))

	require.NoError(t, err)
	values, ok := captured["customFields"].(map[string]any)
	require.True(t, ok, "update_issue must forward custom_fields as customFields")
	assert.Equal(t, float64(7), values["impact"])
}

func resultText(t *testing.T, result *mcpgo.CallToolResult) string {
	t.Helper()
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(mcpgo.TextContent)
	require.True(t, ok)
	return text.Text
}
