package mcp

import (
	"context"
	"fmt"
	"net/http"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerRelationTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("list_relations",
			mcpgo.WithDescription("List all relations for an issue"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListRelations(ctx, req, dispatcher)
		},
	)
	if !allowsWrite(stage) {
		return
	}
	server.AddTool(
		mcpgo.NewTool("add_relation",
			mcpgo.WithDescription("Add a relation between two issues (hierarchy, schedule, duplicates, relates_to)"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("from_issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID of source issue")),
			mcpgo.WithNumber("to_issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID of target issue")),
			mcpgo.WithString("relation_type", mcpgo.Required(), mcpgo.Description("Relation type: hierarchy, schedule, duplicates, relates_to")),
			mcpgo.WithString("relation_sub_type", mcpgo.Description("Sub-type for schedule: finish_to_start, start_to_start, finish_to_finish, start_to_finish")),
			mcpgo.WithNumber("lag_minutes", mcpgo.Description("Lag in minutes (for schedule relations)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleAddRelation(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("update_relation",
			mcpgo.WithDescription("Update lag_minutes on a schedule relation"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID")),
			mcpgo.WithNumber("relation_id", mcpgo.Required(), mcpgo.Description("Relation ID")),
			mcpgo.WithNumber("lag_minutes", mcpgo.Description("New lag in minutes (null to clear)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleUpdateRelation(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("remove_relation",
			mcpgo.WithDescription("Remove a relation between issues"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID")),
			mcpgo.WithNumber("relation_id", mcpgo.Required(), mcpgo.Description("Relation ID")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleRemoveRelation(ctx, req, dispatcher)
		},
	)
}

func handleListRelations(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	path := fmt.Sprintf("/api/private/project/%d/issue/%d/relation", projectID, issueID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "list_relations",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleAddRelation(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	fromIssueID := req.GetInt("from_issue_id", 0)
	if fromIssueID == 0 {
		return mcpgo.NewToolResultError("from_issue_id is required"), nil
	}
	toIssueID := req.GetInt("to_issue_id", 0)
	if toIssueID == 0 {
		return mcpgo.NewToolResultError("to_issue_id is required"), nil
	}
	relationType := req.GetString("relation_type", "")
	if relationType == "" {
		return mcpgo.NewToolResultError("relation_type is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	body := map[string]any{
		"idIssuePublicTo": toIssueID,
		"relationType":    relationType,
	}
	if subType := req.GetString("relation_sub_type", ""); subType != "" {
		body["relationSubType"] = subType
	}
	args := req.GetArguments()
	if _, ok := args["lag_minutes"]; ok {
		body["lagMinutes"] = req.GetInt("lag_minutes", 0)
	}

	path := fmt.Sprintf("/api/private/project/%d/issue/%d/relation", projectID, fromIssueID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "POST",
		Path:     path,
		Body:     body,
		Bearer:   bearer,
		ToolName: "add_relation",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleUpdateRelation(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	relationID := req.GetInt("relation_id", 0)
	if relationID == 0 {
		return mcpgo.NewToolResultError("relation_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	args := req.GetArguments()
	body := map[string]any{}
	if _, ok := args["lag_minutes"]; ok {
		lagMinutes := req.GetInt("lag_minutes", 0)
		body["lagMinutes"] = lagMinutes
	}

	path := fmt.Sprintf("/api/private/project/%d/issue/%d/relation/%d", projectID, issueID, relationID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "PATCH",
		Path:     path,
		Body:     body,
		Bearer:   bearer,
		ToolName: "update_relation",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleRemoveRelation(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	relationID := req.GetInt("relation_id", 0)
	if relationID == 0 {
		return mcpgo.NewToolResultError("relation_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	path := fmt.Sprintf("/api/private/project/%d/issue/%d/relation/%d", projectID, issueID, relationID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "DELETE",
		Path:     path,
		Bearer:   bearer,
		ToolName: "remove_relation",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	if resp.StatusCode == http.StatusNoContent {
		return mcpgo.NewToolResultText(`{"success":true}`), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
