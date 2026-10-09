package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWikiProposal_AgentProposesInImplementationAndARetryUpdatesTheSameProposal(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-upsert")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Deploy runbook"), Body: "old steps"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal upsert")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	header := fmt.Sprint(run.IdRun)

	first := f.suggest(t, header, map[string]any{"kind": "update", "slug": page.Slug, "body": "new steps", "base_version": 1, "reason": "the deploy script moved"})
	require.Equal(t, constants.WikiProposalStateOpen, first.State)
	require.False(t, first.Updated)

	again := f.suggest(t, header, map[string]any{"kind": "update", "slug": page.Slug, "body": "newer steps", "base_version": 1, "reason": "review asked for more"})
	require.Equal(t, first.Id, again.Id, "a retry updates the proposal instead of adding another")
	require.True(t, again.Updated)

	title := uniqueWikiTitle("Retention")
	created := f.suggest(t, header, map[string]any{"kind": "create", "title": title, "body": "keep 7 days", "reason": "new cleanup job"})
	createdAgain := f.suggest(t, header, map[string]any{"kind": "create", "title": title, "body": "keep 14 days", "reason": "new cleanup job"})
	require.Equal(t, created.Id, createdAgain.Id)

	listed := f.callTool(t, f.agentToken, header, "list_my_wiki_proposals", map[string]any{"project_id": f.idProject, "run_id": run.IdRun})
	require.False(t, listed.IsError, listed.Text)
	var views []model.WikiProposalAgentView
	require.NoError(t, json.Unmarshal([]byte(listed.Text), &views))
	require.Len(t, views, 2)
	require.Equal(t, "review asked for more", views[0].Reason)

	proposals := f.issueProposals(t, iss.IdIssue)
	require.Len(t, proposals, 2)
	require.Equal(t, "newer steps", *proposals[0].Body)
	require.Equal(t, 1, *proposals[0].BaseVersion)
	require.Equal(t, "keep 14 days", *proposals[1].Body)

	require.Equal(t, "old steps", f.view(t, f.adminToken, "project", page.Slug).Page.Body, "a proposal never writes the wiki")
}

func TestWikiProposal_OnlyTheImplementationStageOfTheAgentsOwnRunMayPropose(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-guard")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Guarded"), Body: "text"})
	hidden := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Hidden"), Body: "secret", AgentAccess: "hidden"})
	update := map[string]any{"kind": "update", "slug": page.Slug, "body": "x", "base_version": 1, "reason": "r"}

	designIssue := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal design")
	designRun, _ := f.startRun(t, designIssue.IdIssue, constants.StageDesign)
	design := f.callTool(t, f.agentToken, fmt.Sprint(designRun.IdRun), "suggest_wiki_change", withProject(f, update))
	require.True(t, design.IsError)
	require.Contains(t, design.Text, "implementation stage")

	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal guard")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	header := fmt.Sprint(run.IdRun)

	require.True(t, f.callTool(t, f.agentToken, "", "suggest_wiki_change", withProject(f, update)).IsError, "outside a run there is nothing to attach the proposal to")
	require.True(t, f.callTool(t, f.adminToken, header, "suggest_wiki_change", withProject(f, update)).IsError, "a person edits the wiki instead")

	noBase := f.callTool(t, f.agentToken, header, "suggest_wiki_change", withProject(f, map[string]any{"kind": "update", "slug": page.Slug, "body": "x", "reason": "r"}))
	require.True(t, noBase.IsError)
	require.Contains(t, noBase.Text, "base_version")

	hiddenPage := f.callTool(t, f.agentToken, header, "suggest_wiki_change", withProject(f, map[string]any{"kind": "delete", "slug": hidden.Slug, "reason": "r"}))
	require.True(t, hiddenPage.IsError)
	require.Contains(t, hiddenPage.Text, "not found")

	taken := f.callTool(t, f.agentToken, header, "suggest_wiki_change", withProject(f, map[string]any{"kind": "create", "title": page.Title, "body": "x", "reason": "r"}))
	require.True(t, taken.IsError)
	require.Contains(t, taken.Text, "kind=update")

	require.Empty(t, f.issueProposals(t, iss.IdIssue))
	require.Empty(t, f.issueProposals(t, designIssue.IdIssue))
}

