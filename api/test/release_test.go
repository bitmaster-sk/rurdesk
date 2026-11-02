package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/require"
)

type releaseFixture struct {
	*wikiAgentFixture
	final    model.State
	nonFinal model.State
	types    map[string]int64
	received chan []byte
}

func newReleaseFixture(t *testing.T, name string) *releaseFixture {
	f := &releaseFixture{wikiAgentFixture: newWikiAgentFixture(t, name), types: map[string]int64{}}
	f.final, f.nonFinal = finalAndNonFinalState(t, f.app, f.adminToken, f.idProject)
	res := Request(t, f.app, "GET", "/api/private/issue-type", "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var types []model.IssueType
	require.NoError(t, json.NewDecoder(res.Body).Decode(&types))
	for _, issueType := range types {
		if issueType.IdProject == f.idProject {
			f.types[issueType.Name] = issueType.IdIssueType
		}
	}
	return f
}

func (f *releaseFixture) withGateway(t *testing.T) {
	received := make(chan []byte, 16)
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

func (f *releaseFixture) issue(t *testing.T, title string, state model.State, typeName string, idsTag ...int64) model.Issue {
	req := model.CreateIssueReq{IdProject: f.idProject, Title: title, Description: "x", IdState: &state.IdState, IdsTag: idsTag}
	if typeName != "" {
		idType := f.types[typeName]
		req.IdIssueType = &idType
	}
	body, _ := json.Marshal(req)
	res := Request(t, f.app, "POST", "/api/private/project/"+itoa(f.idProject)+"/issue", string(body), f.adminToken)
	resBody := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, resBody)
	var created model.Issue
	require.NoError(t, json.Unmarshal([]byte(resBody), &created))
	return created
}

func (f *releaseFixture) preview(t *testing.T, idTag int64) model.ReleaseDraft {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/release/draft?idTag=%d", f.idProject, idTag), "", f.adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var draft model.ReleaseDraft
	require.NoError(t, json.Unmarshal([]byte(body), &draft))
	return draft
}

func (f *releaseFixture) postDraft(t *testing.T, token string, idTag int64, name string) *http.Response {
	body, _ := json.Marshal(model.CreateReleaseReq{IdTag: idTag, Name: name})
	return Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/release", f.idProject), string(body), token)
}

func (f *releaseFixture) createDraft(t *testing.T, idTag int64, name string) model.ReleaseView {
	return decodeRelease(t, f.postDraft(t, f.adminToken, idTag, name))
}

func (f *releaseFixture) putDraft(t *testing.T, token string, idRelease int64, req model.UpdateReleaseReq) *http.Response {
	body, _ := json.Marshal(req)
	return Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/release/%d", f.idProject, idRelease), string(body), token)
}

func (f *releaseFixture) saveDraft(t *testing.T, idRelease int64, req model.UpdateReleaseReq) model.ReleaseView {
	return decodeRelease(t, f.putDraft(t, f.adminToken, idRelease, req))
}

func (f *releaseFixture) putNotes(t *testing.T, token string, idRelease int64, text string) *http.Response {
	body, _ := json.Marshal(model.UpdateReleaseNotesReq{Notes: text})
	return Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/release/%d/notes", f.idProject, idRelease), string(body), token)
}

func (f *releaseFixture) postAction(t *testing.T, token string, idRelease int64, action, body string) *http.Response {
	return Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/release/%d/%s", f.idProject, idRelease, action), body, token)
}

func (f *releaseFixture) publish(t *testing.T, idRelease int64) model.ReleaseView {
	return decodeRelease(t, f.postAction(t, f.adminToken, idRelease, "publish", ""))
}

func (f *releaseFixture) startAgent(t *testing.T, idRelease int64) *http.Response {
	return f.postAction(t, f.adminToken, idRelease, "agent", fmt.Sprintf(`{"idUserAgent":%d}`, f.idAgent))
}

func (f *releaseFixture) withAgent(t *testing.T, idRelease int64) model.ReleaseView {
	return decodeRelease(t, f.startAgent(t, idRelease))
}

func (f *releaseFixture) ship(t *testing.T, idTag int64, req model.UpdateReleaseReq) model.ReleaseView {
	created := f.createDraft(t, idTag, req.Name)
	f.saveDraft(t, created.IdRelease, req)
	return f.publish(t, created.IdRelease)
}

func (f *releaseFixture) loadRelease(t *testing.T, token string, idRelease int64) *http.Response {
	return Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/release/%d", f.idProject, idRelease), "", token)
}

func (f *releaseFixture) releases(t *testing.T, token string) []model.ReleaseView {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/release", f.idProject), "", token)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var views []model.ReleaseView
	require.NoError(t, json.Unmarshal([]byte(body), &views))
	return views
}

func (f *releaseFixture) findRelease(t *testing.T, idRelease int64) model.ReleaseView {
	return decodeRelease(t, f.loadRelease(t, f.adminToken, idRelease))
}

func (f *releaseFixture) deleteRelease(t *testing.T, idRelease int64, req model.DeleteReleaseReq) *http.Response {
	body, _ := json.Marshal(req)
	return Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/project/%d/release/%d", f.idProject, idRelease), string(body), f.adminToken)
}

func decodeRelease(t *testing.T, res *http.Response) model.ReleaseView {
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var view model.ReleaseView
	require.NoError(t, json.Unmarshal([]byte(body), &view))
	return view
}

func requireErr(t *testing.T, res *http.Response, status int, code string) {
	body := readBody(t, res)
	require.Equal(t, status, res.StatusCode, body)
	require.Contains(t, body, code)
}

func notes(name, openIssues, body string, recreateTag bool) model.UpdateReleaseReq {
	return model.UpdateReleaseReq{Name: name, RecreateTag: recreateTag, OpenIssues: openIssues, Notes: &body}
}

func draftSettings(name, openIssues string, recreateTag bool) model.UpdateReleaseReq {
	return model.UpdateReleaseReq{Name: name, RecreateTag: recreateTag, OpenIssues: openIssues}
}

func (f *releaseFixture) tagByName(t *testing.T, name string) *model.Tag {
	for _, tag := range loadTags(t, f.app, f.adminToken, f.idProject) {
		if tag.Name == name {
			return &tag
		}
	}
	return nil
}

func (f *releaseFixture) issueTags(t *testing.T, issue model.Issue) []int64 {
	return loadIssueTags(t, f.app, f.adminToken, f.idProject, issue.IdIssuePublic)
}

func (f *releaseFixture) notesRun(t *testing.T, idRelease int64) *model.AgentRun {
	run, err := injector.GetAgentRunRepository().LoadActiveByRelease(context.Background(), idRelease)
	require.NoError(t, err)
	require.NotNil(t, run, "the release has an active notes run")
	return run
}

func (f *releaseFixture) issueCount(t *testing.T) int {
	var count int
	err := f.app.Pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM issues.issue WHERE id_project = $1", f.idProject).Scan(&count)
	require.NoError(t, err)
	return count
}

func (f *releaseFixture) activeTask(t *testing.T, run *model.AgentRun, stage string) *model.AgentTask {
	ctx := context.Background()
	task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, run.IdUserAgent, stage, 1)
	require.NoError(t, err)
	task, err = injector.GetAgentTaskRepository().TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive)
	require.NoError(t, err)
	return task
}

