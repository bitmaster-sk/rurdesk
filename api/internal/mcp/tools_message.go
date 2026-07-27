package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

// issueRecipientType is the enum value for issue-type message recipients.
const issueRecipientType = 4

func registerMessageTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("list_issue_messages",
			mcpgo.WithDescription("List all messages for an issue. Each message includes a version field and an optional anchor field (idParentMessage, anchorLineStart, anchorLineEnd, isOutdated) for messages that reply to a specific line range of another message."),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Internal issue ID (idIssue)")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListIssueMessages(ctx, req, dispatcher)
		},
	)
	if !allowsWrite(stage) {
		return
	}
	server.AddTool(
		mcpgo.NewTool("post_issue_message",
			mcpgo.WithDescription("Post a message on an issue. Optionally anchor this message to a line range of an earlier message in the thread (e.g. to reply to a specific section of the agent's plan). When replying to a specific part of the user's message or your own earlier plan, prefer anchoring over inline quoting — pass parent_message_id with anchor_line_start and anchor_line_end together."),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Internal issue ID (idIssue)")),
			mcpgo.WithString("message", mcpgo.Required(), mcpgo.Description("Message text")),
			mcpgo.WithNumber("parent_message_id", mcpgo.Description("ID of the message to anchor this reply to. Must be provided together with anchor_line_start and anchor_line_end, or all three must be omitted.")),
			mcpgo.WithNumber("anchor_line_start", mcpgo.Description("First line number (1-based) of the anchor range in the parent message.")),
			mcpgo.WithNumber("anchor_line_end", mcpgo.Description("Last line number (1-based, >= anchor_line_start) of the anchor range in the parent message.")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handlePostIssueMessage(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("update_issue_message",
			mcpgo.WithDescription("Update a message you authored (returns 403 if not the author)"),
			mcpgo.WithNumber("message_id", mcpgo.Required(), mcpgo.Description("Message ID")),
			mcpgo.WithString("message", mcpgo.Required(), mcpgo.Description("New message text")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleUpdateIssueMessage(ctx, req, dispatcher)
		},
	)
}

func handleListIssueMessages(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	path := fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=%d", issueID, issueRecipientType)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "list_issue_messages",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handlePostIssueMessage(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	message := req.GetString("message", "")
	if message == "" {
		return mcpgo.NewToolResultError("message is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	parentMessageID := req.GetInt("parent_message_id", 0)
	anchorLineStart := req.GetInt("anchor_line_start", 0)
	anchorLineEnd := req.GetInt("anchor_line_end", 0)

	hasParent := parentMessageID != 0
	hasStart := anchorLineStart != 0
	hasEnd := anchorLineEnd != 0
	if hasParent != hasStart || hasStart != hasEnd {
		return mcpgo.NewToolResultError("anchor fields parent_message_id, anchor_line_start, and anchor_line_end must all be provided together or all omitted"), nil
	}

	body := map[string]any{
		"idRecipient":            issueID,
		"idMessageRecipientType": issueRecipientType,
		"message":                message,
	}
	if hasParent {
		body["idParentMessage"] = parentMessageID
		body["anchorLineStart"] = anchorLineStart
		body["anchorLineEnd"] = anchorLineEnd
	}

	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "POST",
		Path:     "/api/private/message",
		Body:     body,
		Bearer:   bearer,
		ToolName: "post_issue_message",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleUpdateIssueMessage(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	messageID := req.GetInt("message_id", 0)
	if messageID == 0 {
		return mcpgo.NewToolResultError("message_id is required"), nil
	}
	message := req.GetString("message", "")
	if message == "" {
		return mcpgo.NewToolResultError("message is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	path := fmt.Sprintf("/api/private/message/%d", messageID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "PATCH",
		Path:     path,
		Body:     map[string]any{"message": message},
		Bearer:   bearer,
		ToolName: "update_issue_message",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