func TestWikiProposal_ApprovedBeforeTheMergeIsPublishedWhenThePullRequestMerges(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-approve")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Approved"), Body: "one\ntwo\nthree\nfour\n"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal approve")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	header := fmt.Sprint(run.IdRun)
	proposal := f.suggest(t, header, map[string]any{"kind": "update", "slug": page.Slug, "body": "one\nTWO\nthree\nfour\n", "base_version": 1, "reason": "r"})
	f.setRunPhase(t, run.IdRun, constants.PhasePrOpen)
	require.Len(t, f.openProposals(t), 1, "an open proposal is listed before the merge")
	require.Zero(t, f.tree(t, f.adminToken).ProposalCount, "the wiki counts only proposals that need a person after the merge")

	res := f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{Note: "checked"})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, constants.WikiProposalStateApproved, f.issueProposals(t, iss.IdIssue)[0].State)
	require.Equal(t, "one\ntwo\nthree\nfour\n", f.view(t, f.adminToken, "project", page.Slug).Page.Body, "approving does not write the wiki before the merge")

	again := f.suggest(t, header, map[string]any{"kind": "update", "slug": page.Slug, "body": "one\nTWO!\nthree\nfour\n", "base_version": 1, "reason": "review asked for more"})
	require.Equal(t, constants.WikiProposalStateOpen, again.State, "a new attempt that changes the proposal takes the approval back")
	res = f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{Note: "checked"})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: page.Title, Body: "one\ntwo\nthree\nFOUR by person\n", AgentAccess: "on_demand"}).StatusCode)
	injector.GetWikiProposalService().RunFinished(context.Background(), f.setRunPhase(t, run.IdRun, constants.PhaseDone))

	view := f.view(t, f.adminToken, "project", page.Slug)
	require.Equal(t, "one\nTWO!\nthree\nFOUR by person\n", view.Page.Body, "the approved change merges with the edit made meanwhile")
	latest := f.versions(t, page.IdPage)[0]
	require.Equal(t, fmt.Sprintf("Agent proposal from #%d (run #%d): checked", iss.IdIssuePublic, run.IdRun), latest.Note)
	require.Equal(t, idOfUser(t, f.app, f.adminToken, "test@test.sk"), *latest.CreateBy, "the person who approved is the author")
	published := f.issueProposals(t, iss.IdIssue)[0]
	require.Equal(t, constants.WikiProposalStateAccepted, published.State)
	require.Equal(t, view.Page.VersionNo, *published.ResultVersion)
	require.Empty(t, f.openProposals(t))
	require.Zero(t, f.notificationCount(t, constants.NotificationTypeWikiProposalReady, idOfUser(t, f.app, f.adminToken, "test@test.sk"), iss.IdIssue), "nothing is left to review")
}

func TestWikiProposal_AnApprovedChangeThatNoLongerMergesNeedsResolving(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-resolve")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Resolve"), Body: "alpha\n"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal resolve")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	proposal := f.suggest(t, fmt.Sprint(run.IdRun), map[string]any{"kind": "update", "slug": page.Slug, "body": "alpha by agent\n", "base_version": 1, "reason": "r"})
	f.setRunPhase(t, run.IdRun, constants.PhasePrOpen)
	require.Equal(t, http.StatusOK, f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{}).StatusCode)

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: page.Title, Body: "alpha by person\n", AgentAccess: "on_demand"}).StatusCode)
	injector.GetWikiProposalService().RunFinished(context.Background(), f.setRunPhase(t, run.IdRun, constants.PhaseDone))

	require.Equal(t, constants.WikiProposalStateNeedsResolving, f.issueProposals(t, iss.IdIssue)[0].State)
	require.Equal(t, "alpha by person\n", f.view(t, f.adminToken, "project", page.Slug).Page.Body)
	require.Equal(t, 1, f.tree(t, f.adminToken).ProposalCount, "a proposal to resolve is counted in the wiki")
	idAdmin := idOfUser(t, f.app, f.adminToken, "test@test.sk")
	require.Equal(t, 1, f.notificationCount(t, constants.NotificationTypeWikiProposalConflict, idAdmin, iss.IdIssue), "the person who approved hears about it")
	require.Equal(t, model.NotificationBodyWikiProposal{Count: 1, IdProposal: proposal.Id, Title: page.Title}, f.notificationBody(t, constants.NotificationTypeWikiProposalConflict, idAdmin, iss.IdIssue))
	require.Equal(t, f.agentName(t), f.notificationActor(t, constants.NotificationTypeWikiProposalConflict, idAdmin, iss.IdIssue))

	clash := f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{})
	require.Equal(t, http.StatusConflict, clash.StatusCode)
	require.Contains(t, readBody(t, clash), `"merge"`)
	resolved := "alpha by both\n"
	res := f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{BaseVersion: 2, Body: &resolved})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, resolved, f.view(t, f.adminToken, "project", page.Slug).Page.Body)
	require.Equal(t, constants.WikiProposalStateAccepted, f.issueProposals(t, iss.IdIssue)[0].State)
}

