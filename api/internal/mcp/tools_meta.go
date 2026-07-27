package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerMetaTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	_ = stage // all meta tools are read-only; stage param kept for signature parity
	server.AddTool(
		mcpgo.NewTool("list_states",
			mcpgo.WithDescription("List all states (workflow steps) across accessible projects"),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListStates(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("list_severities",
			mcpgo.WithDescription("List all severities across accessible projects"),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListSeverities(ctx, req, dispatcher)
		},
	)
}

func handleListStates(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	bearer := req.Header.Get("Authorization")
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     "/api/private/state",
		Bearer:   bearer,
		ToolName: "list_states",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleListSeverities(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	bearer := req.Header.Get("Authorization")
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     "/api/private/severity",
		Bearer:   bearer,
		ToolName: "list_severities",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
