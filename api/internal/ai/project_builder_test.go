package ai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProjectResponse_TruncatedStopReason(t *testing.T) {
	validInput := json.RawMessage(`{"summary":"s","issues":[{"ref":"T-1","title":"Task","description":"d","estimated_hours":2}]}`)

	truncatedReasons := []string{"length", "max_tokens", "MAX_TOKENS"}
	for _, reason := range truncatedReasons {
		t.Run(reason+" fails", func(t *testing.T) {
			res := &CompletionRes{ToolUseInput: validInput, StopReason: reason}
			_, _, err := ParseProjectResponse(res)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "truncated")
		})
	}

	okReasons := []string{"stop", "tool_use", "tool_calls", ""}
	for _, reason := range okReasons {
		t.Run(reason+" passes", func(t *testing.T) {
			res := &CompletionRes{ToolUseInput: validInput, StopReason: reason}
			issues, summary, err := ParseProjectResponse(res)
			require.NoError(t, err)
			assert.Equal(t, "s", summary)
			require.Len(t, issues, 1)
			assert.Equal(t, int64(120), issues[0].EstimatedMinutes)
		})
	}
}
