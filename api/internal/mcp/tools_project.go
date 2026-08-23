package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerProjectTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	_ = stage // all project tools are read-only; stage param kept for signature parity
	server.AddTool(
		mcpgo.NewTool("list_projects",
			mcpgo.WithDescription("List all projects accessible to the caller"),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListProjects(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("get_project_context",
			mcpgo.WithDescription("Get project details with states, severities, issue types, and members in a single call — use this as an agent startup shortcut"),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleGetProjectContext(ctx, req, dispatcher)
		},
	)
}

func handleListProjects(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	bearer := req.Header.Get("Authorization")
	resp, err := dispatcher.Request(ctx, RequestOpts{
		Method:   "GET",
		Path:     "/api/private/project",
		Bearer:   bearer,
		ToolName: "list_projects",
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}

func handleGetProjectContext(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	bearer := req.Header.Get("Authorization")

	type parallelResult struct {
		body json.RawMessage
		err  error
	}

	projectCh := make(chan parallelResult, 1)
	statesCh := make(chan parallelResult, 1)
	sevsCh := make(chan parallelResult, 1)
	typesCh := make(chan parallelResult, 1)
	membersCh := make(chan parallelResult, 1)

	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		resp, err := dispatcher.Request(ctx, RequestOpts{
			Method:   "GET",
			Path:     fmt.Sprintf("/api/private/project/%d", projectID),
			Bearer:   bearer,
			ToolName: "get_project_context",
		})
		if err != nil {
			projectCh <- parallelResult{err: err}
			return
		}
		if resp.IsError {
			projectCh <- parallelResult{err: fmt.Errorf("%s", resp.ErrorMessage)}
			return
		}
		projectCh <- parallelResult{body: resp.Body}
	}()

	go func() {
		defer wg.Done()
		resp, err := dispatcher.Request(ctx, RequestOpts{
			Method:   "GET",
			Path:     "/api/private/state",
			Bearer:   bearer,
			ToolName: "get_project_context",
		})
		if err != nil {
			statesCh <- parallelResult{err: err}
			return
		}
		if resp.IsError {
			statesCh <- parallelResult{err: fmt.Errorf("%s", resp.ErrorMessage)}
			return
		}
		statesCh <- parallelResult{body: resp.Body}
	}()

	go func() {
		defer wg.Done()
		resp, err := dispatcher.Request(ctx, RequestOpts{
			Method:   "GET",
			Path:     "/api/private/severity",
			Bearer:   bearer,
			ToolName: "get_project_context",
		})
		if err != nil {
			sevsCh <- parallelResult{err: err}
			return
		}
		if resp.IsError {
			sevsCh <- parallelResult{err: fmt.Errorf("%s", resp.ErrorMessage)}
			return
		}
		sevsCh <- parallelResult{body: resp.Body}
	}()

	go func() {
		defer wg.Done()
		resp, err := dispatcher.Request(ctx, RequestOpts{
			Method:   "GET",
			Path:     "/api/private/issue-type",
			Bearer:   bearer,
			ToolName: "get_project_context",
		})
		if err != nil {
			typesCh <- parallelResult{err: err}
			return
		}
		if resp.IsError {
			typesCh <- parallelResult{err: fmt.Errorf("%s", resp.ErrorMessage)}
			return
		}
		typesCh <- parallelResult{body: resp.Body}
	}()

	go func() {
		defer wg.Done()
		resp, err := dispatcher.Request(ctx, RequestOpts{
			Method:   "GET",
			Path:     fmt.Sprintf("/api/private/project/%d/members", projectID),
			Bearer:   bearer,
			ToolName: "get_project_context",
		})
		if err != nil {
			membersCh <- parallelResult{err: err}
			return
		}
		if resp.IsError {
			membersCh <- parallelResult{err: fmt.Errorf("%s", resp.ErrorMessage)}
			return
		}
		membersCh <- parallelResult{body: resp.Body}
	}()

	wg.Wait()

	projectResult := <-projectCh
	statesResult := <-statesCh
	sevsResult := <-sevsCh
	typesResult := <-typesCh
	membersResult := <-membersCh

	for _, result := range []parallelResult{projectResult, statesResult, sevsResult, typesResult, membersResult} {
		if result.err != nil {
			return mcpgo.NewToolResultError(result.err.Error()), nil
		}
	}

	filteredStates := filterByProject(statesResult.body, int64(projectID))
	filteredSevs := filterByProject(sevsResult.body, int64(projectID))
	filteredTypes := filterByProject(typesResult.body, int64(projectID))

	combined := map[string]json.RawMessage{
		"project":    projectResult.body,
		"states":     filteredStates,
		"severities": filteredSevs,
		"issueTypes": filteredTypes,
		"members":    membersResult.body,
	}
	combinedJSON, err := json.Marshal(combined)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("merging results: %s", err.Error())), nil
	}
	return mcpgo.NewToolResultText(string(combinedJSON)), nil
}

// filterByProject filters a JSON array to only items where idProject matches.
func filterByProject(body json.RawMessage, idProject int64) json.RawMessage {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return body
	}
	filtered := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		var check struct {
			IdProject int64 `json:"idProject"`
		}
		if err := json.Unmarshal(item, &check); err == nil && check.IdProject == idProject {
			filtered = append(filtered, item)
		}
	}
	out, err := json.Marshal(filtered)
	if err != nil {
		return body
	}
	return out
}
