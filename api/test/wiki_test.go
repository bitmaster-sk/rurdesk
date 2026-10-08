package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/stretchr/testify/require"
)

type wikiFixture struct {
	app        *issue.Application
	adminToken string
	idProject  int64
}

func TestWiki_CreateAndReadPageWithResolvedLinksAndBacklinks(t *testing.T) {
	f := newWikiFixture(t, "wiki-read")
	target := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Prehľad systému", Body: "layers"})
	source := f.mustCreate(t, model.CreateWikiPageReq{
		Space: "project", Title: "Konvencie backendu",
		Body: "See [[Prehľad systému]] and the missing [[Nepísaná stránka]].",
	})
	require.Equal(t, "konvencie-backendu", source.Slug)
	require.Equal(t, 1, source.VersionNo)

	view := f.view(t, f.adminToken, "project", source.Slug)
	require.True(t, view.CanEdit)
	require.Len(t, view.Links, 2)
	require.NotNil(t, view.Links[0].IdPage)
	require.Equal(t, target.IdPage, *view.Links[0].IdPage)
	require.Nil(t, view.Links[1].IdPage)

	targetView := f.view(t, f.adminToken, "project", target.Slug)
	require.Len(t, targetView.Backlinks.Items, 1)
	require.Equal(t, 1, targetView.Backlinks.Total)
	require.Equal(t, source.IdPage, targetView.Backlinks.Items[0].IdPage)

	tree := f.tree(t, f.adminToken)
	require.Len(t, tree.Spaces, 2)
	require.Equal(t, "instance", string(tree.Spaces[0].Kind))
}