func TestWikiProposal_ClosingThePullRequestDiscardsEvenApprovedProposals(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-closed")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Closed"), Body: "text"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal closed")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	proposal := f.suggest(t, fmt.Sprint(run.IdRun), map[string]any{"kind": "delete", "slug": page.Slug, "reason": "r"})
	f.setRunPhase(t, run.IdRun, constants.PhasePrOpen)
	require.Equal(t, http.StatusOK, f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{}).StatusCode)

	injector.GetWikiProposalService().RunFinished(context.Background(), f.setRunPhase(t, run.IdRun, constants.PhaseCancelled))
	require.Equal(t, constants.WikiProposalDiscarded, f.issueProposals(t, iss.IdIssue)[0].State)
	accept := f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{})
	require.Equal(t, http.StatusConflict, accept.StatusCode)
	require.Contains(t, readBody(t, accept), "WIKI_PROPOSAL_NOT_READY")
	require.Empty(t, f.openProposals(t), "a discarded proposal leaves the wiki list")
	require.Equal(t, "text", f.view(t, f.adminToken, "project", page.Slug).Page.Body)

	f.setRunPhase(t, run.IdRun, constants.PhasePrOpen)
	require.Equal(t, constants.WikiProposalStateApproved, f.issueProposals(t, iss.IdIssue)[0].State, "a run brought back to life brings its proposals back")
}

