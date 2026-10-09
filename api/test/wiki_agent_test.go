package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

type wikiAgentFixture struct {
	*wikiFixture
	agentToken  string
	idAgent     int64
	mcpSessions map[string]string
	received    chan []byte
}

func newWikiAgentFixture(t *testing.T, name string) *wikiAgentFixture {
	f := newWikiFixture(t, name)
	email := name + "-agent@test.sk"
	agentToken := f.agentUser(t, email, "member")
	return &wikiAgentFixture{
		wikiFixture: f,
		agentToken:  agentToken,
		idAgent:     idOfUser(t, f.app, f.adminToken, email),
		mcpSessions: map[string]string{},
	}
}

func TestWikiAgent_PromptCarriesAlwaysAndLinkedPagesAndIndexesTheRest(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-prompt")
	always := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Ako pracujeme"), Body: "tests first", AgentAccess: "always"})
	linked := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Gateway"), Summary: "worktrees", Body: "worktree per run"})
	onDemand := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Runbook"), Summary: "how to deploy", Body: "deploy steps"})
	hidden := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Salaries"), Body: "secret numbers", AgentAccess: "hidden"})
	hiddenLinked := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Contracts"), Body: "secret contract", AgentAccess: "hidden"})

	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki prompt")
	f.linkPage(t, iss.IdIssue, linked.IdPage)
	_, err := f.app.Pool.Exec(context.Background(), `INSERT INTO wiki.issue_page (id_issue, id_page, source) VALUES ($1, $2, 'manual')`, iss.IdIssue, hiddenLinked.IdPage)
	require.NoError(t, err)

	run, task, bundle := f.dispatch(t, iss.IdIssue, "design")
	wiki := bundle.Wiki
	require.NotNil(t, wiki)

	pages := map[string]model.WikiPromptPage{}
	for _, page := range wiki.Pages {
		pages[page.Slug] = page
	}
	require.Equal(t, "tests first", pages[always.Slug].Body)
	require.Equal(t, "always", pages[always.Slug].Reason)
	require.Equal(t, 1, pages[always.Slug].Version)
	require.Equal(t, "worktree per run", pages[linked.Slug].Body, "a linked page is whole even when it is on demand")
	require.Equal(t, "linked", pages[linked.Slug].Reason)
	require.NotContains(t, pages, onDemand.Slug)

	index := map[string]model.WikiPromptIndexEntry{}
	for _, entry := range wiki.Index {
		index[entry.Slug] = entry
	}
	require.Equal(t, "how to deploy", index[onDemand.Slug].Summary)
	require.NotContains(t, index, always.Slug)
	require.NotContains(t, index, linked.Slug)

	raw, _ := json.Marshal(wiki)
	require.NotContains(t, string(raw), hidden.Slug)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), hiddenLinked.Slug, "a hidden page stays out even when it is linked to the task")

	reads := f.waitForReads(t, run.IdRun, func(reads []model.AgentRunWikiRead) bool { return len(reads) > 0 })
	sources := map[string]constants.WikiReadSource{}
	var indexRows []model.AgentRunWikiRead
	for _, read := range reads {
		require.Equal(t, task.IdTask, *read.IdTask)
		require.Equal(t, "design", read.Stage)
		if read.Source == constants.WikiReadPromptIndex {
			indexRows = append(indexRows, read)
			continue
		}
		sources[*read.Slug] = read.Source
	}
	require.Equal(t, constants.WikiReadPromptAlways, sources[always.Slug])
	require.Equal(t, constants.WikiReadPromptLinked, sources[linked.Slug])
	require.NotContains(t, sources, hidden.Slug)

	require.Len(t, indexRows, 1, "the whole index is one row however large the wiki is")
	indexed := map[string]bool{}
	for _, page := range indexRows[0].Pages {
		indexed[page.Slug] = true
	}
	require.True(t, indexed[onDemand.Slug])
	require.False(t, indexed[hidden.Slug])
	require.Equal(t, len(wiki.Index), len(indexRows[0].Pages))
	require.Positive(t, indexRows[0].Tokens)

	_, again, err := injector.GetWikiAgentService().BuildPromptContext(context.Background(), run, task)
	require.NoError(t, err)
	require.NoError(t, injector.GetWikiAgentService().RecordReads(context.Background(), again))
	require.Len(t, f.reads(t, run.IdRun), len(reads), "dispatching the same attempt again adds no rows")
}