func (f *releaseFixture) dispatchPayload(t *testing.T, run *model.AgentRun, task *model.AgentTask) map[string]any {
	for len(f.received) > 0 {
		<-f.received
	}
	injector.GetDispatcher().DispatchStageExecute(context.Background(), run, task)
	select {
	case raw := <-f.received:
		var event struct {
			Payload map[string]any `json:"payload"`
		}
		require.NoError(t, json.Unmarshal(raw, &event))
		return event.Payload
	case <-time.After(10 * time.Second):
		t.Fatal("gateway never received the stage_execute event")
		return nil
	}
}

func (f *releaseFixture) completeStage(t *testing.T, idTask int64, req model.CompleteStageReq) model.CompleteStageRes {
	body, _ := json.Marshal(req)
	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/agent/task/%d/complete", idTask), string(body), f.agentToken)
	resBody := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, resBody)
	var out model.CompleteStageRes
	require.NoError(t, json.Unmarshal([]byte(resBody), &out))
	return out
}

func TestRelease_PreviewGroupsClosedTasksByTypeAndListsOpenOnes(t *testing.T) {
	f := newReleaseFixture(t, "release-preview")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	bug := f.issue(t, "Login loops", f.final, "Bug", next.IdTag)
	feature := f.issue(t, "Dark mode", f.final, "Feature", next.IdTag)
	untyped := f.issue(t, "Bump deps", f.final, "", next.IdTag)
	open := f.issue(t, "Half done", f.nonFinal, "Feature", next.IdTag)
	f.issue(t, "Not in the release", f.final, "Bug")

	draft := f.preview(t, next.IdTag)

	require.Len(t, draft.Sections, 3)
	require.Equal(t, "Bug", *draft.Sections[0].Name)
	require.Equal(t, bug.IdIssuePublic, draft.Sections[0].Issues[0].IdIssuePublic)
	require.Equal(t, "Feature", *draft.Sections[1].Name)
	require.Equal(t, feature.IdIssuePublic, draft.Sections[1].Issues[0].IdIssuePublic)
	require.Nil(t, draft.Sections[2].Name, "tasks without a type form the last section")
	require.Equal(t, untyped.IdIssuePublic, draft.Sections[2].Issues[0].IdIssuePublic)
	require.Len(t, draft.OpenIssues, 1)
	require.Equal(t, open.IdIssuePublic, draft.OpenIssues[0].IdIssuePublic)
}