func TestWikiProposal_AcceptingAnUpdateMergesWithEditsMadeSinceTheBase(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-merge")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Branches"), Body: "one\ntwo\nthree\n"})
	other := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Worktrees"), Body: "alpha\n"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal merge")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	header := fmt.Sprint(run.IdRun)
	merged := f.suggest(t, header, map[string]any{"kind": "update", "slug": page.Slug, "body": "one\ntwo\nTHREE by agent\n", "base_version": 1, "reason": "r"})
	conflicting := f.suggest(t, header, map[string]any{"kind": "update", "slug": other.Slug, "body": "alpha by agent\n", "base_version": 1, "reason": "r"})

	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, page.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: page.Title, Body: "ONE by person\ntwo\nthree\n", AgentAccess: "on_demand"}).StatusCode)
	require.Equal(t, http.StatusOK, f.save(t, f.adminToken, other.IdPage, model.SaveWikiPageReq{BaseVersion: 1, Title: other.Title, Body: "alpha by person\n", AgentAccess: "on_demand"}).StatusCode)
	f.setRunPhase(t, run.IdRun, constants.PhaseDone)

	mergedDetail := f.proposalDetail(t, merged.Id)
	require.Contains(t, mergedDetail.Diff, "+THREE by agent")
	require.Contains(t, mergedDetail.Diff, "-three")
	require.Zero(t, mergedDetail.Conflicts, "the person's edit does not overlap the agent's")
	require.Positive(t, f.proposalDetail(t, conflicting.Id).Conflicts)

	res := f.accept(t, f.adminToken, merged.Id, model.WikiProposalAcceptReq{Note: "checked"})
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	view := f.view(t, f.adminToken, "project", page.Slug)
	require.Equal(t, "ONE by person\ntwo\nTHREE by agent\n", view.Page.Body, "the person's edit since the base survives")
	require.Equal(t, 3, view.Page.VersionNo)

	versions := f.versions(t, page.IdPage)
	latest := versions[0]
	require.Equal(t, 3, latest.VersionNo)
	require.Equal(t, fmt.Sprintf("Agent proposal from #%d (run #%d): checked", iss.IdIssuePublic, run.IdRun), latest.Note)
	require.NotNil(t, latest.CreateBy)
	require.Equal(t, idOfUser(t, f.app, f.adminToken, "test@test.sk"), *latest.CreateBy, "the person who accepted is the author")

	proposals := f.issueProposals(t, iss.IdIssue)
	require.Equal(t, constants.WikiProposalStateAccepted, proposals[0].State)
	require.Equal(t, 3, *proposals[0].ResultVersion)
	require.Equal(t, http.StatusConflict, f.accept(t, f.adminToken, merged.Id, model.WikiProposalAcceptReq{}).StatusCode, "a proposal is accepted once")

	clash := f.accept(t, f.adminToken, conflicting.Id, model.WikiProposalAcceptReq{})
	clashBody := readBody(t, clash)
	require.Equal(t, http.StatusConflict, clash.StatusCode)
	require.Contains(t, clashBody, `"merge"`, "an overlapping edit comes back as a conflict to resolve in the editor")
	require.Equal(t, constants.WikiProposalReady, f.issueProposals(t, iss.IdIssue)[1].State)
	require.Equal(t, "alpha by person\n", f.view(t, f.adminToken, "project", other.Slug).Page.Body)

	resolved := "alpha by both\n"
	res = f.accept(t, f.adminToken, conflicting.Id, model.WikiProposalAcceptReq{BaseVersion: 2, Body: &resolved})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	require.Equal(t, resolved, f.view(t, f.adminToken, "project", other.Slug).Page.Body)
}

func TestWikiProposal_CreateMoveAndDeleteApply(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-kinds")
	parent := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Operations"), Body: "ops"})
	moved := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Backups"), Body: "backups"})
	deleted := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Old cron"), Body: "cron"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal kinds")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	header := fmt.Sprint(run.IdRun)

	title := uniqueWikiTitle("Cleanup job")
	create := f.suggest(t, header, map[string]any{"kind": "create", "title": title, "summary": "what it removes", "body": "runs nightly", "parent": parent.Slug, "reason": "new job"})
	move := f.suggest(t, header, map[string]any{"kind": "move", "slug": moved.Slug, "parent": parent.Slug, "reason": "belongs to ops"})
	remove := f.suggest(t, header, map[string]any{"kind": "delete", "slug": deleted.Slug, "reason": "cron is gone"})
	f.setRunPhase(t, run.IdRun, constants.PhaseDone)

	for _, id := range []int64{create.Id, move.Id, remove.Id} {
		res := f.accept(t, f.adminToken, id, model.WikiProposalAcceptReq{})
		require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	}

	createdView := f.view(t, f.adminToken, "project", create.Slug)
	require.Equal(t, title, createdView.Page.Title)
	require.Equal(t, "runs nightly", createdView.Page.Body)
	require.Equal(t, "what it removes", createdView.Page.Summary)
	require.Equal(t, parent.IdPage, *createdView.Page.IdParent)
	require.Equal(t, parent.IdPage, *f.view(t, f.adminToken, "project", moved.Slug).Page.IdParent)

	gone := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/page/project/%s", f.idProject, deleted.Slug), "", f.adminToken)
	require.Equal(t, http.StatusNotFound, gone.StatusCode)
	trash := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/trash", f.idProject), "", f.adminToken)
	require.Contains(t, readBody(t, trash), deleted.Title, "a delete proposal moves the page to the trash")

	proposals := f.issueProposals(t, iss.IdIssue)
	for _, proposal := range proposals {
		require.Equal(t, constants.WikiProposalStateAccepted, proposal.State)
	}
	require.Equal(t, createdView.Page.IdPage, *proposals[0].IdPage, "an accepted new page is linked to its proposal")
}

