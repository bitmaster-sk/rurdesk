package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQualityPrompt_IncludesTitle(t *testing.T) {
	msgs := ai.BuildQualityPrompt("Fix login redirect on timeout", "Detailed description here", "MyProject", false, false, false, false)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "Fix login redirect on timeout")
}

func TestBuildQualityPrompt_MetadataAllUnset(t *testing.T) {
	msgs := ai.BuildQualityPrompt("Title", "Desc", "Project", false, false, false, false)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "assignee=not set")
	assert.Contains(t, msgs[0].Content, "severity=not set")
	assert.Contains(t, msgs[0].Content, "state=not set")
	assert.Contains(t, msgs[0].Content, "estimation=not set")
}

func TestBuildQualityPrompt_MetadataAllSet(t *testing.T) {
	msgs := ai.BuildQualityPrompt("Title", "Desc", "Project", true, true, true, true)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "assignee=set")
	assert.Contains(t, msgs[0].Content, "severity=set")
	assert.Contains(t, msgs[0].Content, "state=set")
	assert.Contains(t, msgs[0].Content, "estimation=set")
}

func TestParseQualityResponse_ValidJSON(t *testing.T) {
	payload := `{
		"score": 75,
		"dimensions": {"clarity": 80, "completeness": 70, "actionability": 75, "scope": 90, "metadata": 60},
		"problems": ["Missing acceptance criteria"],
		"suggestions": [{"type": "add_section", "explanation": "Add AC", "new_value": "## Acceptance Criteria\n- ..."}]
	}`
	res := &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}

	report, err := ai.ParseQualityResponse(res)
	require.NoError(t, err)
	assert.Equal(t, 75, report.Score)
	assert.Equal(t, 80, report.Dimensions.Clarity)
	assert.Equal(t, 70, report.Dimensions.Completeness)
	assert.Len(t, report.Problems, 1)
	assert.Len(t, report.Suggestions, 1)
	assert.Equal(t, "add_section", report.Suggestions[0].Type)
	assert.Equal(t, "## Acceptance Criteria\n- ...", report.Suggestions[0].NewValue)
}

func TestParseQualityResponse_ScoreOutOfRange_ReturnsError(t *testing.T) {
	payload := `{
		"score": 150,
		"dimensions": {"clarity": 80, "completeness": 70, "actionability": 75, "scope": 90, "metadata": 60},
		"problems": [],
		"suggestions": []
	}`
	res := &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}

	_, err := ai.ParseQualityResponse(res)
	assert.Error(t, err)
}

func TestParseQualityResponse_InvalidJSON_ReturnsError(t *testing.T) {
	res := &ai.CompletionRes{ToolUseInput: json.RawMessage(`not-json`), StopReason: "tool_use"}
	_, err := ai.ParseQualityResponse(res)
	assert.Error(t, err)
}

func TestParseQualityResponse_SuggestionMissingType_IsSkipped(t *testing.T) {
	payload := `{
		"score": 60,
		"dimensions": {"clarity": 60, "completeness": 60, "actionability": 60, "scope": 60, "metadata": 60},
		"problems": [],
		"suggestions": [
			{"type": "", "explanation": "some explanation"},
			{"type": "rewrite_title", "explanation": "clearer title", "new_value": "Better title"}
		]
	}`
	res := &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
	report, err := ai.ParseQualityResponse(res)
	require.NoError(t, err)
	require.Len(t, report.Suggestions, 1)
	assert.Equal(t, "rewrite_title", report.Suggestions[0].Type)
}

func TestParseQualityResponse_SuggestionMissingExplanation_UsesFallback(t *testing.T) {
	// Some models (e.g. kimi via OpenAI-compatible providers) omit explanation
	// despite the schema requiring it — the request must not fail.
	payload := `{
		"score": 5,
		"dimensions": {"clarity": 5, "completeness": 0, "actionability": 5, "scope": 20, "metadata": 0},
		"problems": ["Title is vague"],
		"suggestions": [{"type": "rewrite_title", "new_value": "Update primary UI color"}]
	}`
	res := &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
	report, err := ai.ParseQualityResponse(res)
	require.NoError(t, err)
	require.Len(t, report.Suggestions, 1)
	assert.Equal(t, "rewrite_title", report.Suggestions[0].Type)
	assert.NotEmpty(t, report.Suggestions[0].Explanation)
	assert.Equal(t, "Update primary UI color", report.Suggestions[0].NewValue)
}

func TestParseQualityResponse_NilProblemsBecomesEmptySlice(t *testing.T) {
	payload := `{
		"score": 80,
		"dimensions": {"clarity": 80, "completeness": 80, "actionability": 80, "scope": 80, "metadata": 80},
		"problems": null,
		"suggestions": []
	}`
	res := &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
	report, err := ai.ParseQualityResponse(res)
	require.NoError(t, err)
	assert.NotNil(t, report.Problems)
	assert.Empty(t, report.Problems)
}

func TestParseQualityResponse_TruncatedStopReason_ReturnsError(t *testing.T) {
	res := &ai.CompletionRes{
		ToolUseInput: []byte(`{"score":80,"dimensions":{"clarity":80,"completeness":80,"actionability":80,"scope":80,"metadata":80},"problems":[],"suggestions":[]}`),
		StopReason:   "length",
	}
	_, err := ai.ParseQualityResponse(res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "truncated")
}