func TestWiki_DuplicateTitleInOneSpaceIsRejected(t *testing.T) {
	f := newWikiFixture(t, "wiki-dup")
	f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Testovanie"})
	res, _ := f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "project", Title: "testovanie!"})
	require.Equal(t, http.StatusConflict, res.StatusCode)

	res, _ = f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "project", Title: "!!!"})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestWiki_PermissionsFollowProjectRoles(t *testing.T) {
	f := newWikiFixture(t, "wiki-acl")
	viewer := f.userWithRole(t, "wiki-viewer@test.sk", "viewer")
	member := f.userWithRole(t, "wiki-member@test.sk", "member")
	stranger := f.userWithRole(t, "wiki-stranger@test.sk", "")

	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Roles"})

	require.Equal(t, http.StatusForbidden, Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/tree", f.idProject), "", stranger).StatusCode)
	require.Equal(t, http.StatusNotFound, Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/version", page.IdPage), "", stranger).StatusCode)

	viewerView := f.view(t, viewer, "project", page.Slug)
	require.False(t, viewerView.CanEdit)
	res, _ := f.create(t, viewer, model.CreateWikiPageReq{Space: "project", Title: "By viewer"})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.Equal(t, http.StatusForbidden, f.save(t, viewer, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Roles", Body: "x", AgentAccess: "on_demand"}).StatusCode)

	res, _ = f.create(t, member, model.CreateWikiPageReq{Space: "project", Title: "By member"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	res, _ = f.create(t, member, model.CreateWikiPageReq{Space: "project", Title: "Always by member", AgentAccess: "always"})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.Equal(t, http.StatusForbidden, f.save(t, member, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Roles", AgentAccess: "always"}).StatusCode)
	require.Equal(t, http.StatusOK, f.save(t, member, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Roles", Body: "edited", AgentAccess: "on_demand"}).StatusCode)

	sharedTitle := uniqueWikiTitle("Shared by member")
	res, _ = f.create(t, member, model.CreateWikiPageReq{Space: "instance", Title: sharedTitle})
	require.Equal(t, http.StatusOK, res.StatusCode, "a member of any project may write shared pages")
	res, _ = f.create(t, viewer, model.CreateWikiPageReq{Space: "instance", Title: uniqueWikiTitle("Shared by viewer")})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	res, _ = f.create(t, member, model.CreateWikiPageReq{Space: "instance", Title: uniqueWikiTitle("Shared always"), AgentAccess: "always"})
	require.Equal(t, http.StatusForbidden, res.StatusCode, "only an admin manages agent access of shared pages")

	require.Equal(t, http.StatusForbidden, Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/wiki/settings", f.idProject), `{"alwaysTokenLimit":100}`, member).StatusCode)
}

func TestWiki_ConcurrentSavesOfDifferentPartsMergeWithoutError(t *testing.T) {
	f := newWikiFixture(t, "wiki-merge")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Merge", Body: "## Layers\ncontroller\n\n## Errors\nstatus from error"})

	first := f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Merge", AgentAccess: "on_demand", Note: "errors",
		Body: "## Layers\ncontroller\n\n## Errors\nstatus from error (see #11)",
	})
	require.Equal(t, http.StatusOK, first.StatusCode)

	second := f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Merge", AgentAccess: "on_demand", Note: "layers",
		Body: "## Layers\ncontroller calls service\n\n## Errors\nstatus from error",
	})
	require.Equal(t, http.StatusOK, second.StatusCode)
	var result service.WikiSaveResult
	require.Nil(t, json.NewDecoder(second.Body).Decode(&result))
	require.NotNil(t, result.MergedFrom)
	require.Equal(t, 2, *result.MergedFrom)
	require.Equal(t, 3, result.Page.VersionNo)
	require.Equal(t, "## Layers\ncontroller calls service\n\n## Errors\nstatus from error (see #11)", result.Page.Body)

	versions := f.versions(t, page.IdPage)
	require.Len(t, versions, 3)
	require.Equal(t, 2, *versions[0].MergedFrom)
}

func TestWiki_OverlappingSaveReturnsConflictAndWritesNothing(t *testing.T) {
	f := newWikiFixture(t, "wiki-conflict")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Conflict", Body: "intro\nshared line\noutro"})

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Conflict", AgentAccess: "on_demand", Body: "intro\ntheirs\noutro",
	}).StatusCode)

	res := f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Conflict", AgentAccess: "on_demand", Body: "intro\nmine\noutro",
	})
	require.Equal(t, http.StatusConflict, res.StatusCode)
	var conflict service.WikiConflictInfo
	require.Nil(t, json.NewDecoder(res.Body).Decode(&conflict))
	require.Equal(t, 1, conflict.Merge.Conflicts)
	require.Equal(t, 2, conflict.Current.VersionNo)

	require.Len(t, f.versions(t, page.IdPage), 2)
	view := f.view(t, f.adminToken, "project", page.Slug)
	require.Equal(t, "intro\ntheirs\noutro", view.Page.Body)

	preview := Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/merge-preview", page.IdPage),
		`{"baseVersion":1,"title":"Conflict","body":"intro\nmine\noutro"}`, f.adminToken)
	require.Equal(t, http.StatusOK, preview.StatusCode)
	var mergePreview service.WikiMergePreview
	require.Nil(t, json.NewDecoder(preview.Body).Decode(&mergePreview))
	require.Equal(t, 2, mergePreview.CurrentVersion)
	require.Equal(t, 1, mergePreview.Merge.Conflicts)
}

func TestWiki_SavingUnchangedContentCreatesNoVersionAndClearsDraft(t *testing.T) {
	f := newWikiFixture(t, "wiki-noop")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Noop", Body: "same"})

	draftUrl := fmt.Sprintf("/api/private/wiki/page/%d/draft", page.IdPage)
	require.Equal(t, http.StatusNoContent, Request(t, f.app, "PUT", draftUrl, `{"baseVersion":1,"title":"Noop","body":"half written"}`, f.adminToken).StatusCode)
	draftRes := Request(t, f.app, "GET", draftUrl, "", f.adminToken)
	require.Contains(t, readBody(t, draftRes), "half written")
	require.NotNil(t, f.view(t, f.adminToken, "project", page.Slug).Draft)

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Noop", Body: "same", AgentAccess: "on_demand"}).StatusCode)
	require.Len(t, f.versions(t, page.IdPage), 1)
	require.Equal(t, "null", strings.TrimSpace(readBody(t, Request(t, f.app, "GET", draftUrl, "", f.adminToken))))
}

func TestWiki_RevertCreatesNewVersionAndDiffShowsChange(t *testing.T) {
	f := newWikiFixture(t, "wiki-revert")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Revert", Body: "first"})
	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: "Revert", Body: "second", AgentAccess: "on_demand"}).StatusCode)

	diffRes := Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/diff?from=1&to=2", page.IdPage), "", f.adminToken)
	require.Equal(t, http.StatusOK, diffRes.StatusCode)
	diff := readBody(t, diffRes)
	require.Contains(t, diff, "-first")
	require.Contains(t, diff, "+second")

	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/revert/1", page.IdPage), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var reverted model.WikiPage
	require.Nil(t, json.NewDecoder(res.Body).Decode(&reverted))
	require.Equal(t, 3, reverted.VersionNo)
	require.Equal(t, "first", reverted.Body)
	require.Equal(t, "restored v1", f.versions(t, page.IdPage)[0].Note)
}