func TestWikiProposal_RejectingLeavesACommentInTheTask(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-reject")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Style"), Body: "tabs"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal reject")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	proposal := f.suggest(t, fmt.Sprint(run.IdRun), map[string]any{"kind": "update", "slug": page.Slug, "body": "spaces", "base_version": 1, "reason": "r"})
	f.setRunPhase(t, run.IdRun, constants.PhaseDone)

	viewer := f.userWithRole(t, "wiki-proposal-reject-viewer@test.sk", "viewer")
	require.Equal(t, http.StatusForbidden, f.reject(t, viewer, proposal.Id, "no").StatusCode, "only someone who may edit the wiki decides")

	res := f.reject(t, f.adminToken, proposal.Id, "we keep tabs")
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	rejected := f.issueProposals(t, iss.IdIssue)[0]
	require.Equal(t, constants.WikiProposalStateRejected, rejected.State)
	require.Equal(t, "we keep tabs", *rejected.DecisionNote)

	messages := Request(t, f.app, "GET", fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=4", iss.IdIssue), "", f.adminToken)
	messagesBody := readBody(t, messages)
	require.Contains(t, messagesBody, "Rejected the wiki proposal")
	require.Contains(t, messagesBody, "we keep tabs")

	require.Equal(t, http.StatusConflict, f.accept(t, f.adminToken, proposal.Id, model.WikiProposalAcceptReq{}).StatusCode)
	require.Equal(t, "tabs", f.view(t, f.adminToken, "project", page.Slug).Page.Body)
}

func TestWikiProposal_AgentKeysHaveNoWayToWriteTheWiki(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-agent-write")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Protected"), Body: "keep"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal agent write")
	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	proposal := f.suggest(t, fmt.Sprint(run.IdRun), map[string]any{"kind": "update", "slug": page.Slug, "body": "agent", "base_version": 1, "reason": "r"})
	f.setRunPhase(t, run.IdRun, constants.PhaseDone)

	pagePath := fmt.Sprintf("/api/private/wiki/page/%d", page.IdPage)
	for name, res := range map[string]*http.Response{
		"create":  Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/wiki/page", f.idProject), fmt.Sprintf(`{"space":"project","title":%q}`, uniqueWikiTitle("By agent")), f.agentToken),
		"save":    Request(t, f.app, "PUT", pagePath, fmt.Sprintf(`{"baseVersion":1,"title":%q,"body":"agent","agentAccess":"on_demand"}`, page.Title), f.agentToken),
		"move":    Request(t, f.app, "PUT", pagePath+"/position", `{"idParent":null}`, f.agentToken),
		"revert":  Request(t, f.app, "POST", pagePath+"/revert/1", "", f.agentToken),
		"trash":   Request(t, f.app, "DELETE", pagePath, "", f.agentToken),
		"upsert":  f.upsert(t, f.agentToken, map[string]any{"slug": page.Slug, "title": page.Title, "body": "agent", "baseVersion": 1}),
		"accept":  f.accept(t, f.agentToken, proposal.Id, model.WikiProposalAcceptReq{}),
		"reject":  f.reject(t, f.agentToken, proposal.Id, ""),
		"publish": Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/wiki/home", f.idProject), fmt.Sprintf(`{"idPage":%d}`, page.IdPage), f.agentToken),
	} {
		require.Equal(t, http.StatusForbidden, res.StatusCode, "%s: %s", name, readBody(t, res))
	}

	view := f.view(t, f.adminToken, "project", page.Slug)
	require.Equal(t, "keep", view.Page.Body)
	require.Equal(t, 1, view.Page.VersionNo)
	require.Equal(t, constants.WikiProposalReady, f.issueProposals(t, iss.IdIssue)[0].State)
}