func TestWikiAgent_PurgingPagesFromALoggedPromptStillWorks(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-purge")
	first := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("First rules"), Body: "one", AgentAccess: "always"})
	second := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Second rules"), Body: "two", AgentAccess: "always"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki purge")
	run, _, _ := f.dispatch(t, iss.IdIssue, "design")
	before := f.waitForReads(t, run.IdRun, func(reads []model.AgentRunWikiRead) bool { return len(reads) >= 2 })

	for _, page := range []model.WikiPage{first, second} {
		require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", page.IdPage), "", f.adminToken).StatusCode)
		res := Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d/purge", page.IdPage), "", f.adminToken)
		require.Equal(t, http.StatusNoContent, res.StatusCode, readBody(t, res))
	}

	after := f.reads(t, run.IdRun)
	require.Len(t, after, len(before), "purging a page keeps its reads in the log")
	titles := map[string]bool{}
	for _, read := range after {
		if read.Title != nil {
			titles[*read.Title] = true
			if *read.Title == first.Title || *read.Title == second.Title {
				require.Nil(t, read.IdPage)
			}
		}
	}
	require.True(t, titles[first.Title])
	require.True(t, titles[second.Title])
}

func TestWikiAgent_PromptStaysWithinATinyTokenLimit(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-limit")
	big := strings.Repeat("lorem ipsum ", 3000)
	linked := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Huge"), Body: big})
	require.Equal(t, http.StatusNoContent, Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/wiki/settings", f.idProject), `{"alwaysTokenLimit":50}`, f.adminToken).StatusCode)

	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki limit")
	f.linkPage(t, iss.IdIssue, linked.IdPage)
	_, _, bundle := f.dispatch(t, iss.IdIssue, "design")

	if bundle.Wiki == nil {
		return
	}
	raw, _ := json.Marshal(bundle.Wiki)
	require.NotContains(t, string(raw), "lorem ipsum lorem", "a page over the limit is never sent")
	total := 0
	for _, page := range bundle.Wiki.Pages {
		total += len([]rune(page.Body))
	}
	for _, entry := range bundle.Wiki.Index {
		total += len([]rune(entry.Slug + entry.Title + entry.Summary))
		if entry.Slug == linked.Slug {
			require.True(t, entry.OverLimit)
		}
	}
	require.LessOrEqual(t, (total+3)/4, 50)
}

func TestWikiAgent_ReadToolsWorkForAgentsAndPeopleAndNeverReturnHiddenPages(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-mcp")
	word := fmt.Sprintf("zzmcp%d", time.Now().UnixNano())
	open := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Open"), Summary: "visible", Body: word + " see [[Missing page]]"})
	hidden := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Hidden"), Body: word, AgentAccess: "hidden"})
	shared := f.mustCreateShared(t, uniqueWikiTitle("Shared rules"), word)

	for _, token := range []string{f.agentToken, f.adminToken} {
		hits := f.callTool(t, token, "", "search_wiki", map[string]any{"project_id": f.idProject, "query": word})
		require.False(t, hits.IsError, hits.Text)
		require.Contains(t, hits.Text, `"slug":"`+open.Slug+`"`)
		require.Contains(t, hits.Text, `"slug":"shared:`+shared.Slug+`"`)
		require.NotContains(t, hits.Text, hidden.Slug)

		page := f.callTool(t, token, "", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": open.Slug})
		require.False(t, page.IsError, page.Text)
		var view model.WikiAgentPage
		require.NoError(t, json.Unmarshal([]byte(page.Text), &view))
		require.Equal(t, 1, view.Version)
		require.Contains(t, view.Body, word)
		require.Equal(t, []model.WikiAgentLink{{Slug: "missing-page", Exists: false}}, view.Links)

		sharedPage := f.callTool(t, token, "", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": "shared:" + shared.Slug})
		require.False(t, sharedPage.IsError, sharedPage.Text)

		missing := f.callTool(t, token, "", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": hidden.Slug})
		require.True(t, missing.IsError)
		require.Contains(t, missing.Text, "not found", "a hidden page is a 404 for every key")

		list := f.callTool(t, token, "", "list_wiki_pages", map[string]any{"project_id": f.idProject})
		require.False(t, list.IsError, list.Text)
		require.Contains(t, list.Text, open.Slug)
		require.NotContains(t, list.Text, hidden.Slug)
	}
}

func TestWikiAgent_GetPageReadsAnOlderVersion(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-version")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Versioned"), Body: "first"})
	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: page.Title, Body: "second", AgentAccess: "on_demand"}).StatusCode)

	old := f.callTool(t, f.agentToken, "", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": page.Slug, "version": 1})
	require.False(t, old.IsError, old.Text)
	var view model.WikiAgentPage
	require.NoError(t, json.Unmarshal([]byte(old.Text), &view))
	require.Equal(t, "first", view.Body)
	require.Equal(t, 1, view.Version)
	require.Equal(t, 2, view.LatestVersion)

	gone := f.callTool(t, f.agentToken, "", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": page.Slug, "version": 9})
	require.True(t, gone.IsError)
}

func TestWikiAgent_UpsertIsForPeopleOnly(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-upsert")
	title := uniqueWikiTitle("Written over MCP")

	denied := f.upsert(t, f.agentToken, map[string]any{"title": title, "body": "agent text"})
	require.Equal(t, http.StatusForbidden, denied.StatusCode, readBody(t, denied))

	created := f.upsert(t, f.adminToken, map[string]any{"title": title, "body": "v1", "summary": "about it"})
	createdBody := readBody(t, created)
	require.Equal(t, http.StatusOK, created.StatusCode, createdBody)
	var result model.WikiAgentUpsertRes
	require.NoError(t, json.Unmarshal([]byte(createdBody), &result))
	require.True(t, result.Created)
	require.Equal(t, 1, result.Version)

	blind := f.upsert(t, f.adminToken, map[string]any{"slug": result.Slug, "title": title, "body": "blind overwrite"})
	require.Equal(t, http.StatusConflict, blind.StatusCode, "an existing page needs base_version")
	blindTool := f.callTool(t, f.adminToken, "", "upsert_wiki_page", map[string]any{"project_id": f.idProject, "slug": result.Slug, "title": title, "body": "x"})
	require.True(t, blindTool.IsError)
	require.Contains(t, blindTool.Text, "base_version", "the tool tells a person how to fix the call")

	updated := f.upsert(t, f.adminToken, map[string]any{"slug": result.Slug, "title": title, "body": "v2", "baseVersion": 1})
	updatedBody := readBody(t, updated)
	require.Equal(t, http.StatusOK, updated.StatusCode, updatedBody)
	require.NoError(t, json.Unmarshal([]byte(updatedBody), &result))
	require.False(t, result.Created)
	require.Equal(t, 2, result.Version)

	view := f.view(t, f.adminToken, "project", result.Slug)
	require.Equal(t, "v2", view.Page.Body)
	require.Equal(t, "about it", view.Page.Summary, "an omitted summary is kept")

	stale := f.upsert(t, f.adminToken, map[string]any{"slug": result.Slug, "title": title, "body": "v1 edited", "baseVersion": 1})
	require.Equal(t, http.StatusConflict, stale.StatusCode, "an overlapping edit is refused")

	hidden := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Hidden target"), AgentAccess: "hidden"})
	require.Equal(t, http.StatusNotFound, f.upsert(t, f.adminToken, map[string]any{"slug": hidden.Slug, "title": hidden.Title, "body": "x", "baseVersion": 1}).StatusCode)

	tool := f.callTool(t, f.agentToken, "", "upsert_wiki_page", map[string]any{"project_id": f.idProject, "title": uniqueWikiTitle("Agent page"), "body": "x"})
	require.True(t, tool.IsError)
	require.Contains(t, tool.Text, "forbidden")

	viaMcp := f.callTool(t, f.adminToken, "", "upsert_wiki_page", map[string]any{"project_id": f.idProject, "title": uniqueWikiTitle("Person page"), "body": "x"})
	require.False(t, viaMcp.IsError, viaMcp.Text)
}

func TestWikiAgent_McpReadsAreLoggedOnlyForTheCallersOwnRun(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-agent-log")
	word := fmt.Sprintf("zzlog%d", time.Now().UnixNano())
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Logged"), Body: word})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki log")
	run, task := f.startRun(t, iss.IdIssue, "implementation")
	header := fmt.Sprint(run.IdRun)

	f.callTool(t, f.adminToken, header, "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": page.Slug})
	f.callTool(t, f.agentToken, "", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": page.Slug})
	f.callTool(t, f.agentToken, "not-a-number", "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": page.Slug})
	require.Empty(t, f.reads(t, run.IdRun), "reads from people or without the run header are not logged")

	got := f.callTool(t, f.agentToken, header, "get_wiki_page", map[string]any{"project_id": f.idProject, "slug": page.Slug})
	require.False(t, got.IsError, got.Text)
	f.callTool(t, f.agentToken, header, "search_wiki", map[string]any{"project_id": f.idProject, "query": word})
	f.callTool(t, f.agentToken, header, "search_wiki", map[string]any{"project_id": f.idProject, "query": "zznothingmatches"})

	reads := f.reads(t, run.IdRun)
	require.Len(t, reads, 3)
	require.Equal(t, constants.WikiReadMcpGet, reads[0].Source)
	require.Equal(t, page.Slug, *reads[0].Slug)
	require.Equal(t, 1, *reads[0].VersionNo)
	require.Equal(t, task.IdTask, *reads[0].IdTask)
	require.Equal(t, "implementation", reads[0].Stage)
	require.Positive(t, reads[0].Tokens)
	require.Equal(t, constants.WikiReadMcpSearch, reads[1].Source)
	require.Equal(t, word, *reads[1].Query)
	require.Equal(t, page.IdPage, *reads[1].IdPage)
	require.Equal(t, constants.WikiReadMcpSearch, reads[2].Source)
	require.Nil(t, reads[2].IdPage, "an empty search is logged without a page")

	other := f.otherProject(t, "wiki-agent-log-other")
	otherAgent := &wikiAgentFixture{wikiFixture: other, agentToken: other.agentUser(t, "wiki-agent-log-other@test.sk", "member"), mcpSessions: map[string]string{}}
	otherAgent.callTool(t, otherAgent.agentToken, header, "get_wiki_page", map[string]any{"project_id": other.idProject, "slug": page.Slug})
	require.Len(t, f.reads(t, run.IdRun), 3, "an agent cannot write into someone else's run")

	secondProject := f.otherProject(t, "wiki-agent-log-second")
	addMember(t, f.app, f.adminToken, secondProject.idProject, f.idAgent)
	secret := secondProject.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Other project plan"), Body: word})
	fromOther := f.callTool(t, f.agentToken, header, "get_wiki_page", map[string]any{"project_id": secondProject.idProject, "slug": secret.Slug})
	require.False(t, fromOther.IsError, fromOther.Text)
	f.callTool(t, f.agentToken, header, "search_wiki", map[string]any{"project_id": secondProject.idProject, "query": word})
	require.Len(t, f.reads(t, run.IdRun), 3, "reads from another project never land in this run's log")

	outsider := createUserAndLogin(t, f.app, f.adminToken, "wiki-agent-log-outsider@test.sk")
	require.Equal(t, http.StatusForbidden, Request(t, f.app, "GET", fmt.Sprintf("/api/private/agent/run/%d/wiki-read", run.IdRun), "", outsider).StatusCode)
}