func TestRelease_ADraftChangesNothingUntilItIsReleased(t *testing.T) {
	f := newReleaseFixture(t, "release-draft")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Shipped", f.final, "Feature", next.IdTag)

	created := f.createDraft(t, next.IdTag, " v1.0.4 ")

	require.Equal(t, constants.ReleaseStatusDraft, created.Status)
	require.Equal(t, "v1.0.4", created.Name)
	require.True(t, created.RecreateTag, "a draft creates the tag again by default")
	require.Equal(t, constants.ReleaseOpenIssuesMove, created.OpenIssues)
	require.Equal(t, "next", *created.TagName)
	require.Nil(t, created.ReleasedAt)

	saved := f.saveDraft(t, created.IdRelease, notes("v1.0.5", constants.ReleaseOpenIssuesKeep, "## Features\n\n- Shipped (#1)\n", false))
	require.Equal(t, "v1.0.5", saved.Name)
	require.Equal(t, constants.ReleaseOpenIssuesKeep, saved.OpenIssues)
	require.False(t, saved.RecreateTag)
	require.Equal(t, "## Features\n\n- Shipped (#1)\n", f.findRelease(t, created.IdRelease).Notes)
	require.NotNil(t, f.tagByName(t, "next"), "saving a draft leaves the tag alone")
	require.Nil(t, f.tagByName(t, "v1.0.5"))

	requireErr(t, f.postDraft(t, f.adminToken, next.IdTag, "v9"), http.StatusConflict, "RELEASE_DRAFT_EXISTS")
	requireErr(t, f.putDraft(t, f.adminToken, created.IdRelease, notes("x", "later", "", true)), http.StatusBadRequest, "")
}

func TestRelease_PublishingRenamesTheTagRecreatesItAndMovesOpenTasks(t *testing.T) {
	f := newReleaseFixture(t, "release-publish")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	closed := f.issue(t, "Shipped", f.final, "Feature", next.IdTag)
	open := f.issue(t, "Unfinished", f.nonFinal, "Feature", next.IdTag)
	created := f.createDraft(t, next.IdTag, "v1.0.4")
	f.saveDraft(t, created.IdRelease, notes("v1.0.4", constants.ReleaseOpenIssuesMove, "## Fixes\n\n- Shipped (#1)\n", true))

	view := f.publish(t, created.IdRelease)

	require.Equal(t, constants.ReleaseStatusReleased, view.Status)
	require.Equal(t, 1, view.IssueCount)
	require.NotNil(t, view.ReleasedAt)
	require.Equal(t, next.IdTag, *view.IdTag)
	version := f.tagByName(t, "v1.0.4")
	require.NotNil(t, version)
	require.Equal(t, next.IdTag, version.IdTag, "the tag is renamed, not replaced")
	recreated := f.tagByName(t, "next")
	require.NotNil(t, recreated)
	require.NotEqual(t, next.IdTag, recreated.IdTag)
	require.Equal(t, next.Color, recreated.Color)
	require.Equal(t, []int64{next.IdTag}, f.issueTags(t, closed))
	require.Equal(t, []int64{recreated.IdTag}, f.issueTags(t, open), "an open task moves to the new tag")

	require.Equal(t, "## Fixes\n\n- Shipped (#1)\n", view.Notes)
	wiki := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/wiki/tree", f.idProject), "", f.adminToken)
	require.NotContains(t, readBody(t, wiki), "v1.0.4", "the notes stay on the release, no wiki page is written")

	requireErr(t, f.postAction(t, f.adminToken, created.IdRelease, "publish", ""), http.StatusConflict, "RELEASE_NOT_DRAFT")
	requireErr(t, f.putDraft(t, f.adminToken, created.IdRelease, notes("v1.0.4", constants.ReleaseOpenIssuesMove, "other", true)), http.StatusConflict, "RELEASE_NOT_DRAFT")
	requireErr(t, f.startAgent(t, created.IdRelease), http.StatusConflict, "RELEASE_NOT_DRAFT")
}