func TestWikiProposal_ReadyProposalsNotifyTheAuthorAndAssigneeOnce(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-notify")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Notified"), Body: "text"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal notify")
	f.userWithRole(t, "wiki-proposal-notify-assignee@test.sk", "member")
	idAssignee := idOfUser(t, f.app, f.adminToken, "wiki-proposal-notify-assignee@test.sk")
	_, err := f.app.Pool.Exec(context.Background(), `UPDATE issues.issue SET assigned_to = $2 WHERE id_issue = $1`, iss.IdIssue, idAssignee)
	require.NoError(t, err)

	run, _ := f.startRun(t, iss.IdIssue, constants.StageImplementation)
	header := fmt.Sprint(run.IdRun)
	first := f.suggest(t, header, map[string]any{"kind": "update", "slug": page.Slug, "body": "a", "base_version": 1, "reason": "r"})
	f.suggest(t, header, map[string]any{"kind": "create", "title": uniqueWikiTitle("Notified new"), "body": "b", "reason": "r"})

	finished := f.setRunPhase(t, run.IdRun, constants.PhaseDone)
	injector.GetWikiProposalService().RunFinished(context.Background(), finished)

	idAdmin := idOfUser(t, f.app, f.adminToken, "test@test.sk")
	require.Equal(t, 1, f.notificationCount(t, constants.NotificationTypeWikiProposalReady, idAdmin, iss.IdIssue))
	require.Equal(t, 1, f.notificationCount(t, constants.NotificationTypeWikiProposalReady, idAssignee, iss.IdIssue))
	require.Zero(t, f.notificationCount(t, constants.NotificationTypeWikiProposalReady, f.idAgent, iss.IdIssue))
	require.Equal(t, model.NotificationBodyWikiProposal{Count: 2, IdProposal: first.Id, Title: page.Title}, f.notificationBody(t, constants.NotificationTypeWikiProposalReady, idAssignee, iss.IdIssue),
		"the notification links the first proposal of the run")
	require.Equal(t, f.agentName(t), f.notificationActor(t, constants.NotificationTypeWikiProposalReady, idAssignee, iss.IdIssue), "the agent of the run is the actor")

	quietIssue := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal none")
	quietRun, _ := f.startRun(t, quietIssue.IdIssue, constants.StageImplementation)
	injector.GetWikiProposalService().RunFinished(context.Background(), f.setRunPhase(t, quietRun.IdRun, constants.PhaseDone))
	require.Zero(t, f.notificationCount(t, constants.NotificationTypeWikiProposalReady, idAdmin, quietIssue.IdIssue), "a run without proposals notifies nobody")
}

func TestWikiProposal_TheNextImplementationAttemptSeesItsOpenProposals(t *testing.T) {
	f := newWikiAgentFixture(t, "wiki-proposal-prompt")
	page := f.mustCreate(t, model.CreateWikiPageReq{Space: "project", Title: uniqueWikiTitle("Prompted"), Body: "text"})
	iss := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal prompt")
	run, _, first := f.dispatch(t, iss.IdIssue, constants.StageImplementation)
	require.Empty(t, first.WikiProposals)

	f.suggest(t, fmt.Sprint(run.IdRun), map[string]any{"kind": "update", "slug": page.Slug, "body": "new", "base_version": 1, "reason": "renamed the flag"})
	task, err := injector.GetAgentTaskRepository().Insert(context.Background(), run.IdRun, f.idAgent, constants.StageImplementation, 2)
	require.NoError(t, err)
	bundle := f.dispatchTask(t, run, task)
	require.Len(t, bundle.WikiProposals, 1)
	require.Equal(t, page.Slug, bundle.WikiProposals[0].Slug)
	require.Equal(t, "renamed the flag", bundle.WikiProposals[0].Reason)

	designIssue := createIssue(t, f.app, f.adminToken, f.idProject, "wiki proposal design prompt")
	_, _, design := f.dispatch(t, designIssue.IdIssue, constants.StageDesign)
	require.Nil(t, design.WikiProposals, "only the implementation stage gets proposals")
}

func withProject(f *wikiAgentFixture, arguments map[string]any) map[string]any {
	merged := map[string]any{"project_id": f.idProject}
	for key, value := range arguments {
		merged[key] = value
	}
	return merged
}

func (f *wikiAgentFixture) suggest(t *testing.T, idRunHeader string, arguments map[string]any) model.WikiProposalAgentView {
	result := f.callTool(t, f.agentToken, idRunHeader, "suggest_wiki_change", withProject(f, arguments))
	require.False(t, result.IsError, result.Text)
	var view model.WikiProposalAgentView
	require.NoError(t, json.Unmarshal([]byte(result.Text), &view))
	return view
}

