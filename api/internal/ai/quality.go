package ai

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

var qualityTool = Tool{
	Name:        "quality_report",
	Description: "Evaluate the quality of an issue ticket and return a structured quality report",
	InputSchema: json.RawMessage(`{
		"type": "object",
		"required": ["score", "dimensions", "problems", "suggestions"],
		"properties": {
			"score": { "type": "integer", "minimum": 0, "maximum": 100 },
			"dimensions": {
				"type": "object",
				"required": ["clarity", "completeness", "actionability", "scope", "metadata"],
				"properties": {
					"clarity":       { "type": "integer", "minimum": 0, "maximum": 100 },
					"completeness":  { "type": "integer", "minimum": 0, "maximum": 100 },
					"actionability": { "type": "integer", "minimum": 0, "maximum": 100 },
					"scope":         { "type": "integer", "minimum": 0, "maximum": 100 },
					"metadata":      { "type": "integer", "minimum": 0, "maximum": 100 }
				}
			},
			"problems": {
				"type": "array",
				"maxItems": 5,
				"items": { "type": "string", "maxLength": 100 }
			},
			"suggestions": {
				"type": "array",
				"maxItems": 3,
				"items": {
					"type": "object",
					"required": ["type", "explanation"],
					"properties": {
						"type":        { "type": "string" },
						"explanation": { "type": "string" },
						"new_value":   { "type": "string" }
					}
				}
			}
		}
	}`),
}

func boolToSet(v bool) string {
	if v {
		return "set"
	}
	return "not set"
}

// BuildQualityPrompt builds the messages for the quality check completion.
func BuildQualityPrompt(title, description, projectName string, hasAssignee, hasSeverity, hasState, hasEstimated bool) []Message {
	metaContext := fmt.Sprintf(
		"Metadata from the issue tracker: severity=%s, state=%s, assignee=%s, estimation=%s.",
		boolToSet(hasSeverity), boolToSet(hasState), boolToSet(hasAssignee), boolToSet(hasEstimated),
	)

	system := fmt.Sprintf(`You are an engineering team lead reviewing issue tickets for quality.
Project: %s

Evaluate the issue on these dimensions (each 0–100):
- Clarity (25%%): Is the title clear and specific? Does the description explain what needs to be done?
- Completeness (25%%): Are acceptance criteria present? Are steps to reproduce included for bugs?
- Actionability (20%%): Can a developer pick this up immediately? Are dependencies identified?
- Scope (15%%): Is the issue focused on one thing? No multi-part "and also" issues.
- Metadata (15%%): Is severity set? Is it estimated? Is it assigned? Use the metadata values provided below — do not infer them from the text.

Overall score = round(clarity*0.25 + completeness*0.25 + actionability*0.20 + scope*0.15 + metadata*0.15). You MUST always include score.

Rules:
- Be concise and direct — no verbose academic analysis.
- You MUST always include all required fields: score, dimensions, problems, suggestions. Problems and suggestions may be empty arrays.
- Only report problems that materially lower a dimension score. If the issue is genuinely good, return empty problems and suggestions — do not invent nitpicks.
- Problems: at most 5, each at most 100 characters. Fewer is better — list only real problems, never pad the list.
- Suggestions: at most 3. Only suggest changes that clearly improve the issue.
- Only suggest rewrite_title or rewrite_description when you have a clearly better version.
- new_value is required for rewrite_title, rewrite_description, and add_section types.
- Every suggestion MUST include both a "type" and a one-sentence "explanation" of why the change is needed. Never omit "explanation".
%s`, projectName, metaContext)

	user := fmt.Sprintf("%s\n\nIssue to evaluate:\nTitle: %s\n\nDescription:\n%s", system, title, description)

	return []Message{
		{Role: "user", Content: user},
	}
}

type llmQualityDimensions struct {
	Clarity       int `json:"clarity"`
	Completeness  int `json:"completeness"`
	Actionability int `json:"actionability"`
	Scope         int `json:"scope"`
	Metadata      int `json:"metadata"`
}

type llmQualitySuggestion struct {
	Type        string `json:"type"`
	Explanation string `json:"explanation"`
	NewValue    string `json:"new_value"`
}

type llmQualityResult struct {
	Score       int                    `json:"score"`
	Dimensions  llmQualityDimensions   `json:"dimensions"`
	Problems    []string               `json:"problems"`
	Suggestions []llmQualitySuggestion `json:"suggestions"`
}

// defaultSuggestionExplanation returns a fallback explanation for a suggestion type
// when the model omits one.
func defaultSuggestionExplanation(suggestionType string) string {
	switch suggestionType {
	case "rewrite_title":
		return "Rewrite the title to be clearer and more specific."
	case "rewrite_description":
		return "Rewrite the description to explain what needs to be done."
	case "add_section":
		return "Add the missing section to make the issue more complete."
	default:
		return "Improve the issue based on this suggestion."
	}
}

// ParseQualityResponse parses the AI completion response into a QualityCheckRes.
func ParseQualityResponse(res *CompletionRes) (*model.QualityCheckRes, error) {
	if isTruncatedStopReason(res.StopReason) {
		return nil, fmt.Errorf("quality response: output truncated by max_tokens limit (stop_reason=%s)", res.StopReason)
	}

	var result llmQualityResult
	if err := json.Unmarshal(res.ToolUseInput, &result); err != nil {
		return nil, fmt.Errorf("quality response: %w", err)
	}

	if result.Score == 0 {
		d := result.Dimensions
		result.Score = int(math.Round(float64(d.Clarity)*0.25 + float64(d.Completeness)*0.25 + float64(d.Actionability)*0.20 + float64(d.Scope)*0.15 + float64(d.Metadata)*0.15))
	}

	if result.Score < 0 || result.Score > 100 {
		return nil, fmt.Errorf("quality response: score %d out of range", result.Score)
	}

	suggestions := make([]model.QualitySuggestion, 0, len(result.Suggestions))
	for _, s := range result.Suggestions {
		// Type is essential — without it the suggestion is not actionable, so skip it.
		if s.Type == "" {
			continue
		}
		// Some models omit explanation despite the schema requiring it; fall back to a
		// type-derived default rather than failing the whole request.
		explanation := s.Explanation
		if explanation == "" {
			explanation = defaultSuggestionExplanation(s.Type)
		}
		suggestions = append(suggestions, model.QualitySuggestion{
			Type:        s.Type,
			Explanation: explanation,
			NewValue:    s.NewValue,
		})
	}

	problems := result.Problems
	if problems == nil {
		problems = []string{}
	}

	return &model.QualityCheckRes{
		Score: result.Score,
		Dimensions: model.QualityDimensions{
			Clarity:       result.Dimensions.Clarity,
			Completeness:  result.Dimensions.Completeness,
			Actionability: result.Dimensions.Actionability,
			Scope:         result.Dimensions.Scope,
			Metadata:      result.Dimensions.Metadata,
		},
		Problems:    problems,
		Suggestions: suggestions,
	}, nil
}

// QualityTools returns the tools slice for the quality check completion request.
func QualityTools() []Tool {
	return []Tool{qualityTool}
}