func TestWiki_TrashRestorePurge(t *testing.T) {
	f := newWikiFixture(t, "wiki-trash-flow")
	member := f.userWithRole(t, "wiki-trash-member@test.sk", "member")
	ops := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Ops"})
	deploy := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Deploy", IdParent: &ops.IdPage})
	rollback := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Rollback", IdParent: &deploy.IdPage})

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", deploy.IdPage), "", member).StatusCode)
	require.Len(t, f.projectNodes(t), 1)

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", ops.IdPage), "", f.adminToken).StatusCode)
	require.Equal(t, http.StatusNotFound, Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/restore", rollback.IdPage), "", f.adminToken).StatusCode,
		"only the root of a trashed subtree is restorable")

	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/restore", deploy.IdPage), "", member)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var restored model.WikiPage
	require.Nil(t, json.NewDecoder(res.Body).Decode(&restored))
	require.Nil(t, restored.IdParent, "a page whose parent is still in the trash returns to the root")
	require.Len(t, f.projectNodes(t), 2)

	f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Ops"})
	require.Equal(t, http.StatusConflict, Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/restore", ops.IdPage), "", f.adminToken).StatusCode)

	require.Equal(t, http.StatusForbidden, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d/purge", ops.IdPage), "", member).StatusCode)
	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d/purge", ops.IdPage), "", f.adminToken).StatusCode)
	trashRes := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/trash", f.idProject), "", f.adminToken)
	require.NotContains(t, readBody(t, trashRes), fmt.Sprintf(`"idPage":%d,`, ops.IdPage))
}

func TestWiki_ExpiredTrashIsPurged(t *testing.T) {
	f := newWikiFixture(t, "wiki-expire")
	old := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Old"})
	fresh := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Fresh"})
	for _, id := range []int64{old.IdPage, fresh.IdPage} {
		require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", id), "", f.adminToken).StatusCode)
	}
	_, err := f.app.Pool.Exec(context.Background(), `UPDATE wiki.page SET deleted_at = deleted_at - interval '31 days' WHERE id_page = $1`, old.IdPage)
	require.Nil(t, err)

	_, err = injector.GetWikiService().PurgeExpiredTrash(context.Background(), time.Now().UTC())
	require.Nil(t, err)

	gone, err := injector.GetWikiPageRepository().LoadPage(context.Background(), old.IdPage)
	require.Nil(t, err)
	require.Nil(t, gone)
	kept, err := injector.GetWikiPageRepository().LoadPage(context.Background(), fresh.IdPage)
	require.Nil(t, err)
	require.NotNil(t, kept)
}

func TestWiki_AlwaysPagesRespectTheTokenLimit(t *testing.T) {
	f := newWikiFixture(t, "wiki-budget")
	require.Equal(t, http.StatusNoContent, Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/wiki/settings", f.idProject), `{"alwaysTokenLimit":50}`, f.adminToken).StatusCode)

	res, small := f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "project", Title: "Small", Body: strings.Repeat("a", 100), AgentAccess: "always"})
	require.Equal(t, http.StatusOK, res.StatusCode)

	res, _ = f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "project", Title: "Big", Body: strings.Repeat("b", 200), AgentAccess: "always"})
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	require.Equal(t, http.StatusUnprocessableEntity, f.save(t, f.adminToken, small.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Small", Body: strings.Repeat("a", 400), AgentAccess: "always",
	}).StatusCode)

	tree := f.tree(t, f.adminToken)
	require.Equal(t, 50, tree.TokenLimit)
	require.Greater(t, tree.AlwaysTokens, 25)
}

func TestWiki_MoveRejectsCyclesAndReorders(t *testing.T) {
	f := newWikiFixture(t, "wiki-move")
	parent := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Parent"})
	child := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Child", IdParent: &parent.IdPage})
	other := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Other"})

	res := Request(t, f.app, "PUT", fmt.Sprintf("/api/private/wiki/page/%d/position", parent.IdPage), fmt.Sprintf(`{"idParent":%d}`, child.IdPage), f.adminToken)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	res = Request(t, f.app, "PUT", fmt.Sprintf("/api/private/wiki/page/%d/position", other.IdPage), fmt.Sprintf(`{"idNext":%d}`, parent.IdPage), f.adminToken)
	require.Equal(t, http.StatusNoContent, res.StatusCode)

	tree := f.tree(t, f.adminToken)
	roots := make([]int64, 0)
	for _, node := range tree.Nodes {
		if node.IdSpace == tree.Spaces[1].IdSpace && node.IdParent == nil {
			roots = append(roots, node.IdPage)
		}
	}
	require.Equal(t, []int64{other.IdPage, parent.IdPage}, roots)
}

