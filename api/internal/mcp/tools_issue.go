package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerIssueTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("list_issues",
			mcpgo.WithDescription("List issues in a project with optional filters"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("title_search", mcpgo.Description("Filter by title substring (ILIKE)")),
			mcpgo.WithArray("state_ids", mcpgo.Description("Filter by state IDs"), mcpgo.WithNumberItems()),
			mcpgo.WithArray("severity_ids", mcpgo.Description("Filter by severity IDs"), mcpgo.WithNumberItems()),
			mcpgo.WithArray("assigned_to_ids", mcpgo.Description("Filter by assigned user IDs"), mcpgo.WithNumberItems()),
			mcpgo.WithBoolean("exclude_final_states", mcpgo.Description("Exclude issues in final/terminal states")),
			mcpgo.WithNumber("limit", mcpgo.Description("Max results (default 50, max 200)")),
			mcpgo.WithNumber("offset", mcpgo.Description("Skip N results")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListIssues(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("search_issues",
			mcpgo.WithDescription("Full-text search across issue titles and descriptions"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Search query")),
			mcpgo.WithNumber("limit", mcpgo.Description("Max results (default 20, max 100)")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleSearchIssues(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("get_my_issues",
			mcpgo.WithDescription("List non-final issues assigned to the caller (the bot's current workload)"),
			mcpgo.WithNumber("project_id", mcpgo.Description("Optional project filter")),
			mcpgo.WithNumber("limit", mcpgo.Description("Max results (default 50, max 200)")),
			mcpgo.WithNumber("offset", mcpgo.Description("Skip N results")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleGetMyIssues(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("get_issue",
			mcpgo.WithDescription("Get a single issue by its public ID"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID (idIssuePublic)")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleGetIssue(ctx, req, dispatcher)
		},
	)
	if !allowsWrite(stage) {
		return
	}
	server.AddTool(
		mcpgo.NewTool("create_issue",
			mcpgo.WithDescription("Create a new issue in a project"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("title", mcpgo.Required(), mcpgo.Description("Issue title (max 100 chars)")),
			mcpgo.WithString("description", mcpgo.Description("Issue description")),
			mcpgo.WithNumber("state_id", mcpgo.Description("State ID")),
			mcpgo.WithNumber("severity_id", mcpgo.Description("Severity ID")),
			mcpgo.WithNumber("assigned_to_id", mcpgo.Description("User ID to assign")),
			mcpgo.WithNumber("estimated", mcpgo.Description("Estimated seconds")),
			mcpgo.WithNumber("points", mcpgo.Description("Story points")),
			mcpgo.WithString("scheduled_at", mcpgo.Description("Scheduled datetime (RFC3339)")),
			mcpgo.WithString("idempotency_key", mcpgo.Description("Unique key to prevent duplicate creation on retry")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleCreateIssue(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("update_issue",
			mcpgo.WithDescription("Update an existing issue (only provided fields are changed)"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Public issue ID (idIssuePublic)")),
			mcpgo.WithString("title", mcpgo.Description("New title")),
			mcpgo.WithString("description", mcpgo.Description("New description")),
			mcpgo.WithNumber("state_id", mcpgo.Description("New state ID")),
			mcpgo.WithNumber("severity_id", mcpgo.Description("New severity ID")),
			mcpgo.WithNumber("assigned_to_id", mcpgo.Description("New assignee user ID")),
			mcpgo.WithNumber("estimated", mcpgo.Description("New estimated seconds")),
			mcpgo.WithNumber("points", mcpgo.Description("New story points")),
			mcpgo.WithString("scheduled_at", mcpgo.Description("New scheduled datetime (RFC3339)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleUpdateIssue(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("bulk_update_issues",
			mcpgo.WithDescription("Update state, severity, or assignee on multiple issues atomically"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithArray("issue_ids", mcpgo.Required(), mcpgo.Description("Public issue IDs to update"), mcpgo.WithNumberItems()),
			mcpgo.WithNumber("state_id", mcpgo.Description("State ID to apply to all")),
			mcpgo.WithNumber("severity_id", mcpgo.Description("Severity ID to apply to all")),
			mcpgo.WithNumber("assigned_to_id", mcpgo.Description("Assignee user ID to apply to all")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleBulkUpdateIssues(ctx, req, dispatcher)
		},
	)
}

func handleListIssues(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	query := url.Values{}
	if title := req.GetString("title_search", ""); title != "" {
		query.Set("title", title)
	}
	for _, id := range req.GetIntSlice("state_ids", nil) {
		query.Add("idsState", strconv.Itoa(id))
	}
	for _, id := range req.GetIntSlice("severity_ids", nil) {
		query.Add("idsSeverity", strconv.Itoa(id))
	}
	for _, id := range req.GetIntSlice("assigned_to_ids", nil) {
		query.Add("idsAssignedTo", strconv.Itoa(id))
	}
	if req.GetBool("exclude_final_states", false) {
		query.Set("excludeFinalStates", "true")
	}
	limit := req.GetInt("limit", 50)
	if limit > 200 {
		limit = 200
	}
	query.Set("limit", strconv.Itoa(limit))
	if offset := req.GetInt("offset", -1); offset >= 0 {
		query.Set("offset", strconv.Itoa(offset))
	}

	path := fmt.Sprintf("/api/private/project/%d/issue?%s", projectID, query.Encode())
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "list_issues",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleSearchIssues(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	searchQuery := req.GetString("query", "")
	if searchQuery == "" {
		return mcpgo.NewToolResultError("query is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	limit := req.GetInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	query := url.Values{}
	query.Set("search", searchQuery)
	query.Set("limit", strconv.Itoa(limit))

	path := fmt.Sprintf("/api/private/project/%d/issue?%s", projectID, query.Encode())
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "search_issues",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleGetMyIssues(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	bearer := req.Header.Get("Authorization")

	query := url.Values{}
	if projectID := req.GetInt("project_id", 0); projectID > 0 {
		query.Set("idProject", strconv.Itoa(projectID))
	}
	limit := req.GetInt("limit", 50)
	if limit > 200 {
		limit = 200
	}
	query.Set("limit", strconv.Itoa(limit))
	if offset := req.GetInt("offset", -1); offset >= 0 {
		query.Set("offset", strconv.Itoa(offset))
	}

	path := fmt.Sprintf("/api/private/my-issues?%s", query.Encode())
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "get_my_issues",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleGetIssue(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	path := fmt.Sprintf("/api/private/project/%d/issue/%d", projectID, issueID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "get_issue",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleCreateIssue(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	title := req.GetString("title", "")
	if title == "" {
		return mcpgo.NewToolResultError("title is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	body := map[string]any{
		"idProject": projectID,
		"title":     title,
	}
	if desc := req.GetString("description", ""); desc != "" {
		body["description"] = desc
	}
	if stateID := req.GetInt("state_id", 0); stateID > 0 {
		body["idState"] = stateID
	}
	if sevID := req.GetInt("severity_id", 0); sevID > 0 {
		body["idSeverity"] = sevID
	}
	if assignedTo := req.GetInt("assigned_to_id", 0); assignedTo > 0 {
		body["assignedTo"] = assignedTo
	}
	if estimated := req.GetInt("estimated", 0); estimated > 0 {
		body["estimated"] = estimated
	}
	// Presence check (not > 0) so 0-point issues are settable at create.
	if val, ok := req.GetArguments()["points"]; ok {
		body["points"] = val
	}
	if scheduledAt := req.GetString("scheduled_at", ""); scheduledAt != "" {
		body["scheduledAt"] = scheduledAt
	}

	idempotencyKey := req.GetString("idempotency_key", "")

	path := fmt.Sprintf("/api/private/project/%d/issue", projectID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:         "POST",
		Path:           path,
		Body:           body,
		Bearer:         bearer,
		ToolName:       "create_issue",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleUpdateIssue(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	args := req.GetArguments()
	body := map[string]any{
		"idProject":     projectID,
		"idIssuePublic": issueID,
	}
	fieldMap := map[string]string{
		"title":        "title",
		"description":  "description",
		"scheduled_at": "scheduledAt",
	}
	for argKey, bodyKey := range fieldMap {
		if val, ok := args[argKey]; ok {
			body[bodyKey] = val
		}
	}
	numericMap := map[string]string{
		"state_id":       "idState",
		"severity_id":    "idSeverity",
		"assigned_to_id": "assignedTo",
		"estimated":      "estimated",
		"points":         "points",
	}
	for argKey, bodyKey := range numericMap {
		if val, ok := args[argKey]; ok {
			body[bodyKey] = val
		}
	}

	path := fmt.Sprintf("/api/private/project/%d/issue/%d", projectID, issueID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "PATCH",
		Path:     path,
		Body:     body,
		Bearer:   bearer,
		ToolName: "update_issue",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleBulkUpdateIssues(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	issueIDs := req.GetIntSlice("issue_ids", nil)
	if len(issueIDs) == 0 {
		return mcpgo.NewToolResultError("issue_ids is required and must be non-empty"), nil
	}
	bearer := req.Header.Get("Authorization")

	args := req.GetArguments()
	type entry struct {
		IdIssuePublic  int  `json:"idIssuePublic"`
		IdState        *int `json:"idState,omitempty"`
		IdSeverity     *int `json:"idSeverity,omitempty"`
		IdUserAssigned *int `json:"idUserAssigned,omitempty"`
	}

	stateID := req.GetInt("state_id", 0)
	sevID := req.GetInt("severity_id", 0)
	assignedTo := req.GetInt("assigned_to_id", 0)

	entries := make([]entry, len(issueIDs))
	for i, id := range issueIDs {
		e := entry{IdIssuePublic: id}
		if _, ok := args["state_id"]; ok && stateID > 0 {
			e.IdState = &stateID
		}
		if _, ok := args["severity_id"]; ok && sevID > 0 {
			e.IdSeverity = &sevID
		}
		if _, ok := args["assigned_to_id"]; ok && assignedTo > 0 {
			e.IdUserAssigned = &assignedTo
		}
		entries[i] = e
	}

	path := fmt.Sprintf("/api/private/project/%d/issue/batch", projectID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "PATCH",
		Path:     path,
		Body:     map[string]any{"issues": entries},
		Bearer:   bearer,
		ToolName: "bulk_update_issues",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
