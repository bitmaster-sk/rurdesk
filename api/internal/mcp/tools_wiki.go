package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpsdk "github.com/mark3labs/mcp-go/server"
)

func registerWikiTools(server *mcpsdk.MCPServer, dispatcher *Dispatcher, stage string) {
	server.AddTool(
		mcpgo.NewTool("search_wiki",
			mcpgo.WithDescription("Full-text search over the project wiki and the shared wiki. Every word matches as a prefix. Returns slug, space, title, summary, a snippet with matches between << and >>, and the current version. Read a hit with get_wiki_page."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Words to search for")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of hits, 10 by default, at most 50")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleSearchWiki(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("get_wiki_page",
			mcpgo.WithDescription("Read one wiki page as Markdown with its version, the pages it links to and the 20 most recent tasks linked to it with their total count (open tasks first; use search_issues for more). A plain slug is looked up in the project wiki first, then in the shared wiki; prefix it with shared: to read the shared page."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("slug", mcpgo.Required(), mcpgo.Description("Page slug, e.g. deploy-runbook or shared:security. A page title works too.")),
			mcpgo.WithNumber("version", mcpgo.Description("Version number to read; the current version when omitted")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleGetWikiPage(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("list_wiki_pages",
			mcpgo.WithDescription("List the pages of the project wiki and the shared wiki as a tree: slug, space, title, parent slug and how agents get the page (always or on_demand)."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListWikiPages(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("list_my_wiki_proposals",
			mcpgo.WithDescription("List the wiki change proposals of an agent run with their state: open or approved while the pull request is open (a later suggestion for the same page takes an approval back), ready after the merge, needs_resolving when an approved change no longer merges, accepted once it is in the wiki, rejected, or discarded when the run ended without a merge."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithNumber("run_id", mcpgo.Required(), mcpgo.Description("Agent run ID")),
			mcpgo.WithReadOnlyHintAnnotation(true),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleListMyWikiProposals(ctx, req, dispatcher)
		},
	)
	if !allowsWrite(stage) {
		return
	}
	server.AddTool(
		mcpgo.NewTool("suggest_wiki_change",
			mcpgo.WithDescription("Propose a change to the wiki during the implementation stage. Nothing is written: a person reviews the proposal after the pull request is merged. Calling it again for the same page, or the same new slug, updates your proposal instead of adding another. kind=create needs title and body; update needs slug, body and base_version (the version you read); move needs slug and parent; delete needs slug."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("kind", mcpgo.Required(), mcpgo.Description("create, update, move or delete")),
			mcpgo.WithString("reason", mcpgo.Required(), mcpgo.Description("Why the wiki must change: what in the code makes the page wrong or missing")),
			mcpgo.WithString("slug", mcpgo.Description("Slug of the page to change, e.g. deploy-runbook or shared:security. For create, optional: the slug always comes from the title, prefix it with shared: to create in the shared wiki.")),
			mcpgo.WithString("title", mcpgo.Description("Page title; required for create, a new title for update")),
			mcpgo.WithString("body", mcpgo.Description("Whole page content as Markdown, for create and update")),
			mcpgo.WithString("summary", mcpgo.Description("One-line summary; kept as is when omitted")),
			mcpgo.WithString("parent", mcpgo.Description("Slug of the parent page: where to create the page, or where to move it (empty string for the top level)")),
			mcpgo.WithString("space", mcpgo.Description("project (default) or instance for the shared wiki, for create")),
			mcpgo.WithNumber("base_version", mcpgo.Description("For update: the version of the page you based the change on")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleSuggestWikiChange(ctx, req, dispatcher)
		},
	)
	server.AddTool(
		mcpgo.NewTool("upsert_wiki_page",
			mcpgo.WithDescription("Create a wiki page or save a new version of an existing one. Only personal API keys can write; agent keys get 403. To change an existing page pass base_version, the version you read with get_wiki_page: edits made since then are merged, and overlapping edits are refused with 409."),
			mcpgo.WithNumber("project_id", mcpgo.Required(), mcpgo.Description("Project ID")),
			mcpgo.WithString("title", mcpgo.Required(), mcpgo.Description("Page title. A new page gets its slug from it.")),
			mcpgo.WithString("body", mcpgo.Required(), mcpgo.Description("Whole page content as Markdown")),
			mcpgo.WithString("slug", mcpgo.Description("Slug of the page to update, e.g. deploy-runbook or shared:security. Defaults to the slug of the title.")),
			mcpgo.WithString("space", mcpgo.Description("project (default) or instance for the shared wiki")),
			mcpgo.WithString("summary", mcpgo.Description("One-line summary shown in search and in the agent index; kept as is when omitted")),
			mcpgo.WithString("parent", mcpgo.Description("Slug of the parent page; only used when the page is created")),
			mcpgo.WithString("agent_access", mcpgo.Description("always, on_demand or hidden; kept as is when omitted, on_demand for a new page")),
			mcpgo.WithNumber("base_version", mcpgo.Description("Version the change is based on; required when the page already exists")),
			mcpgo.WithString("note", mcpgo.Description("Short note shown in the page history")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return handleUpsertWikiPage(ctx, req, dispatcher)
		},
	)
}

func handleSearchWiki(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	query := req.GetString("query", "")
	if query == "" {
		return mcpgo.NewToolResultError("query is required"), nil
	}
	params := url.Values{"query": {query}}
	if limit := req.GetInt("limit", 0); limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	return wikiRequest(ctx, req, dispatcher, "search_wiki", RequestOpts{
		Method: "GET",
		Path:   fmt.Sprintf("/api/private/project/%d/wiki/agent/search?%s", projectID, params.Encode()),
	})
}

func handleGetWikiPage(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	slug := req.GetString("slug", "")
	if slug == "" {
		return mcpgo.NewToolResultError("slug is required"), nil
	}
	path := fmt.Sprintf("/api/private/project/%d/wiki/agent/page/%s", projectID, url.PathEscape(slug))
	if version := req.GetInt("version", 0); version > 0 {
		path += "?version=" + strconv.Itoa(version)
	}
	return wikiRequest(ctx, req, dispatcher, "get_wiki_page", RequestOpts{Method: "GET", Path: path})
}

func handleListWikiPages(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	return wikiRequest(ctx, req, dispatcher, "list_wiki_pages", RequestOpts{
		Method: "GET",
		Path:   fmt.Sprintf("/api/private/project/%d/wiki/agent/pages", projectID),
	})
}

func handleUpsertWikiPage(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	title := req.GetString("title", "")
	if title == "" {
		return mcpgo.NewToolResultError("title is required"), nil
	}
	body := map[string]any{
		"title":       title,
		"body":        req.GetString("body", ""),
		"slug":        req.GetString("slug", ""),
		"space":       req.GetString("space", ""),
		"agentAccess": req.GetString("agent_access", ""),
		"baseVersion": req.GetInt("base_version", 0),
		"note":        req.GetString("note", ""),
	}
	arguments := req.GetArguments()
	if summary, ok := arguments["summary"].(string); ok {
		body["summary"] = summary
	}
	if parent, ok := arguments["parent"].(string); ok {
		body["parent"] = parent
	}
	return wikiRequest(ctx, req, dispatcher, "upsert_wiki_page", RequestOpts{
		Method: "POST",
		Path:   fmt.Sprintf("/api/private/project/%d/wiki/agent/page", projectID),
		Body:   body,
	})
}

func handleListMyWikiProposals(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	runID := req.GetInt("run_id", 0)
	if runID == 0 {
		return mcpgo.NewToolResultError("run_id is required"), nil
	}
	return wikiRequest(ctx, req, dispatcher, "list_my_wiki_proposals", RequestOpts{
		Method: "GET",
		Path:   fmt.Sprintf("/api/private/project/%d/wiki/agent/proposal?run=%d", projectID, runID),
	})
}

func handleSuggestWikiChange(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher) (*mcpgo.CallToolResult, error) {
	projectID := req.GetInt("project_id", 0)
	if projectID == 0 {
		return mcpgo.NewToolResultError("project_id is required"), nil
	}
	kind := req.GetString("kind", "")
	if kind == "" {
		return mcpgo.NewToolResultError("kind is required"), nil
	}
	body := map[string]any{
		"kind":        kind,
		"reason":      req.GetString("reason", ""),
		"slug":        req.GetString("slug", ""),
		"title":       req.GetString("title", ""),
		"space":       req.GetString("space", ""),
		"baseVersion": req.GetInt("base_version", 0),
	}
	arguments := req.GetArguments()
	for _, name := range []string{"body", "summary", "parent"} {
		if value, ok := arguments[name].(string); ok {
			body[name] = value
		}
	}
	return wikiRequest(ctx, req, dispatcher, "suggest_wiki_change", RequestOpts{
		Method: "POST",
		Path:   fmt.Sprintf("/api/private/project/%d/wiki/agent/proposal", projectID),
		Body:   body,
	})
}

func wikiRequest(ctx context.Context, req mcpgo.CallToolRequest, dispatcher *Dispatcher, toolName string, opts RequestOpts) (*mcpgo.CallToolResult, error) {
	opts.Bearer = req.Header.Get("Authorization")
	opts.ToolName = toolName
	opts.Headers = map[string]string{constants.AgentRunHeader: req.Header.Get(constants.AgentRunHeader)}
	resp, err := dispatcher.Request(ctx, opts)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if resp.IsError {
		return mcpgo.NewToolResultError(resp.ErrorMessage), nil
	}
	return mcpgo.NewToolResultText(string(resp.Body)), nil
}