func TestWiki_SearchFindsPagesInBothSpaces(t *testing.T) {
	f := newWikiFixture(t, "wiki-search-http")
	marker := fmt.Sprintf("zebra%d", time.Now().UnixNano())
	f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Local", Body: "the " + marker + " lives here"})
	f.mustCreate(t, model.CreateWikiPageReq{Space: "instance", Title: uniqueWikiTitle("Shared"), Body: marker + " too"})

	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/search?q=%s", f.idProject, marker), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var hits []model.WikiSearchHit
	require.Nil(t, json.NewDecoder(res.Body).Decode(&hits))
	require.Len(t, hits, 2)

	res = Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/search?q=%s", f.idProject, marker[:len(marker)-4]), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Nil(t, json.NewDecoder(res.Body).Decode(&hits))
	require.Len(t, hits, 2, "a partly typed word finds the pages")
}

func TestWiki_IssueDescriptionLinksAndManualLinks(t *testing.T) {
	f := newWikiFixture(t, "wiki-issue")
	described := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Agentný beh"})
	manual := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Git integrácia"})

	body, _ := json.Marshal(map[string]any{"idProject": f.idProject, "title": "linked", "description": "Súvisí s [[Agentný beh]]."})
	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/issue", f.idProject), string(body), f.adminToken)
	issueBody := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, issueBody)
	var created model.Issue
	require.Nil(t, json.Unmarshal([]byte(issueBody), &created))

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "POST", fmt.Sprintf("/api/private/issue/%d/wiki", created.IdIssue), fmt.Sprintf(`{"idPage":%d}`, manual.IdPage), f.adminToken).StatusCode)

	linksRes := Request(t, f.app, "GET", fmt.Sprintf("/api/private/issue/%d/wiki", created.IdIssue), "", f.adminToken)
	require.Equal(t, http.StatusOK, linksRes.StatusCode)
	var links []model.WikiIssueLink
	require.Nil(t, json.NewDecoder(linksRes.Body).Decode(&links))
	require.Len(t, links, 2)
	sources := map[int64]string{}
	for _, link := range links {
		sources[link.IdPage] = string(link.Source)
	}
	require.Equal(t, "description", sources[described.IdPage])
	require.Equal(t, "manual", sources[manual.IdPage])

	view := f.view(t, f.adminToken, "project", described.Slug)
	require.Len(t, view.Issues.Items, 1)
	require.Equal(t, 1, view.Issues.Total)
	require.Equal(t, created.IdIssuePublic, view.Issues.Items[0].IdIssuePublic)
}

func TestWiki_HeartbeatListsOtherEditors(t *testing.T) {
	f := newWikiFixture(t, "wiki-presence")
	member := f.userWithRole(t, "wiki-presence@test.sk", "member")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Presence"})
	url := fmt.Sprintf("/api/private/wiki/page/%d/editing", page.IdPage)

	require.Equal(t, http.StatusOK, Request(t, f.app, "PUT", url, "", member).StatusCode)
	res := Request(t, f.app, "PUT", url, "", f.adminToken)
	var editors []model.WikiEditor
	require.Nil(t, json.NewDecoder(res.Body).Decode(&editors))
	require.Len(t, editors, 1)
	require.Equal(t, "stranger", editors[0].Name)

	require.Equal(t, http.StatusOK, Request(t, f.app, "DELETE", url, "", member).StatusCode)
	res = Request(t, f.app, "PUT", url, "", f.adminToken)
	require.Nil(t, json.NewDecoder(res.Body).Decode(&editors))
	require.Empty(t, editors)
}