func (f *wikiAgentFixture) setRunPhase(t *testing.T, idRun int64, phase string) *model.AgentRun {
	finished := constants.TerminalPhases[phase]
	_, err := f.app.Pool.Exec(context.Background(), `
		UPDATE agent.run SET phase = $2, finished_at = CASE WHEN $3 THEN now() END WHERE id_run = $1
	`, idRun, phase, finished)
	require.NoError(t, err)
	run, err := injector.GetAgentRunRepository().LoadById(context.Background(), idRun)
	require.NoError(t, err)
	return run
}

func (f *wikiAgentFixture) issueProposals(t *testing.T, idIssue int64) []model.WikiProposal {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/issue/%d/wiki-proposal", idIssue), "", f.adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var proposals []model.WikiProposal
	require.NoError(t, json.Unmarshal([]byte(body), &proposals))
	return proposals
}

func (f *wikiAgentFixture) openProposals(t *testing.T) []model.WikiProposal {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/proposal", f.idProject), "", f.adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var proposals []model.WikiProposal
	require.NoError(t, json.Unmarshal([]byte(body), &proposals))
	return proposals
}

func (f *wikiAgentFixture) accept(t *testing.T, token string, idProposal int64, req model.WikiProposalAcceptReq) *http.Response {
	body, _ := json.Marshal(req)
	return Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/proposal/%d/accept", idProposal), string(body), token)
}

func (f *wikiAgentFixture) reject(t *testing.T, token string, idProposal int64, reason string) *http.Response {
	body, _ := json.Marshal(model.WikiProposalRejectReq{Reason: reason})
	return Request(t, f.app, "POST", fmt.Sprintf("/api/private/wiki/proposal/%d/reject", idProposal), string(body), token)
}

func (f *wikiAgentFixture) proposalDetail(t *testing.T, idProposal int64) service.WikiProposalDetail {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/proposal/%d", idProposal), "", f.adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var detail service.WikiProposalDetail
	require.NoError(t, json.Unmarshal([]byte(body), &detail))
	return detail
}

func (f *wikiAgentFixture) versions(t *testing.T, idPage int64) []model.WikiPageVersionSummary {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/wiki/page/%d/version", idPage), "", f.adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var versions []model.WikiPageVersionSummary
	require.NoError(t, json.Unmarshal([]byte(body), &versions))
	return versions
}

func (f *wikiAgentFixture) notificationBody(t *testing.T, notificationType string, idUser, idIssue int64) model.NotificationBodyWikiProposal {
	var raw []byte
	err := f.app.Pool.QueryRow(context.Background(), `
		SELECT body FROM notification.notification WHERE id_user = $1 AND type = $2 AND ref_id = $3
	`, idUser, notificationType, fmt.Sprint(idIssue)).Scan(&raw)
	require.NoError(t, err)
	var body model.NotificationBodyWikiProposal
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}

func (f *wikiAgentFixture) notificationActor(t *testing.T, notificationType string, idUser, idIssue int64) string {
	var actor string
	err := f.app.Pool.QueryRow(context.Background(), `
		SELECT coalesce(actor_name, '') FROM notification.notification WHERE id_user = $1 AND type = $2 AND ref_id = $3
	`, idUser, notificationType, fmt.Sprint(idIssue)).Scan(&actor)
	require.NoError(t, err)
	return actor
}

func (f *wikiAgentFixture) agentName(t *testing.T) string {
	var name string
	err := f.app.Pool.QueryRow(context.Background(), `SELECT name FROM users.user WHERE id_user = $1`, f.idAgent).Scan(&name)
	require.NoError(t, err)
	require.NotEmpty(t, name)
	return name
}

func (f *wikiAgentFixture) notificationCount(t *testing.T, notificationType string, idUser, idIssue int64) int {
	var count int
	err := f.app.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM notification.notification WHERE id_user = $1 AND type = $2 AND ref_id = $3
	`, idUser, notificationType, fmt.Sprint(idIssue)).Scan(&count)
	require.NoError(t, err)
	return count
}
