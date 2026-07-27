package ai

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSplitPrompt_IncludesParentTitle(t *testing.T) {
	messages := BuildSplitPrompt("Fix login bug", "Some description", "My Project", "", 0)
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].Content, "Fix login bug")
}

func TestBuildSplitPrompt_IncludesHintWhenProvided(t *testing.T) {
	messages := BuildSplitPrompt("Fix login bug", "Some description", "My Project", "focus on frontend", 0)
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0].Content, "focus on frontend")
}

func TestBuildSplitPrompt_OmitsHintWhenEmpty(t *testing.T) {
	messages := BuildSplitPrompt("Fix login bug", "Some description", "My Project", "", 0)
	require.Len(t, messages, 1)
	assert.False(t, strings.Contains(messages[0].Content, "Split strategy hint"), "expected no hint-related text in prompt")
}

func TestParseSplitResponse_ValidChildren(t *testing.T) {
	payload := `{"children":[
		{"title":"Child one","description":"First child description."},
		{"title":"Child two","description":"Second child description."}
	]}`
	res := &CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}

	children, err := ParseSplitResponse(res)
	require.NoError(t, err)
	assert.Len(t, children, 2)
	assert.Equal(t, "Child one", children[0].Title)
	assert.Equal(t, "Child two", children[1].Title)
	assert.Equal(t, "First child description.", children[0].Description)
}

func TestParseSplitResponse_TooFewChildren_ReturnsError(t *testing.T) {
	payload := `{"children":[
		{"title":"Only child","description":"Just one child."}
	]}`
	res := &CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}

	_, err := ParseSplitResponse(res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too few children")
}

func TestParseSplitResponse_TitleTooLong_Truncates(t *testing.T) {
	longTitle := strings.Repeat("a", 101)
	payload := `{"children":[
		{"title":"` + longTitle + `","description":"First."},
		{"title":"Child two","description":"Second."}
	]}`
	res := &CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}

	children, err := ParseSplitResponse(res)
	require.NoError(t, err)
	assert.Len(t, children, 2)
	assert.Equal(t, 100, utf8.RuneCountInString(children[0].Title))
}

// The limit is 100 characters, not 100 bytes.
func TestParseSplitResponse_TitleTooLong_TruncatesByRune(t *testing.T) {
	longTitle := strings.Repeat("ť", 120) // two bytes per rune
	payload := `{"children":[
		{"title":"` + longTitle + `","description":"First."},
		{"title":"Child two","description":"Second."}
	]}`
	res := &CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}

	children, err := ParseSplitResponse(res)
	require.NoError(t, err)
	assert.Equal(t, 100, utf8.RuneCountInString(children[0].Title), "must keep 100 characters")
	assert.True(t, utf8.ValidString(children[0].Title), "must not cut a rune in half")
	assert.Equal(t, strings.Repeat("ť", 100), children[0].Title)
}

func TestParseSplitResponse_TruncatedStopReason_ReturnsError(t *testing.T) {
	res := &CompletionRes{
		ToolUseInput: []byte(`{"children":[{"title":"A","description":"a"},{"title":"B","description":"b"}]}`),
		StopReason:   "max_tokens",
	}
	_, err := ParseSplitResponse(res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "truncated")
}
