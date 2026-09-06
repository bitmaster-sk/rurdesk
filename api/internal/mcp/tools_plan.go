package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

// registerPlanTools registers the sole bot-only tool that ends a stage
// attempt. Available in every stage subset since every productive stage
// completes through it.
func registerPlanTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("complete_stage",
			mcpgo.WithDescription(
				"End the current stage attempt. Required exactly once per stage_execute event. The outcome dictates the next state: output_submitted records a message and either moves to approval (design / implementation_plan) or pr_open (implementation); question_asked posts a brainstorming question and waits for the user; no_action_needed records that no clarification or output was necessary and the pipeline moves forward; errored marks the attempt failed so the user can decide Continue / Restart. The server validates outcome ↔ stage compatibility (e.g. question_asked is only valid in brainstorming).",
			),
			mcpgo.WithNumber("id_task", mcpgo.Required(), mcpgo.Description("The id_task from the stage_execute event payload.")),
			mcpgo.WithString("outcome", mcpgo.Required(), mcpgo.Description("One of: output_submitted, question_asked, no_action_needed, errored.")),
			mcpgo.WithString("message", mcpgo.Description("Output message body, plain markdown. Required when outcome=output_submitted or question_asked.")),
			mcpgo.WithString("message_kind", mcpgo.Description("Required when message is set. One of: brainstorming_question, brainstorming_complete, design, implementation_plan, pull_request_pushed, implementation_done, review_reply. Use review_reply to answer a review comment without changing code — submit it with no branch_name and no PR is opened.")),
			mcpgo.WithString("pr_url", mcpgo.Description("DEPRECATED — do not set. The tracker opens the PR/MR itself from branch_name. Kept only for backward compatibility.")),
			mcpgo.WithString("branch_name", mcpgo.Description("Branch you pushed to. Set only on implementation outcome=output_submitted — the tracker opens the PR/MR from it.")),
			mcpgo.WithString("pr_title", mcpgo.Description("Title for the PR/MR the tracker will open. Set on implementation outcome=output_submitted; falls back to the issue title if empty.")),
			mcpgo.WithString("pr_body", mcpgo.Description("Markdown body/description for the PR/MR the tracker will open. Set on implementation outcome=output_submitted.")),
			mcpgo.WithNumber("tokens_used", mcpgo.Description("Tokens used by this stage attempt.")),
			mcpgo.WithNumber("duration_ms", mcpgo.Description("Wall time of this stage attempt in milliseconds.")),
			mcpgo.WithNumber("tool_calls", mcpgo.Description("Tool calls made by this stage attempt.")),
			mcpgo.WithString("error_reason", mcpgo.Description("Short machine-friendly reason. Required when outcome=errored.")),
			mcpgo.WithString("error_detail", mcpgo.Description("Free-form human-readable detail. Optional.")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleCompleteStage(ctx, req, dispatcher)
		},
	)
	_ = stage // both subsets get the same tool — keep param for signature parity
}

func handleCompleteStage(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	idTask := req.GetInt("id_task", 0)
	if idTask == 0 {
		return mcpgo.NewToolResultError("id_task is required"), nil
	}
	outcome := req.GetString("outcome", "")
	if outcome == "" {
		return mcpgo.NewToolResultError("outcome is required"), nil
	}

	body := map[string]any{"outcome": outcome}
	stringMappings := map[string]string{
		"message":      "message",
		"message_kind": "messageKind",
		"pr_url":       "prUrl",
		"branch_name":  "branchName",
		"pr_title":     "prTitle",
		"pr_body":      "prBody",
		"error_reason": "errorReason",
		"error_detail": "errorDetail",
	}
	for snake, camel := range stringMappings {
		if v := req.GetString(snake, ""); v != "" {
			body[camel] = v
		}
	}
	intMappings := map[string]string{
		"tokens_used": "tokensUsed",
		"duration_ms": "durationMs",
		"tool_calls":  "toolCallsCount",
	}
	for snake, camel := range intMappings {
		if v := req.GetInt(snake, -1); v >= 0 {
			body[camel] = v
		}
	}

	bearer := req.Header.Get("Authorization")
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "POST",
		Path:     fmt.Sprintf("/api/private/agent/task/%d/complete", idTask),
		Body:     body,
		Bearer:   bearer,
		ToolName: "complete_stage",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