func TestWiki_ConflictCarriesMergedFieldsSoResolvingKeepsTheirChanges(t *testing.T) {
	f := newWikiFixture(t, "wiki-conflict-fields")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Fields", Body: "intro\nline\noutro"})

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Fields renamed", Summary: "their summary", AgentAccess: "hidden", Body: "intro\ntheirs\noutro",
	}).StatusCode)

	res := f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Fields", AgentAccess: "on_demand", Body: "intro\nmine\noutro",
	})
	require.Equal(t, http.StatusConflict, res.StatusCode)
	var conflict service.WikiConflictInfo
	require.Nil(t, json.NewDecoder(res.Body).Decode(&conflict))
	require.Equal(t, "Fields renamed", conflict.Title)
	require.Equal(t, "their summary", conflict.Summary)
	require.Equal(t, "hidden", string(conflict.AgentAccess))

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: conflict.Current.VersionNo, Title: conflict.Title, Summary: conflict.Summary,
		AgentAccess: conflict.AgentAccess, Body: "intro\nmine\noutro",
	}).StatusCode)
	view := f.view(t, f.adminToken, "project", page.Slug)
	require.Equal(t, "Fields renamed", view.Page.Title)
	require.Equal(t, "hidden", string(view.Page.AgentAccess))
	require.Equal(t, "intro\nmine\noutro", view.Page.Body)

	preview := Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/merge-preview", page.IdPage),
		`{"baseVersion":1,"title":"Fields","agentAccess":"on_demand","body":"intro\nline\noutro"}`, f.adminToken)
	require.Equal(t, http.StatusOK, preview.StatusCode)
	var mergePreview service.WikiMergePreview
	require.Nil(t, json.NewDecoder(preview.Body).Decode(&mergePreview))
	require.Equal(t, "Fields renamed", mergePreview.Title)
	require.Equal(t, "hidden", string(mergePreview.AgentAccess))
}

func TestWiki_RestoringAnAlwaysPageNeedsTheOwnerAndFreeBudget(t *testing.T) {
	f := newWikiFixture(t, "wiki-restore-always")
	member := f.userWithRole(t, "wiki-restore-member@test.sk", "member")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Always restored", Body: strings.Repeat("a", 400), AgentAccess: "always"})
	restoreUrl := fmt.Sprintf("/api/private/wiki/page/%d/restore", page.IdPage)
	settingsUrl := fmt.Sprintf("/api/private/project/%d/wiki/settings", f.idProject)

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", page.IdPage), "", member).StatusCode)
	require.Equal(t, http.StatusForbidden, Request(t, f.app, "POST", restoreUrl, "", member).StatusCode)

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "PUT", settingsUrl, `{"alwaysTokenLimit":10}`, f.adminToken).StatusCode)
	require.Equal(t, http.StatusUnprocessableEntity, Request(t, f.app, "POST", restoreUrl, "", f.adminToken).StatusCode)

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "PUT", settingsUrl, `{"alwaysTokenLimit":200000}`, f.adminToken).StatusCode)
	require.Equal(t, http.StatusOK, Request(t, f.app, "POST", restoreUrl, "", f.adminToken).StatusCode)
}

