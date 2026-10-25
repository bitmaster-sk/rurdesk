package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerTagTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("list_tags",
			mcpgo.WithDescription("List tags (idTag, name, color, issueCount) across accessible projects. "+
				"Tags are free labels on issues; filter with list_issues tag_ids and set them with create_issue, update_issue or bulk_update_issues."),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListTags(ctx, req, dispatcher)
		},
	)
	if !allowsWrite(stage) {
		return
	}
	server.AddTool(
		mcpgo.NewTool("create_tag",
			mcpgo.WithDescription("Create a tag in a project. Names are unique per project, case-insensitive. Requires project owner."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("name", mcpgo.Required(), mcpgo.Description("Tag name (max 40 chars)")),
			mcpgo.WithString("color", mcpgo.Required(), mcpgo.Description(`Hex color, e.g. "#16a34a"`)),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleCreateTag(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("update_tag",
			mcpgo.WithDescription("Rename a tag or change its color. Every issue with the tag keeps it. Only provided fields change. Requires project owner."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("tag_id", mcpgo.Required(), mcpgo.Description("Tag ID")),
			mcpgo.WithString("name", mcpgo.Description("New name (max 40 chars)")),
			mcpgo.WithString("color", mcpgo.Description(`New hex color, e.g. "#16a34a"`)),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleUpdateTag(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("delete_tag",
			mcpgo.WithDescription("Delete a tag. It is removed from every issue; the issues stay. Requires project owner."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("tag_id", mcpgo.Required(), mcpgo.Description("Tag ID")),
			mcpgo.WithDestructiveHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleDeleteTag(ctx, req, dispatcher)
		},
	)
}

func handleListTags(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     "/api/private/tag",
		Bearer:   req.Header.Get("Authorization"),
		ToolName: "list_tags",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleCreateTag(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method: "POST",
		Path:   "/api/private/tag",
		Body: map[string]any{
			"idProject": projectID,
			"name":      req.GetString("name", ""),
			"color":     req.GetString("color", ""),
		},
		Bearer:   req.Header.Get("Authorization"),
		ToolName: "create_tag",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleUpdateTag(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	tagID := req.GetInt("tag_id", 0)
	if projectID == 0 || tagID == 0 {
		return mcpgo.NewToolResultError("project_id and tag_id are required"), nil
	}
	bearer := req.Header.Get("Authorization")

	current, errResult := loadTag(ctx, dispatcher, bearer, int64(projectID), int64(tagID))
	if errResult != nil {
		return errResult, nil
	}
	body := map[string]any{
		"idProject": projectID,
		"name":      req.GetString("name", current.Name),
		"color":     req.GetString("color", current.Color),
		"orderRank": current.OrderRank,
	}
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "PATCH",
		Path:     fmt.Sprintf("/api/private/tag/%d", tagID),
		Body:     body,
		Bearer:   bearer,
		ToolName: "update_tag",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleDeleteTag(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	tagID := req.GetInt("tag_id", 0)
	if projectID == 0 || tagID == 0 {
		return mcpgo.NewToolResultError("project_id and tag_id are required"), nil
	}
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "DELETE",
		Path:     fmt.Sprintf("/api/private/tag/%d/project/%d", tagID, projectID),
		Bearer:   req.Header.Get("Authorization"),
		ToolName: "delete_tag",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func loadTag(ctx context.Context, dispatcher *Dispatcher, bearer string, idProject, idTag int64) (*model.Tag, *mcpgo.CallToolResult) {
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     "/api/private/tag",
		Bearer:   bearer,
		ToolName: "update_tag",
	})
	if err != nil {
		return nil, mcpgo.NewToolResultError(err.Error())
	}
	if resp.IsError {
		return nil, mcpgo.NewToolResultError(resp.ErrorMessage)
	}
	var tags []model.Tag
	if err := json.Unmarshal(resp.Body, &tags); err != nil {
		return nil, mcpgo.NewToolResultError(fmt.Sprintf("decoding tags: %v", err))
	}
	for i := range tags {
		if tags[i].IdTag == idTag && tags[i].IdProject == idProject {
			return &tags[i], nil
		}
	}
	return nil, mcpgo.NewToolResultError(fmt.Sprintf("tag %d not found in project %d", idTag, idProject))
}