func TestRelease_OpenTasksStayOnTheVersionOrLeaveTheTag(t *testing.T) {
	f := newReleaseFixture(t, "release-open")
	keepTag := createTag(t, f.app, f.adminToken, f.idProject, "keep-me")
	f.issue(t, "Done", f.final, "Bug", keepTag.IdTag)
	kept := f.issue(t, "Open kept", f.nonFinal, "Bug", keepTag.IdTag)
	f.ship(t, keepTag.IdTag, notes("v2.0", constants.ReleaseOpenIssuesKeep, "notes", false))
	require.Equal(t, []int64{keepTag.IdTag}, f.issueTags(t, kept))
	require.Nil(t, f.tagByName(t, "keep-me"), "without recreateTag no tag keeps the old name")

	moveTag := createTag(t, f.app, f.adminToken, f.idProject, "drop-me")
	f.issue(t, "Done too", f.final, "Bug", moveTag.IdTag)
	dropped := f.issue(t, "Open dropped", f.nonFinal, "Bug", moveTag.IdTag)
	f.ship(t, moveTag.IdTag, notes("v3.0", constants.ReleaseOpenIssuesMove, "notes", false))
	require.Empty(t, f.issueTags(t, dropped), "with nowhere to move the open task loses the tag")
}

func TestRelease_PublishingNeedsNotesButNoClosedTask(t *testing.T) {
	f := newReleaseFixture(t, "release-empty")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	open := f.issue(t, "Still open", f.nonFinal, "Bug", next.IdTag)
	created := f.createDraft(t, next.IdTag, "v0.1")

	requireErr(t, f.postAction(t, f.adminToken, created.IdRelease, "publish", ""), http.StatusBadRequest, "VALIDATION_FAILED")
	require.Equal(t, constants.ReleaseStatusDraft, f.findRelease(t, created.IdRelease).Status)

	f.saveDraft(t, created.IdRelease, notes("v0.1", constants.ReleaseOpenIssuesMove, "notes", true))
	published := f.publish(t, created.IdRelease)
	require.Equal(t, constants.ReleaseStatusReleased, published.Status)
	require.Zero(t, published.IssueCount)
	recreated := f.tagByName(t, "next")
	require.NotNil(t, recreated)
	require.Equal(t, []int64{recreated.IdTag}, f.issueTags(t, open), "the open task moves to the new tag")
}

func TestRelease_ANameTakenByATagOrAnotherReleaseIsAConflict(t *testing.T) {
	f := newReleaseFixture(t, "release-name")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Done", f.final, "Bug", next.IdTag)
	createTag(t, f.app, f.adminToken, f.idProject, "v1.0")

	requireErr(t, f.postDraft(t, f.adminToken, next.IdTag, "V1.0"), http.StatusConflict, "RELEASE_NAME_TAKEN")

	first := f.ship(t, next.IdTag, notes("v1.1", constants.ReleaseOpenIssuesKeep, "notes", false))
	require.Equal(t, http.StatusOK, editTag(t, f.app, f.adminToken, *f.tagByName(t, "v1.1"), "old-1.1").StatusCode)

	other := createTag(t, f.app, f.adminToken, f.idProject, "later")
	f.issue(t, "Done later", f.final, "Bug", other.IdTag)
	requireErr(t, f.postDraft(t, f.adminToken, other.IdTag, "v1.1"), http.StatusConflict, "RELEASE_NAME_TAKEN")
	draft := f.createDraft(t, other.IdTag, "v1.2")
	requireErr(t, f.putDraft(t, f.adminToken, draft.IdRelease, notes("v1.1", constants.ReleaseOpenIssuesKeep, "", true)), http.StatusConflict, "RELEASE_NAME_TAKEN")

	f.saveDraft(t, draft.IdRelease, notes("v1.2", constants.ReleaseOpenIssuesKeep, "notes", false))
	createTag(t, f.app, f.adminToken, f.idProject, "v1.2")
	requireErr(t, f.postAction(t, f.adminToken, draft.IdRelease, "publish", ""), http.StatusConflict, "RELEASE_NAME_TAKEN")
	require.NotNil(t, f.tagByName(t, "later"), "the release is rolled back whole")
	require.Equal(t, constants.ReleaseStatusDraft, f.findRelease(t, draft.IdRelease).Status)

	f.saveDraft(t, draft.IdRelease, notes("later", constants.ReleaseOpenIssuesKeep, "notes", true))
	requireErr(t, f.postAction(t, f.adminToken, draft.IdRelease, "publish", ""), http.StatusConflict, "RELEASE_NAME_IS_TAG")

	views := f.releases(t, f.adminToken)
	require.Len(t, views, 2)
	require.Equal(t, draft.IdRelease, views[0].IdRelease, "drafts come first")
	require.Equal(t, first.IdRelease, views[1].IdRelease)
}

