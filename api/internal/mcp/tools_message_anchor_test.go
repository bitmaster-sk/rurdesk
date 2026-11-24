package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingMessageEngine records the body the tool sends to POST /message.
func capturingMessageEngine(t *testing.T) (*gin.Engine, *map[string]any, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	captured := map[string]any{}
	isCalled := false
	engine.POST("/api/private/message", func(c *gin.Context) {
		isCalled = true
		require.NoError(t, json.NewDecoder(c.Request.Body).Decode(&captured))
		c.String(http.StatusOK, `{"idMessage":42}`)
	})
	return engine, &captured, &isCalled
}

func TestPostIssueMessage_PassesQuoteAndThreadThrough(t *testing.T) {
	t.Run("quote with anchor", func(t *testing.T) {
		engine, captured, _ := capturingMessageEngine(t)
		result, err := handlePostIssueMessage(context.Background(), toolRequest("post_issue_message", map[string]any{
			"issue_id": float64(7), "message": "why?",
			"parent_message_id": float64(5), "anchor_line_start": float64(2), "anchor_line_end": float64(3),
			"anchor_quote": "backfill",
		}), NewDispatcher(engine))
		require.NoError(t, err)
		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "backfill", (*captured)["anchorQuote"])
		assert.Equal(t, float64(5), (*captured)["idParentMessage"])
	})

	t.Run("thread reply", func(t *testing.T) {
		engine, captured, _ := capturingMessageEngine(t)
		result, err := handlePostIssueMessage(context.Background(), toolRequest("post_issue_message", map[string]any{
			"issue_id": float64(7), "message": "done", "thread_message_id": float64(13),
		}), NewDispatcher(engine))
		require.NoError(t, err)
		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, float64(13), (*captured)["idThreadMessage"])
		assert.NotContains(t, *captured, "idParentMessage")
	})
}

func TestPostIssueMessage_RejectsInvalidAnchorCombinations(t *testing.T) {
	cases := []struct {
		name      string
		arguments map[string]any
	}{
		{"quote without anchor", map[string]any{"anchor_quote": "backfill"}},
		{"thread with anchor fields", map[string]any{
			"thread_message_id": float64(13),
			"parent_message_id": float64(5), "anchor_line_start": float64(2), "anchor_line_end": float64(3),
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			engine, _, isCalled := capturingMessageEngine(t)
			arguments := map[string]any{"issue_id": float64(7), "message": "x"}
			for key, value := range testCase.arguments {
				arguments[key] = value
			}
			result, err := handlePostIssueMessage(context.Background(), toolRequest("post_issue_message", arguments), NewDispatcher(engine))
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.False(t, *isCalled, "an invalid combination must not reach the API")
		})
	}
}
