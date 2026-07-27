package ai

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	model "github.com/bitmaster-sk/rurdesk/api/internal/model"
)

var splitTool = Tool{
	Name:        "split_result",
	Description: "Split a work item into focused, independent child tasks",
	InputSchema: json.RawMessage(`{
		"type": "object",
		"required": ["children"],
		"properties": {
			"children": {
				"type": "array",
				"minItems": 2,
				"maxItems": 6,
				"items": {
					"type": "object",
					"required": ["title", "description"],
					"properties": {
						"title": { "type": "string", "maxLength": 100 },
						"description": { "type": "string" },
						"estimated_minutes": { "type": "integer", "minimum": 0 }
					}
				}
			}
		}
	}`),
}

// BuildSplitPrompt builds the messages for the issue split completion.
func BuildSplitPrompt(title, description, projectName, hint string, estimatedMinutes int64) []Message {
	estimateRule := "Omit estimated_minutes from all children — the parent has no time estimate."
	if estimatedMinutes > 0 {
		estimateRule = fmt.Sprintf(
			"The parent is estimated at %d minutes. Distribute this proportionally across children. estimated_minutes is required for every child.",
			estimatedMinutes,
		)
	}

	hintRule := ""
	if hint != "" {
		hintRule = fmt.Sprintf("\nSplit strategy hint from the user: %s", hint)
	}

	system := fmt.Sprintf(`You are a senior engineering lead decomposing a work item into child tasks.
Project: %s

Rules:
- Generate 2–6 focused, independent child tasks.
- Each child must be actionable on its own without referencing the parent issue.
- Titles must be clear and specific — do NOT use generic names like "Part 1", "Part 2", etc.
- Prefer fewer, well-scoped children over artificially padded lists.
- %s%s`,
		projectName,
		estimateRule,
		hintRule,
	)

	user := fmt.Sprintf("System instructions: %s\n\nIssue to split:\nTitle: %s\n\nDescription:\n%s", system, title, description)

	return []Message{
		{Role: "user", Content: user},
	}
}

type llmSplitChild struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	EstimatedMinutes *int64 `json:"estimated_minutes"`
}

type llmSplitResult struct {
	Children []llmSplitChild `json:"children"`
}

// ParseSplitResponse parses the AI completion response and returns a slice of model.ProposedIssue.
func ParseSplitResponse(res *CompletionRes) ([]model.ProposedIssue, error) {
	if isTruncatedStopReason(res.StopReason) {
		return nil, fmt.Errorf("split response: output truncated by max_tokens limit (stop_reason=%s)", res.StopReason)
	}

	var result llmSplitResult
	if err := json.Unmarshal(res.ToolUseInput, &result); err != nil {
		return nil, fmt.Errorf("split response: %w", err)
	}

	if len(result.Children) < 2 {
		return nil, fmt.Errorf("split response: too few children")
	}

	if len(result.Children) > 6 {
		return nil, fmt.Errorf("split response: too many children")
	}

	issues := make([]model.ProposedIssue, 0, len(result.Children))
	for i, child := range result.Children {
		if child.Title == "" {
			return nil, fmt.Errorf("split response: child at index %d has empty title", i)
		}

		// Runes, not bytes: len() truncates accented titles at ~half the limit and
		// can slice a rune in half.
		title := child.Title
		if utf8.RuneCountInString(title) > 100 {
			title = string([]rune(title)[:100])
		}

		issues = append(issues, model.ProposedIssue{
			Title:            title,
			Description:      child.Description,
			EstimatedMinutes: child.EstimatedMinutes,
		})
	}

	return issues, nil
}

// SplitTools returns the tools slice for the issue split completion request.
func SplitTools() []Tool {
	return []Tool{splitTool}
}
