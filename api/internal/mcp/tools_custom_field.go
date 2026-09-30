package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerCustomFieldTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	_ = stage // unused; the signature matches the other register funcs
	server.AddTool(
		mcpgo.NewTool("list_custom_fields",
			mcpgo.WithDescription("List custom field definitions (key, name, type, options, defaultValue) across accessible projects. "+
				"isRequired means create_issue must carry that key for the field's project, unless the field "+
				"has a defaultValue, which is applied to keys create_issue omits."),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListCustomFields(ctx, req, dispatcher)
		},
	)
}

func handleListCustomFields(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	bearer := req.Header.Get("Authorization")
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     "/api/private/custom-field",
		Bearer:   bearer,
		ToolName: "list_custom_fields",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
