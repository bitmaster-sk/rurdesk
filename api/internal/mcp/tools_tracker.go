package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerTrackerTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("list_track_records",
			mcpgo.WithDescription("List time-tracking records with optional filters"),
			mcpgo.WithNumber("project_id", mcpgo.Description("Filter by project ID")),
			mcpgo.WithNumber("issue_id", mcpgo.Description("Filter by internal issue ID (idIssue)")),
			mcpgo.WithNumber("user_id", mcpgo.Description("Filter by user ID")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListTrackRecords(ctx, req, dispatcher)
		},
	)
	if !allowsWrite(stage) {
		return
	}
	server.AddTool(
		mcpgo.NewTool("create_track",
			mcpgo.WithDescription("Create a time-tracking record for an issue"),
			mcpgo.WithNumber("issue_id", mcpgo.Required(), mcpgo.Description("Internal issue ID (idIssue)")),
			mcpgo.WithNumber("tracked", mcpgo.Required(), mcpgo.Description("Tracked time in seconds")),
			mcpgo.WithString("start_at", mcpgo.Description("Start datetime (RFC3339)")),
			mcpgo.WithString("end_at", mcpgo.Description("End datetime (RFC3339)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleCreateTrack(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("update_track",
			mcpgo.WithDescription("Update a time-tracking record"),
			mcpgo.WithNumber("track_id", mcpgo.Required(), mcpgo.Description("Track record ID")),
			mcpgo.WithNumber("tracked", mcpgo.Description("New tracked time in seconds")),
			mcpgo.WithString("start_at", mcpgo.Description("New start datetime (RFC3339)")),
			mcpgo.WithString("end_at", mcpgo.Description("New end datetime (RFC3339)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleUpdateTrack(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("delete_track",
			mcpgo.WithDescription("Delete a time-tracking record"),
			mcpgo.WithNumber("track_id", mcpgo.Required(), mcpgo.Description("Track record ID")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleDeleteTrack(ctx, req, dispatcher)
		},
	)
}

func handleListTrackRecords(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	bearer := req.Header.Get("Authorization")

	query := url.Values{}
	if projectID := req.GetInt("project_id", 0); projectID > 0 {
		query.Set("idProject", strconv.Itoa(projectID))
	}
	if issueID := req.GetInt("issue_id", 0); issueID > 0 {
		query.Set("idIssue", strconv.Itoa(issueID))
	}
	if userID := req.GetInt("user_id", 0); userID > 0 {
		query.Set("idUser", strconv.Itoa(userID))
	}

	path := "/api/private/track"
	if encoded := query.Encode(); encoded != "" {
		path = fmt.Sprintf("/api/private/track?%s", encoded)
	}
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     path,
		Bearer:   bearer,
		ToolName: "list_track_records",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleCreateTrack(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	issueID := req.GetInt("issue_id", 0)
	if issueID == 0 {
		return mcpgo.NewToolResultError("issue_id is required"), nil
	}
	tracked := req.GetInt("tracked", -1)
	if tracked < 0 {
		return mcpgo.NewToolResultError("tracked is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	body := map[string]any{
		"idIssue": issueID,
		"tracked": tracked,
	}
	if startAt := req.GetString("start_at", ""); startAt != "" {
		body["startAt"] = startAt
	}
	if endAt := req.GetString("end_at", ""); endAt != "" {
		body["endAt"] = endAt
	}

	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "POST",
		Path:     "/api/private/track",
		Body:     body,
		Bearer:   bearer,
		ToolName: "create_track",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleUpdateTrack(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	trackID := req.GetInt("track_id", 0)
	if trackID == 0 {
		return mcpgo.NewToolResultError("track_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	args := req.GetArguments()
	body := map[string]any{"idTrack": trackID}
	if _, ok := args["tracked"]; ok {
		body["tracked"] = req.GetInt("tracked", 0)
	}
	if startAt := req.GetString("start_at", ""); startAt != "" {
		body["startAt"] = startAt
	}
	if endAt := req.GetString("end_at", ""); endAt != "" {
		body["endAt"] = endAt
	}

	path := fmt.Sprintf("/api/private/track/%d", trackID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "PATCH",
		Path:     path,
		Body:     body,
		Bearer:   bearer,
		ToolName: "update_track",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleDeleteTrack(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	trackID := req.GetInt("track_id", 0)
	if trackID == 0 {
		return mcpgo.NewToolResultError("track_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	path := fmt.Sprintf("/api/private/track/%d", trackID)
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "DELETE",
		Path:     path,
		Bearer:   bearer,
		ToolName: "delete_track",
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