type dispatchedBundle struct {
	Wiki          *model.WikiPromptContext      `json:"wiki"`
	WikiProposals []model.WikiProposalAgentView `json:"wikiProposals"`
}

func (f *wikiAgentFixture) startRun(t *testing.T, idIssue int64, stage string) (*model.AgentRun, *model.AgentTask) {
	ctx := context.Background()
	stagePlan, err := injector.GetStagePlanService().Build(map[string][]int64{})
	require.NoError(t, err)
	run, err := injector.GetAgentRunRepository().Insert(ctx, idIssue, f.idAgent, f.idProject, stagePlan)
	require.NoError(t, err)
	task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, f.idAgent, stage, 1)
	require.NoError(t, err)
	return run, task
}

func (f *wikiAgentFixture) dispatch(t *testing.T, idIssue int64, stage string) (*model.AgentRun, *model.AgentTask, dispatchedBundle) {
	run, task := f.startRun(t, idIssue, stage)
	return run, task, f.dispatchTask(t, run, task)
}

func (f *wikiAgentFixture) dispatchTask(t *testing.T, run *model.AgentRun, task *model.AgentTask) dispatchedBundle {
	if f.received == nil {
		received := make(chan []byte, 1)
		gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			select {
			case received <- body:
			default:
			}
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(gateway.Close)
		res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", f.idAgent), fmt.Sprintf(`{"gatewayUrl":%q}`, gateway.URL), f.adminToken)
		require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
		f.received = received
	}

	injector.GetDispatcher().DispatchStageExecute(context.Background(), run, task)

	var raw []byte
	select {
	case raw = <-f.received:
	case <-time.After(10 * time.Second):
		t.Fatal("gateway never received the stage_execute event")
	}
	var event struct {
		Payload struct {
			ContextBundle dispatchedBundle `json:"contextBundle"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &event))
	return event.Payload.ContextBundle
}

func (f *wikiAgentFixture) reads(t *testing.T, idRun int64) []model.AgentRunWikiRead {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/agent/run/%d/wiki-read", idRun), "", f.adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var reads []model.AgentRunWikiRead
	require.NoError(t, json.Unmarshal([]byte(body), &reads))
	return reads
}

func (f *wikiAgentFixture) waitForReads(t *testing.T, idRun int64, done func([]model.AgentRunWikiRead) bool) []model.AgentRunWikiRead {
	deadline := time.Now().Add(10 * time.Second)
	for {
		reads := f.reads(t, idRun)
		if done(reads) || time.Now().After(deadline) {
			return reads
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func addMember(t *testing.T, app *issue.Application, adminToken string, idProject, idUser int64) {
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/member/user", idProject), fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
}

func (f *wikiAgentFixture) linkPage(t *testing.T, idIssue, idPage int64) {
	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/issue/%d/wiki", idIssue), fmt.Sprintf(`{"idPage":%d}`, idPage), f.adminToken)
	require.Equal(t, http.StatusNoContent, res.StatusCode, readBody(t, res))
}

func (f *wikiAgentFixture) mustCreateShared(t *testing.T, title, body string) model.WikiPage {
	res, page := f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "instance", Title: title, Body: body})
	require.Equal(t, http.StatusOK, res.StatusCode)
	return page
}

func (f *wikiAgentFixture) upsert(t *testing.T, token string, req map[string]any) *http.Response {
	body, _ := json.Marshal(req)
	return Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/wiki/agent/page", f.idProject), string(body), token)
}

type wikiToolResult struct {
	IsError bool
	Text    string
}

func (f *wikiAgentFixture) callTool(t *testing.T, token, idRunHeader, name string, arguments map[string]any) wikiToolResult {
	session, ok := f.mcpSessions[token]
	if !ok {
		session = f.mcpInitialize(t, token)
		f.mcpSessions[token] = session
	}
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": arguments},
	})
	headers := map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "application/json, text/event-stream",
		"Authorization":  "Bearer " + token,
		"Mcp-Session-Id": session,
	}
	if idRunHeader != "" {
		headers[constants.AgentRunHeader] = idRunHeader
	}
	res := RequestWithHeaders(t, f.app, "POST", "/mcp/http", string(payload), headers)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var out mcpToolResult
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	require.Nil(t, out.Error)
	require.Len(t, out.Result.Content, 1)
	return wikiToolResult{IsError: out.Result.IsError, Text: out.Result.Content[0].Text}
}

func (f *wikiAgentFixture) mcpInitialize(t *testing.T, token string) string {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{` +
		`"protocolVersion":"2024-11-05","capabilities":{},` +
		`"clientInfo":{"name":"integration-test","version":"1.0.0"}}}`
	res := RequestWithHeaders(t, f.app, "POST", "/mcp/http", body, map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json, text/event-stream",
		"Authorization": "Bearer " + token,
	})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	session := res.Header.Get("Mcp-Session-Id")
	require.NotEmpty(t, session)
	return session
}