func TestRelease_TheNotesOfAReleaseCanBeEditedAfterwards(t *testing.T) {
	f := newReleaseFixture(t, "release-notes-edit")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Done", f.final, "Bug", next.IdTag)
	view := f.ship(t, next.IdTag, notes("v1.0", constants.ReleaseOpenIssuesKeep, "first notes", false))
	member := f.userWithRole(t, "release-notes-edit-member@test.sk", "member")

	edited := decodeRelease(t, f.putNotes(t, f.adminToken, view.IdRelease, "## Fixed\n\n- Done (#1)\n"))

	require.Equal(t, "## Fixed\n\n- Done (#1)\n", edited.Notes)
	require.Equal(t, constants.ReleaseStatusReleased, edited.Status)
	require.Equal(t, "## Fixed\n\n- Done (#1)\n", f.findRelease(t, view.IdRelease).Notes)
	requireErr(t, f.putNotes(t, f.adminToken, view.IdRelease, "  "), http.StatusBadRequest, "VALIDATION_FAILED")
	require.Equal(t, http.StatusForbidden, f.putNotes(t, member, view.IdRelease, "mine").StatusCode)

	other := createTag(t, f.app, f.adminToken, f.idProject, "later")
	draft := f.createDraft(t, other.IdTag, "v2.0")
	requireErr(t, f.putNotes(t, f.adminToken, draft.IdRelease, "early"), http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestRelease_OnlyTheOwnerWorksOnReleasesButMembersSeeThem(t *testing.T) {
	f := newReleaseFixture(t, "release-acl")
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Done", f.final, "Bug", next.IdTag)
	member := f.userWithRole(t, "release-acl-member@test.sk", "member")
	created := f.createDraft(t, next.IdTag, "v1.0")

	require.Len(t, f.releases(t, member), 1)
	require.Equal(t, http.StatusOK, f.loadRelease(t, member, created.IdRelease).StatusCode)
	draftRes := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/release/draft?idTag=%d", f.idProject, next.IdTag), "", member)
	require.Equal(t, http.StatusOK, draftRes.StatusCode, "members see the tasks of a draft")
	other := createTag(t, f.app, f.adminToken, f.idProject, "other")
	require.Equal(t, http.StatusForbidden, f.postDraft(t, member, other.IdTag, "v2.0").StatusCode)
	require.Equal(t, http.StatusForbidden, f.putDraft(t, member, created.IdRelease, notes("v1.0", constants.ReleaseOpenIssuesKeep, "x", true)).StatusCode)
	require.Equal(t, http.StatusForbidden, f.postAction(t, member, created.IdRelease, "publish", "").StatusCode)
	require.Equal(t, http.StatusForbidden, f.postAction(t, member, created.IdRelease, "agent", fmt.Sprintf(`{"idUserAgent":%d}`, f.idAgent)).StatusCode)
	require.Equal(t, http.StatusNotFound, f.loadRelease(t, f.adminToken, created.IdRelease+1000).StatusCode)
}

func TestRelease_AnAgentWritesTheNotesIntoTheDraft(t *testing.T) {
	f := newReleaseFixture(t, "release-agent")
	f.withGateway(t)
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Faster search", f.final, "Feature", next.IdTag)
	f.issue(t, "Half done", f.nonFinal, "Bug", next.IdTag)
	f.issue(t, "Not in the release", f.final, "Bug")
	created := f.createDraft(t, next.IdTag, "v2.0")

	view := f.withAgent(t, created.IdRelease)

	require.True(t, view.IsNotesPending)
	run := f.notesRun(t, created.IdRelease)
	require.Equal(t, run.IdRun, *view.IdRunNotes)
	require.Equal(t, constants.RunKindReleaseNotes, run.Kind)
	require.Nil(t, run.IdIssue, "the run belongs to the release, not to a task")
	require.Equal(t, 3, f.issueCount(t), "writing the notes creates no task")
	var plan model.AgentStagePlan
	require.NoError(t, json.Unmarshal(run.StagePlan, &plan))
	require.Equal(t, []string{constants.StagePickup, constants.StageReleaseNotes}, []string{plan.Stages[0].Name, plan.Stages[1].Name})

	requireErr(t, f.startAgent(t, created.IdRelease), http.StatusConflict, "RELEASE_NOTES_PENDING")
	requireErr(t, f.putDraft(t, f.adminToken, created.IdRelease, notes("v2.0", constants.ReleaseOpenIssuesMove, "by hand", true)), http.StatusConflict, "RELEASE_NOTES_PENDING")
	requireErr(t, f.postAction(t, f.adminToken, created.IdRelease, "publish", ""), http.StatusConflict, "RELEASE_NOTES_PENDING")
	renamed := f.saveDraft(t, created.IdRelease, notes("v2.1", constants.ReleaseOpenIssuesKeep, "", true))
	require.Equal(t, "v2.1", renamed.Name, "other draft fields can change while the agent writes")

	task := f.activeTask(t, run, constants.StageReleaseNotes)
	overviewRes := Request(t, f.app, "GET", fmt.Sprintf("/api/private/project/%d/agents/overview", f.idProject), "", f.adminToken)
	require.Equal(t, http.StatusOK, overviewRes.StatusCode)
	var overviews []model.AgentOverview
	require.NoError(t, json.NewDecoder(overviewRes.Body).Decode(&overviews))
	require.Len(t, overviews, 1)
	require.Equal(t, "v2.1", *overviews[0].Current.ReleaseName, "the agent dock names the release it writes")
	require.Nil(t, overviews[0].Current.IdIssuePublic)
	payload := f.dispatchPayload(t, run, task)
	require.Nil(t, payload["repo"], "a release notes run needs no repository")
	require.NotContains(t, payload["contextBundle"], "issue")
	settings, err := injector.GetAppSettingsService()
	require.NoError(t, err)
	require.EqualValues(t, settings.AgentMaxTurnsPlan(), payload["maxTurns"], "release notes use the plan turn limit")
	release := payload["contextBundle"].(map[string]any)["release"].(map[string]any)
	require.Equal(t, "v2.1", release["name"])
	require.Equal(t, "next", release["tag"])
	sections := release["sections"].([]any)
	require.Len(t, sections, 1, "only the tasks with the tag go into the prompt")
	require.Equal(t, "Feature", sections[0].(map[string]any)["name"])
	openIssues := release["openIssues"].([]any)
	require.Len(t, openIssues, 1, "open tasks are released too when the draft keeps them")
	require.Equal(t, "Half done", openIssues[0].(map[string]any)["title"])

	written := "## Features\n\n- Faster search (#1)\n"
	done := f.completeStage(t, task.IdTask, model.CompleteStageReq{Outcome: constants.StageOutcomeOutputSubmitted, Message: written})
	require.Equal(t, constants.PhaseDone, done.NextPhase)
	filled := f.findRelease(t, created.IdRelease)
	require.Equal(t, written, filled.Notes, "the agent's message becomes the draft notes")
	kept := f.saveDraft(t, created.IdRelease, draftSettings("v2.2", constants.ReleaseOpenIssuesKeep, true))
	require.Equal(t, written, kept.Notes, "a save without notes keeps the notes")
	require.False(t, filled.IsNotesPending)
	require.Equal(t, constants.PhaseDone, *filled.NotesRunPhase)
	var messages int
	require.NoError(t, f.app.Pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM messages.message WHERE message = $1", written).Scan(&messages))
	require.Zero(t, messages, "the notes are no message anywhere")

	again := f.withAgent(t, created.IdRelease)
	rerun := f.notesRun(t, created.IdRelease)
	require.NotEqual(t, run.IdRun, rerun.IdRun)
	require.Equal(t, rerun.IdRun, *again.IdRunNotes, "writing again starts a new run")
	f.setRunPhase(t, rerun.IdRun, constants.PhaseCancelled)

	published := f.publish(t, created.IdRelease)
	require.Equal(t, written, published.Notes)
}

func TestRelease_ARestartedNotesRunStaysOnTheRelease(t *testing.T) {
	f := newReleaseFixture(t, "release-restart")
	f.withGateway(t)
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Done", f.final, "Bug", next.IdTag)
	created := f.createDraft(t, next.IdTag, "v4.0")
	f.withAgent(t, created.IdRelease)
	run := f.notesRun(t, created.IdRelease)
	f.setRunPhase(t, run.IdRun, constants.PhaseFailed)
	failed := f.findRelease(t, created.IdRelease)
	require.False(t, failed.IsNotesPending)
	require.Equal(t, constants.PhaseFailed, *failed.NotesRunPhase)

	res := Request(t, f.app, "POST", fmt.Sprintf("/api/private/agent/run/%d/restart", run.IdRun), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	restarted := f.notesRun(t, created.IdRelease)
	require.NotEqual(t, run.IdRun, restarted.IdRun)
	require.Equal(t, constants.RunKindReleaseNotes, restarted.Kind)
	require.Nil(t, restarted.IdIssue)

	snapshot := Request(t, f.app, "GET", fmt.Sprintf("/api/private/agent/run/%d", restarted.IdRun), "", f.adminToken)
	body := readBody(t, snapshot)
	require.Equal(t, http.StatusOK, snapshot.StatusCode, body)
	require.Contains(t, body, `"isParked":false`, "a run without a task is never parked")
}

func TestRelease_DeletingADraftLeavesTheTagAndStopsTheAgent(t *testing.T) {
	f := newReleaseFixture(t, "release-delete-draft")
	f.withGateway(t)
	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	done := f.issue(t, "Done", f.final, "Bug", next.IdTag)
	view := f.withAgent(t, f.createDraft(t, next.IdTag, "v1").IdRelease)
	run := f.notesRun(t, view.IdRelease)
	f.activeTask(t, run, constants.StageReleaseNotes)

	res := f.deleteRelease(t, view.IdRelease, model.DeleteReleaseReq{})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	require.Eventually(t, func() bool {
		select {
		case raw := <-f.received:
			var event struct {
				IdRun int64  `json:"idRun"`
				Event string `json:"event"`
			}
			return json.Unmarshal(raw, &event) == nil && event.IdRun == run.IdRun && event.Event == "cancelled"
		default:
			return false
		}
	}, 10*time.Second, 50*time.Millisecond, "the gateway hears that the run was cancelled")
	_, err := injector.GetAgentRunRepository().LoadById(context.Background(), run.IdRun)
	require.ErrorIs(t, err, repository.ErrRunNotFound, "the run goes with its release")
	require.Equal(t, []int64{next.IdTag}, f.issueTags(t, done))
	require.NotNil(t, f.tagByName(t, "next"))
	require.Empty(t, f.releases(t, f.adminToken))
	f.createDraft(t, next.IdTag, "v1")
}

func TestRelease_DeleteLetsTheUserDecideAboutTheVersionTag(t *testing.T) {
	f := newReleaseFixture(t, "release-delete")
	target := createTag(t, f.app, f.adminToken, f.idProject, "backlog")

	keepTag := createTag(t, f.app, f.adminToken, f.idProject, "a")
	keptIssue := f.issue(t, "Kept", f.final, "Bug", keepTag.IdTag)
	kept := f.ship(t, keepTag.IdTag, notes("v1", constants.ReleaseOpenIssuesKeep, "notes", false))
	require.Equal(t, http.StatusOK, f.deleteRelease(t, kept.IdRelease, model.DeleteReleaseReq{Tag: "keep"}).StatusCode)
	require.Equal(t, []int64{keepTag.IdTag}, f.issueTags(t, keptIssue))
	require.NotNil(t, f.tagByName(t, "v1"))

	moveTag := createTag(t, f.app, f.adminToken, f.idProject, "b")
	moved := f.issue(t, "Moved", f.final, "Bug", moveTag.IdTag)
	both := f.issue(t, "Already on target", f.final, "Bug", moveTag.IdTag, target.IdTag)
	deleted := f.ship(t, moveTag.IdTag, notes("v2", constants.ReleaseOpenIssuesKeep, "notes", false))
	require.Equal(t, http.StatusOK, f.deleteRelease(t, deleted.IdRelease, model.DeleteReleaseReq{Tag: "move_delete", IdTagTarget: &target.IdTag}).StatusCode)
	require.Equal(t, []int64{target.IdTag}, f.issueTags(t, moved))
	require.Equal(t, []int64{target.IdTag}, f.issueTags(t, both), "a task already on the target has it once")
	require.Nil(t, f.tagByName(t, "v2"))

	emptyTag := createTag(t, f.app, f.adminToken, f.idProject, "c")
	emptied := f.issue(t, "Emptied", f.final, "Bug", emptyTag.IdTag)
	emptyRelease := f.ship(t, emptyTag.IdTag, notes("v3", constants.ReleaseOpenIssuesKeep, "notes", false))
	require.Equal(t, http.StatusBadRequest, f.deleteRelease(t, emptyRelease.IdRelease, model.DeleteReleaseReq{Tag: "move_keep", IdTagTarget: &emptyTag.IdTag}).StatusCode)
	require.Equal(t, http.StatusBadRequest, f.deleteRelease(t, emptyRelease.IdRelease, model.DeleteReleaseReq{Tag: "move_keep"}).StatusCode)
	foreign := createTag(t, f.app, f.adminToken, createProject(t, f.app, f.adminToken, "release-delete-other"), "foreign")
	require.Equal(t, http.StatusUnprocessableEntity, f.deleteRelease(t, emptyRelease.IdRelease, model.DeleteReleaseReq{Tag: "move_keep", IdTagTarget: &foreign.IdTag}).StatusCode)
	require.Equal(t, http.StatusOK, f.deleteRelease(t, emptyRelease.IdRelease, model.DeleteReleaseReq{Tag: "move_keep", IdTagTarget: &target.IdTag}).StatusCode)
	require.Equal(t, []int64{target.IdTag}, f.issueTags(t, emptied))
	left := f.tagByName(t, "v3")
	require.NotNil(t, left)
	require.Zero(t, left.IssueCount)
	require.Empty(t, f.releases(t, f.adminToken))
}

func TestRelease_ProjectSkillsReachTheReleaseNotesStage(t *testing.T) {
	Setup(t)
	require.NoError(t, injector.GetSkillService().SyncBuiltins(context.Background()))
	f := newReleaseFixture(t, "release-skills")
	f.withGateway(t)
	defaults := injector.GetProjectSkillService()
	byStage, err := defaults.LoadDefaultIdsSkillByStage(context.Background(), f.idProject)
	require.NoError(t, err)
	require.Len(t, byStage[constants.StageReleaseNotes], 1, "a new project gets the release notes style skill")
	style := byStage[constants.StageReleaseNotes][0]

	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	f.issue(t, "Done", f.final, "Bug", next.IdTag)
	view := f.withAgent(t, f.createDraft(t, next.IdTag, "v1").IdRelease)
	run := f.notesRun(t, view.IdRelease)
	require.Equal(t, []int64{style}, injector.GetAgentStagePlanService().IdsSkillForStage(run.StagePlan, constants.StageReleaseNotes))

	payload := f.dispatchPayload(t, run, f.activeTask(t, run, constants.StageReleaseNotes))
	skills := payload["contextBundle"].(map[string]any)["skills"].([]any)
	require.Len(t, skills, 1)
	require.Equal(t, "Release notes style", skills[0].(map[string]any)["name"])

	unknown := Request(t, f.app, "PUT", fmt.Sprintf("/api/private/project/%d/skills", f.idProject), fmt.Sprintf(`[{"idSkill":%d,"stage":"nope"}]`, style), f.adminToken)
	require.Equal(t, http.StatusBadRequest, unknown.StatusCode)
}

func TestRelease_RunKindsListTheReleaseNotesStage(t *testing.T) {
	app := Setup(t)
	res := Request(t, app, "GET", "/api/private/agent/run-kinds", "", Token(t, app))
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var kinds []struct {
		Name   string `json:"name"`
		Stages []struct {
			Name      string `json:"name"`
			HasSkills bool   `json:"hasSkills"`
		} `json:"stages"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &kinds))
	require.Len(t, kinds, 2)
	require.Equal(t, constants.RunKindReleaseNotes, kinds[1].Name)
	require.Equal(t, constants.StagePickup, kinds[1].Stages[0].Name)
	require.False(t, kinds[1].Stages[0].HasSkills)
	require.Equal(t, constants.StageReleaseNotes, kinds[1].Stages[1].Name)
	require.True(t, kinds[1].Stages[1].HasSkills)
}
