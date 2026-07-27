package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

var projectBuilderTool = Tool{
	Name:        "create_backlog",
	Description: "Create a structured project backlog with issues and schedule relations",
	InputSchema: json.RawMessage(`{
		"type": "object",
		"required": ["issues", "summary"],
		"properties": {
			"summary": { "type": "string" },
			"issues": {
				"type": "array",
				"items": {
					"type": "object",
					"required": ["ref", "title", "description", "estimated_hours"],
					"properties": {
						"ref": { "type": "string" },
						"title": { "type": "string" },
						"description": { "type": "string" },
						"estimated_hours": { "type": "number" },
						"schedule_relations": {
							"type": "array",
							"items": {
								"type": "object",
								"required": ["ref", "type"],
								"properties": {
									"ref": { "type": "string" },
									"type": { "type": "string", "enum": ["finish_to_start", "start_to_start", "finish_to_finish", "start_to_finish"] }
								}
							}
						}
					}
				}
			}
		}
	}`),
}

// BuildProjectBuilderPrompt builds the messages for the project builder completion.
func BuildProjectBuilderPrompt(description string) []Message {
	system := `You are an expert software project manager.
Generate a realistic, actionable project backlog based on the user's project description.

Rules:
- Generate a flat list of tasks. Each task must be small and independently workable.
- Each issue must have a clear, action-oriented title (max 100 characters).
- Each issue must have a detailed, actionable description of at least 100 characters.
- Use schedule_relations to express ordering dependencies between issues.
- Generate as many issues as the project warrants — no artificial cap.
- Assign meaningful estimated_hours (decimal allowed).
- The summary should be 1-3 sentences describing the overall plan.
- Use short, stable string refs like "TASK-1", "TASK-2", etc.`

	user := fmt.Sprintf("System instructions: %s\n\nProject description:\n%s", system, description)

	return []Message{
		{Role: "user", Content: user},
	}
}

// llmIssue is the raw shape parsed from the AI tool_use input.
type llmIssue struct {
	Ref               string        `json:"ref"`
	Title             string        `json:"title"`
	Description       string        `json:"description"`
	EstimatedHours    float64       `json:"estimated_hours"`
	ScheduleRelations []llmRelation `json:"schedule_relations"`
}

type llmRelation struct {
	Ref  string `json:"ref"`
	Type string `json:"type"`
}

type llmBacklog struct {
	Issues  []llmIssue `json:"issues"`
	Summary string     `json:"summary"`
}

// ParseProjectResponse parses the AI completion response and returns a flat slice
// of ProjectBuilderIssue.
func ParseProjectResponse(res *CompletionRes) ([]model.ProjectBuilderIssue, string, error) {
	if isTruncatedStopReason(res.StopReason) {
		return nil, "", fmt.Errorf("parse project response: output truncated by max_tokens limit (stop_reason=%s)", res.StopReason)
	}

	var backlog llmBacklog
	if err := json.Unmarshal(res.ToolUseInput, &backlog); err != nil {
		return nil, "", fmt.Errorf("parse project response: %w", err)
	}

	if len(backlog.Issues) == 0 {
		return nil, "", fmt.Errorf("parse project response: no issues returned")
	}

	refSet := make(map[string]bool, len(backlog.Issues))
	for _, iss := range backlog.Issues {
		if iss.Ref == "" {
			return nil, "", fmt.Errorf("parse project response: issue with empty ref")
		}
		refSet[iss.Ref] = true
	}

	issues := make([]model.ProjectBuilderIssue, 0, len(backlog.Issues))
	for _, iss := range backlog.Issues {
		if iss.Title == "" {
			return nil, "", fmt.Errorf("parse project response: issue %q has empty title", iss.Ref)
		}

		// Filter out schedule relations whose target ref doesn't exist.
		var schedRels []model.ProjectBuilderRelation
		for _, sr := range iss.ScheduleRelations {
			if refSet[sr.Ref] && sr.Ref != iss.Ref {
				schedRels = append(schedRels, model.ProjectBuilderRelation{
					Ref:  sr.Ref,
					Type: sr.Type,
				})
			}
		}

		issues = append(issues, model.ProjectBuilderIssue{
			Ref:               iss.Ref,
			Title:             iss.Title,
			Description:       iss.Description,
			EstimatedMinutes:  int64(iss.EstimatedHours * 60),
			ScheduleRelations: schedRels,
		})
	}

	return issues, backlog.Summary, nil
}

// isTruncatedStopReason reports whether the provider stop reason means the
// output hit the max token limit: openai/ollama "length", anthropic
// "max_tokens", gemini "MAX_TOKENS".
func isTruncatedStopReason(stopReason string) bool {
	switch strings.ToLower(stopReason) {
	case "length", "max_tokens":
		return true
	}
	return false
}

// ProjectBuilderTools returns the tools slice for the project builder completion request.
func ProjectBuilderTools() []Tool {
	return []Tool{projectBuilderTool}
}