func TestWiki_AgentUsersNeverSeeHiddenPages(t *testing.T) {
	f := newWikiFixture(t, "wiki-hidden-agent")
	agent := f.agentUser(t, "wiki-hidden-agent@test.sk", "member")
	word := fmt.Sprintf("zzsecret%d", time.Now().UnixNano())
	hidden := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Secret", Body: word, AgentAccess: "hidden"})
	open := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Open", Body: word + " [[Secret]]"})

	for _, node := range f.tree(t, agent).Nodes {
		require.NotEqual(t, hidden.IdPage, node.IdPage)
	}
	require.Equal(t, http.StatusNotFound, Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/page/project/%s", f.idProject, hidden.Slug), "", agent).StatusCode)
	require.Equal(t, http.StatusNotFound, Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/version/1", hidden.IdPage), "", agent).StatusCode)

	search := readBody(t, Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/search?q=%s", f.idProject, word), "", agent))
	require.Contains(t, search, `"title":"Open"`)
	require.NotContains(t, search, `"title":"Secret"`)

	view := f.view(t, agent, "project", open.Slug)
	require.Nil(t, view.Links[0].IdPage, "a link to a hidden page looks like a missing page to an agent")

	require.Equal(t, http.StatusOK, Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/page/project/%s", f.idProject, hidden.Slug), "", f.adminToken).StatusCode)

	iss := createIssue(t, f.app, f.adminToken, f.idProject, "agent links")
	link := func(idPage int64) int {
		return Request(t, f.app, "POST", fmt.Sprintf("/api/private/issue/%d/wiki", iss.IdIssue), fmt.Sprintf(`{"idPage":%d}`, idPage), agent).StatusCode
	}
	require.Equal(t, http.StatusNotFound, link(open.IdPage+1_000_000))
	require.Equal(t, http.StatusNotFound, link(hidden.IdPage), "an agent cannot tell a hidden page from a missing one")
	require.Equal(t, http.StatusNoContent, link(open.IdPage))
}

func TestWiki_SharedPageBacklinksOnlyComeFromSpacesTheReaderSees(t *testing.T) {
	f := newWikiFixture(t, "wiki-backlinks-a")
	other := f.otherProject(t, "wiki-backlinks-b")
	outsider := other.userWithRole(t, "wiki-backlinks-outsider@test.sk", "member")
	shared := f.mustCreate(t, model.CreateWikiPageReq{Space: "instance", Title: uniqueWikiTitle("Git")})
	f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Client X credentials", Body: "see [[" + shared.Title + "]]"})

	require.Len(t, f.view(t, f.adminToken, "instance", shared.Slug).Backlinks.Items, 1)
	require.Empty(t, other.view(t, outsider, "instance", shared.Slug).Backlinks.Items)
}

func TestWiki_TokenLimitNeverBlocksShrinkingAndZeroOptsOutOfShared(t *testing.T) {
	f := newWikiFixture(t, "wiki-budget-shrink")
	optedOut := f.otherProject(t, "wiki-budget-zero")
	settings := func(fixture *wikiFixture, limit int) {
		require.Equal(t, http.StatusNoContent, Request(t, fixture.app, "PUT", fmt.Sprintf("/api/private/project/%d/wiki/settings", fixture.idProject),
			fmt.Sprintf(`{"alwaysTokenLimit":%d}`, limit), fixture.adminToken).StatusCode)
	}
	settings(f, 200000)
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Big always", Body: strings.Repeat("a", 4000), AgentAccess: "always"})
	settings(f, 10)

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 1, Title: "Big always", Body: strings.Repeat("a", 3000), AgentAccess: "always",
	}).StatusCode, "a smaller always page saves even when the project is over its limit")
	require.Equal(t, http.StatusUnprocessableEntity, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: 2, Title: "Big always", Body: strings.Repeat("a", 3500), AgentAccess: "always",
	}).StatusCode)
	settings(f, 200000)

	settings(optedOut, 0)
	res, shared := f.create(t, f.adminToken, model.CreateWikiPageReq{Space: "instance", Title: uniqueWikiTitle("Shared always"), Body: "short", AgentAccess: "always"})
	require.Equal(t, http.StatusOK, res.StatusCode, "a project with limit 0 does not block shared always pages")
	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", shared.IdPage), "", f.adminToken).StatusCode)
	settings(optedOut, 12000)
}

func TestWiki_OwnerPicksTheWikiHomePage(t *testing.T) {
	f := newWikiFixture(t, "wiki-home-page")
	member := f.userWithRole(t, "wiki-home-member@test.sk", "member")
	other := f.otherProject(t, "wiki-home-other")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Start here"})
	foreign := other.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Elsewhere"})
	homeUrl := fmt.Sprintf("/api/private/project/%d/wiki/home", f.idProject)
	setHome := func(token string, idPage int64) int {
		return Request(t, f.app, "PUT", homeUrl, fmt.Sprintf(`{"idPage":%d}`, idPage), token).StatusCode
	}

	require.Nil(t, f.tree(t, f.adminToken).Spaces[1].IdHomePage)
	require.Equal(t, http.StatusForbidden, setHome(member, page.IdPage))
	require.Equal(t, http.StatusBadRequest, setHome(f.adminToken, foreign.IdPage))
	require.Equal(t, http.StatusNoContent, setHome(f.adminToken, page.IdPage))
	home := f.tree(t, member).Spaces[1].IdHomePage
	require.NotNil(t, home)
	require.Equal(t, page.IdPage, *home)

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", page.IdPage), "", f.adminToken).StatusCode)
	require.Nil(t, f.tree(t, f.adminToken).Spaces[1].IdHomePage, "a home page in the trash is not offered")

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "PUT", homeUrl, `{"idPage":null}`, f.adminToken).StatusCode)
}

