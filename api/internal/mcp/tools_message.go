package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

// issueRecipientType is the enum value for issue-type message recipients.
const issueRecipientType = 4

const taskLinkHint = " Link a task as [#123](/project/{idProject}/issue/123), where {idProject} is the task's project; a bare #123 stays plain text."

func registerMessageTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("list_issue_messages",
			mcpgo.WithDescription("List all messages for an issue. Each message includes a version field and an optional anchor field for a comment that reacts to part of another message: idParentMessage, anchorLineStart and anchorLineEnd (the lines in the parent's current text), anchorText (the anchored source lines, possibly cut), quote (the selected text, if any), idThreadMessage (the thread's first reply; absent on that first reply) and isOutdated (the anchored text no longer exists in the parent)."),
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
			mcpgo.WithDescription("Post a message on an issue. A plain message needs no anchor. Anchor only when reacting to a specific part of a long message: pass parent_message_id with anchor_line_start and anchor_line_end, and optionally anchor_quote. Never anchor a whole message. To answer inside an existing thread, pass thread_message_id (any message of that thread) instead of the anchor fields."),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Internal issue ID (idIssue)")),
			mcpgo.WithString("message", mcpgo.Required(), mcpgo.Description("Message text, Markdown."+taskLinkHint)),
			mcpgo.WithNumber("parent_message_id", mcpgo.Description("ID of the message to anchor this reply to. Must be provided together with anchor_line_start and anchor_line_end, or all three must be omitted.")),
			mcpgo.WithNumber("anchor_line_start", mcpgo.Description("First line number (1-based) of the anchor range in the parent message.")),
			mcpgo.WithNumber("anchor_line_end", mcpgo.Description("Last line number (1-based, >= anchor_line_start) of the anchor range in the parent message.")),
			mcpgo.WithString("anchor_quote", mcpgo.Description("Exact text within the anchored lines this reply is about, at most 2000 characters. Optional; needs the anchor fields.")),
			mcpgo.WithNumber("thread_message_id", mcpgo.Description("ID of a message in the thread to continue. The reply takes that thread's anchor. Do not combine with the anchor fields.")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handlePostIssueMessage(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("update_issue_message",
			mcpgo.WithDescription("Update a message you authored (returns 403 if not the author)"),
			mcpgo.WithNumber("message_id", mcpgo.Required(), mcpgo.Description("Message ID")),
			mcpgo.WithString("message", mcpgo.Required(), mcpgo.Description("New message text, Markdown."+taskLinkHint)),
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
	anchorQuote := req.GetString("anchor_quote", "")
	if anchorQuote != "" && !hasParent {
		return mcpgo.NewToolResultError("anchor_quote needs parent_message_id, anchor_line_start and anchor_line_end"), nil
	}
	threadMessageID := req.GetInt("thread_message_id", 0)
	if threadMessageID != 0 && hasParent {
		return mcpgo.NewToolResultError("thread_message_id cannot be combined with the anchor fields; the reply takes the thread's anchor"), nil
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
	if anchorQuote != "" {
		body["anchorQuote"] = anchorQuote
	}
	if threadMessageID != 0 {
		body["idThreadMessage"] = threadMessageID
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