func TestWiki_TreeCountsTrashItemsTheReaderCanSee(t *testing.T) {
	f := newWikiFixture(t, "wiki-trash-count")
	agent := f.agentUser(t, "wiki-trash-count-agent@test.sk", "member")
	ops := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Ops"})
	f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Deploy", IdParent: &ops.IdPage})
	secret := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: "Secret", AgentAccess: "hidden"})
	before := f.tree(t, f.adminToken).TrashCount
	agentBefore := f.tree(t, agent).TrashCount

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", ops.IdPage), "", f.adminToken).StatusCode)
	require.Equal(t, before+1, f.tree(t, f.adminToken).TrashCount, "a page trashed with its subpages is one item")

	require.Equal(t, http.StatusNoContent, Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/wiki/page/%d", secret.IdPage), "", f.adminToken).StatusCode)
	require.Equal(t, before+2, f.tree(t, f.adminToken).TrashCount)
	require.Equal(t, agentBefore+1, f.tree(t, agent).TrashCount, "agents do not count hidden pages")

	require.Equal(t, http.StatusOK, Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/page/%d/restore", ops.IdPage), "", f.adminToken).StatusCode)
	require.Equal(t, before+1, f.tree(t, f.adminToken).TrashCount)
}

func TestWiki_LinkedTasksAndBacklinksComeInPagesWithTheOpenAndNewestFirst(t *testing.T) {
	f := newWikiFixture(t, "wiki-paging")
	target := f.mustCreate(t, model.CreateWikiPageReq{Space: "instance", Title: uniqueWikiTitle("Conventions")})
	var issues []model.Issue
	for i := 0; i < 25; i++ {
		body, _ := json.Marshal(map[string]any{"idProject": f.idProject, "title": fmt.Sprintf("task %d", i), "description": "See [[shared:" + target.Slug + "]]."})
		res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/issue", f.idProject), string(body), f.adminToken)
		issueBody := readBody(t, res)
		require.Equal(t, http.StatusOK, res.StatusCode, issueBody)
		var created model.Issue
		require.Nil(t, json.Unmarshal([]byte(issueBody), &created))
		issues = append(issues, created)
	}
	closed := issues[len(issues)-1]
	_, err := f.app.Pool.Exec(context.Background(), `
		UPDATE issues.issue SET id_state = (
			SELECT s.id_state FROM issues.state s
			INNER JOIN projects.project_issue_state pis ON pis.id_state = s.id_state
			WHERE pis.id_project = $1 AND s.final LIMIT 1)
		WHERE id_issue = $2`, f.idProject, closed.IdIssue)
	require.Nil(t, err)
	for i := 0; i < 23; i++ {
		f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle(fmt.Sprintf("Source %02d", i)), Body: "[[shared:" + target.Slug + "]]"})
	}

	other := f.otherProject(t, "wiki-paging-other")
	body, _ := json.Marshal(map[string]any{"idProject": other.idProject, "title": "elsewhere", "description": "See [[shared:" + target.Slug + "]]."})
	require.Equal(t, http.StatusOK, Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/issue", other.idProject), string(body), f.adminToken).StatusCode)

	view := f.view(t, f.adminToken, "instance", target.Slug)
	require.Len(t, view.Issues.Items, 20)
	require.Equal(t, 26, view.Issues.Total)
	require.Equal(t, other.idProject, view.Issues.Items[0].IdProject, "the newest task comes first and keeps its own project")
	require.Equal(t, issues[len(issues)-2].IdIssue, view.Issues.Items[1].IdIssue)
	for _, item := range view.Issues.Items {
		require.False(t, item.IsClosed, "open tasks come before closed ones")
	}
	require.Len(t, view.Backlinks.Items, 20)
	require.Equal(t, 23, view.Backlinks.Total)

	rest := Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/issues?offset=20&limit=20", target.IdPage), "", f.adminToken)
	restBody := readBody(t, rest)
	require.Equal(t, http.StatusOK, rest.StatusCode, restBody)
	var restIssues model.WikiPageIssueList
	require.Nil(t, json.Unmarshal([]byte(restBody), &restIssues))
	require.Len(t, restIssues.Items, 6)
	require.Equal(t, 26, restIssues.Total)
	require.Equal(t, closed.IdIssue, restIssues.Items[5].IdIssue)
	require.True(t, restIssues.Items[5].IsClosed)
	require.NotNil(t, restIssues.Items[5].StateName)

	seen := map[int64]bool{}
	for _, item := range append(view.Issues.Items, restIssues.Items...) {
		require.False(t, seen[item.IdIssue], "pages never repeat a task")
		seen[item.IdIssue] = true
	}

	moreLinks := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/backlinks/%d?offset=20", f.idProject, target.IdPage), "", f.adminToken)
	moreBody := readBody(t, moreLinks)
	require.Equal(t, http.StatusOK, moreLinks.StatusCode, moreBody)
	var backlinks model.WikiBacklinkList
	require.Nil(t, json.Unmarshal([]byte(moreBody), &backlinks))
	require.Len(t, backlinks.Items, 3)
	require.Equal(t, 23, backlinks.Total)

	viewer := f.userWithRole(t, "wiki-paging-viewer@test.sk", "viewer")
	var limited model.WikiPageIssueList
	limitedRes := Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/issues", target.IdPage), "", viewer)
	require.Equal(t, http.StatusOK, limitedRes.StatusCode)
	require.Nil(t, json.NewDecoder(limitedRes.Body).Decode(&limited))
	require.Equal(t, 25, limited.Total, "tasks from projects the reader cannot see are not counted")

	outsider := createUserAndLogin(t, f.app, f.adminToken, "wiki-paging-outsider@test.sk")
	require.Equal(t, http.StatusForbidden, Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/backlinks/%d", f.idProject, target.IdPage), "", outsider).StatusCode)
	require.Equal(t, http.StatusBadRequest, Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/issues?offset=-1", target.IdPage), "", f.adminToken).StatusCode)
}

func newWikiFixture(t *testing.T, name string) *wikiFixture {
	app := Setup(t)
	token := Token(t, app)
	return &wikiFixture{app: app, adminToken: token, idProject: createProject(t, app, token, name)}
}

func (f *wikiFixture) otherProject(t *testing.T, name string) *wikiFixture {
	return &wikiFixture{app: f.app, adminToken: f.adminToken, idProject: createProject(t, f.app, f.adminToken, name)}
}

func (f *wikiFixture) userWithRole(t *testing.T, email, role string) string {
	token := createUserAndLogin(t, f.app, f.adminToken, email)
	if role != "" {
		idUser := idOfUser(t, f.app, f.adminToken, email)
		res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/member/user", f.idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"%s"}`, idUser, role), f.adminToken)
		require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	}
	return token
}

func (f *wikiFixture) create(t *testing.T, token string, req model.CreateWikiPageReq) (*http.Response, model.WikiPage) {
	body, _ := json.Marshal(req)
	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/wiki/page", f.idProject), string(body), token)
	var page model.WikiPage
	if res.StatusCode == http.StatusOK {
		require.Nil(t, json.NewDecoder(res.Body).Decode(&page))
	}
	return res, page
}

func (f *wikiFixture) mustCreate(t *testing.T, req model.CreateWikiPageReq) model.WikiPage {
	res, page := f.create(t, f.adminToken, req)
	require.Equal(t, http.StatusOK, res.StatusCode)
	return page
}

func (f *wikiFixture) save(t *testing.T, token string, idPage int64, req model.SaveWikiPageReq) *http.Response {
	body, _ := json.Marshal(req)
	return Request(t, f.app, "PUT", fmt.Sprintf("/api/private/wiki/page/%d", idPage), string(body), token)
}

func (f *wikiFixture) view(t *testing.T, token, space, slug string) model.WikiPageView {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/page/%s/%s", f.idProject, space, slug), "", token)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var view model.WikiPageView
	require.Nil(t, json.Unmarshal([]byte(body), &view))
	return view
}

func (f *wikiFixture) tree(t *testing.T, token string) model.WikiTree {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/tree", f.idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tree model.WikiTree
	require.Nil(t, json.NewDecoder(res.Body).Decode(&tree))
	return tree
}

func (f *wikiFixture) versions(t *testing.T, idPage int64) []model.WikiPageVersionSummary {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/version", idPage), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var versions []model.WikiPageVersionSummary
	require.Nil(t, json.NewDecoder(res.Body).Decode(&versions))
	return versions
}

func (f *wikiFixture) projectNodes(t *testing.T) []*model.WikiTreeNode {
	tree := f.tree(t, f.adminToken)
	nodes := make([]*model.WikiTreeNode, 0)
	for _, node := range tree.Nodes {
		if node.IdSpace == tree.Spaces[1].IdSpace {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func uniqueWikiTitle(prefix string) string {
	return fmt.Sprintf("%s %d", prefix, time.Now().UnixNano())
}

func (f *wikiFixture) agentUser(t *testing.T, email, role string) string {
	token := f.userWithRole(t, email, role)
	idUser := idOfUser(t, f.app, f.adminToken, email)
	_, err := f.app.Pool.Exec(context.Background(), "UPDATE users.user SET is_agent = TRUE WHERE id_user = $1", idUser)
	require.Nil(t, err)
	f.app.Cache.Del(context.Background(), token)
	res := Request(t, f.app, "POST", "/api/public/login", `{"email":"`+email+`","password":"kreslo"}`, "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var login struct {
		Token string `json:"token"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&login))
	return login.Token
}
