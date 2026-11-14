package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type historyFixture struct {
	*hierarchyFixture
}

func newHistoryFixture(t *testing.T, name string) *historyFixture {
	return &historyFixture{hierarchyFixture: newHierarchyFixture(t, name)}
}

func (f *historyFixture) history(issue model.Issue) []model.ReadIssueHistoryDto {
	return loadHistory(f.t, f.app, f.adminToken, f.idProject, issue.IdIssuePublic).History
}

func loadHistory(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64) model.ReadIssueHistoryRes {
	res := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/issue/%d/history", idProject, idIssuePublic), "", token)
	requireStatus(t, res, http.StatusOK)
	var history model.ReadIssueHistoryRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&history))
	return history
}

func (f *historyFixture) adminId() int64 {
	return idOfUser(f.t, f.app, f.adminToken, "test@test.sk")
}

func historyOf(rows []model.ReadIssueHistoryDto, field, source string) []model.ReadIssueHistoryDto {
	var matched []model.ReadIssueHistoryDto
	for _, row := range rows {
		if row.Field == field && row.Source == source {
			matched = append(matched, row)
		}
	}
	return matched
}

func requireJSON(t *testing.T, expected string, actual json.RawMessage) {
	t.Helper()
	require.JSONEq(t, expected, string(actual))
}

func TestIssueHistory_PatchRecordsOneChangeWithTheEditor(t *testing.T) {
	f := newHistoryFixture(t, "history-patch")
	issue := f.createIssue("Old title", nil, 0)
	idState := createStateInProject(t, f.app, f.adminToken, f.idProject, "Review")

	res := f.edit(issue, fmt.Sprintf(`{"title":"New title","idState":%d}`, idState))
	requireStatus(t, res, http.StatusOK)

	rows := f.history(issue)
	require.Len(t, rows, 2)
	require.Equal(t, rows[0].IdChange, rows[1].IdChange)
	for _, row := range rows {
		require.Equal(t, model.IssueHistorySourceUser, row.Source)
		require.NotNil(t, row.IdUser)
		require.Equal(t, f.adminId(), *row.IdUser)
	}
	title := historyOf(rows, model.IssueHistoryFieldTitle, model.IssueHistorySourceUser)
	require.Len(t, title, 1)
	requireJSON(t, `"Old title"`, title[0].OldValue)
	requireJSON(t, `"New title"`, title[0].NewValue)
	state := historyOf(rows, model.IssueHistoryFieldState, model.IssueHistorySourceUser)
	require.Len(t, state, 1)
	requireJSON(t, fmt.Sprintf("%d", idState), state[0].NewValue)
}

func TestIssueHistory_PatchWithoutRealChangeRecordsNothing(t *testing.T) {
	f := newHistoryFixture(t, "history-noop")
	issue := f.createIssue("Same", nil, 0)

	res := f.edit(issue, `{"title":"Same","description":"d"}`)
	requireStatus(t, res, http.StatusOK)

	require.Empty(t, f.history(issue))
}

func TestIssueHistory_EmptyHistoryIsAnArray(t *testing.T) {
	f := newHistoryFixture(t, "history-empty")
	issue := f.createIssue("Untouched", nil, 0)

	res := Request(t, f.app, "GET", f.issueURL(issue.IdIssuePublic)+"/history", "", f.adminToken)
	requireStatus(t, res, http.StatusOK)
	require.JSONEq(t, `{"history":[],"users":[]}`, readBody(t, res))
}

func TestIssueHistory_TagsAreStoredSorted(t *testing.T) {
	f := newHistoryFixture(t, "history-tags")
	issue := f.createIssue("Tagged", nil, 0)
	first := createTag(t, f.app, f.adminToken, f.idProject, "backend")
	second := createTag(t, f.app, f.adminToken, f.idProject, "frontend")

	res := f.edit(issue, fmt.Sprintf(`{"idsTag":[%d,%d]}`, second.IdTag, first.IdTag))
	requireStatus(t, res, http.StatusOK)

	rows := f.history(issue)
	require.Len(t, rows, 1)
	require.Equal(t, model.IssueHistoryFieldTags, rows[0].Field)
	requireJSON(t, `[]`, rows[0].OldValue)
	requireJSON(t, fmt.Sprintf("[%d,%d]", first.IdTag, second.IdTag), rows[0].NewValue)
}

func TestIssueHistory_BulkEditGivesEachIssueItsOwnChange(t *testing.T) {
	f := newHistoryFixture(t, "history-bulk")
	first := f.createIssue("First", nil, 3600)
	second := f.createIssue("Second", nil, 3600)

	body := fmt.Sprintf(`{"issues":[{"idIssuePublic":%d,"estimated":7200},{"idIssuePublic":%d,"estimated":10800}]}`,
		first.IdIssuePublic, second.IdIssuePublic)
	res := Request(t, f.app, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/batch", f.idProject), body, f.adminToken)
	requireStatus(t, res, http.StatusOK)

	firstRows := f.history(first)
	secondRows := f.history(second)
	require.Len(t, firstRows, 1)
	require.Len(t, secondRows, 1)
	require.NotEqual(t, firstRows[0].IdChange, secondRows[0].IdChange)
	requireJSON(t, `3600`, firstRows[0].OldValue)
	requireJSON(t, `7200`, firstRows[0].NewValue)
	requireJSON(t, `10800`, secondRows[0].NewValue)
}

func TestIssueHistory_AssignAgentRecordsTheAssignee(t *testing.T) {
	f := newHistoryFixture(t, "history-assign-agent")
	res := Request(t, f.app, "POST", "/api/private/admin/user", `{"name":"historybot","isAgent":true}`, f.adminToken)
	requireStatus(t, res, http.StatusOK)
	var agentUser struct {
		IdUser int64 `json:"idUser"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&agentUser))
	res = Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/member/user", f.idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, agentUser.IdUser), f.adminToken)
	requireStatus(t, res, http.StatusOK)
	res = Request(t, f.app, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", agentUser.IdUser),
		`{"gatewayUrl":"http://gw:9090"}`, f.adminToken)
	requireStatus(t, res, http.StatusOK)
	issue := f.createIssue("For the agent", nil, 0)

	res = Request(t, f.app, "POST", f.issueURL(issue.IdIssuePublic)+"/assign-agent",
		fmt.Sprintf(`{"idUserAgent":%d}`, agentUser.IdUser), f.adminToken)
	requireStatus(t, res, http.StatusOK)

	assignee := historyOf(f.history(issue), model.IssueHistoryFieldAssignee, model.IssueHistorySourceUser)
	require.Len(t, assignee, 1)
	requireJSON(t, `null`, assignee[0].OldValue)
	requireJSON(t, fmt.Sprintf("%d", agentUser.IdUser), assignee[0].NewValue)
	require.Equal(t, f.adminId(), *assignee[0].IdUser)
}

func TestIssueHistory_UsersOfRemovedMembersComeWithTheHistory(t *testing.T) {
	f := newHistoryFixture(t, "history-users")
	memberToken := f.userWithRole(t, "history-member@test.sk", "member")
	idMember := idOfUser(t, f.app, f.adminToken, "history-member@test.sk")
	issue := f.createIssue("Shared", nil, 0)
	res := Request(t, f.app, "PATCH", f.issueURL(issue.IdIssuePublic), fmt.Sprintf(`{"assignedTo":%d}`, idMember), memberToken)
	requireStatus(t, res, http.StatusOK)
	res = Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/project/%d/member/user/%d", f.idProject, idMember), "", f.adminToken)
	requireStatus(t, res, http.StatusOK)

	history := loadHistory(t, f.app, f.adminToken, f.idProject, issue.IdIssuePublic)

	assignee := historyOf(history.History, model.IssueHistoryFieldAssignee, model.IssueHistorySourceUser)
	require.Len(t, assignee, 1)
	require.Equal(t, idMember, *assignee[0].IdUser)
	require.Len(t, history.Users, 1, "the editor and the assignee are the same user, listed once")
	require.Equal(t, idMember, history.Users[0].IdUser)
	require.NotEmpty(t, history.Users[0].Name)
	require.NotEmpty(t, history.Users[0].ColorAvatarBg)
}

func TestIssueHistory_ReleaseTagMovesAreRecorded(t *testing.T) {
	f := newReleaseFixture(t, "history-release")
	idAdmin := idOfUser(t, f.app, f.adminToken, "test@test.sk")
	tagsHistory := func(issue model.Issue) []model.ReadIssueHistoryDto {
		history := loadHistory(t, f.app, f.adminToken, f.idProject, issue.IdIssuePublic).History
		return historyOf(history, model.IssueHistoryFieldTags, model.IssueHistorySourceUser)
	}

	next := createTag(t, f.app, f.adminToken, f.idProject, "next")
	closed := f.issue(t, "Shipped", f.final, "Feature", next.IdTag)
	open := f.issue(t, "Unfinished", f.nonFinal, "Feature", next.IdTag)
	f.ship(t, next.IdTag, notes("v1.0", constants.ReleaseOpenIssuesMove, "notes", true))
	recreated := f.tagByName(t, "next")
	require.NotNil(t, recreated)

	require.Empty(t, tagsHistory(closed), "a closed task keeps its tag")
	moved := tagsHistory(open)
	require.Len(t, moved, 1)
	require.Equal(t, idAdmin, *moved[0].IdUser)
	requireJSON(t, fmt.Sprintf("[%d]", next.IdTag), moved[0].OldValue)
	requireJSON(t, fmt.Sprintf("[%d]", recreated.IdTag), moved[0].NewValue)

	dropTag := createTag(t, f.app, f.adminToken, f.idProject, "drop")
	dropped := f.issue(t, "Dropped", f.nonFinal, "Feature", dropTag.IdTag)
	f.ship(t, dropTag.IdTag, notes("v2.0", constants.ReleaseOpenIssuesMove, "notes", false))
	lost := tagsHistory(dropped)
	require.Len(t, lost, 1)
	requireJSON(t, fmt.Sprintf("[%d]", dropTag.IdTag), lost[0].OldValue)
	requireJSON(t, `[]`, lost[0].NewValue)

	target := createTag(t, f.app, f.adminToken, f.idProject, "backlog")
	versionTag := createTag(t, f.app, f.adminToken, f.idProject, "version")
	movedOnDelete := f.issue(t, "Moved", f.final, "Bug", versionTag.IdTag)
	already := f.issue(t, "Already on target", f.final, "Bug", versionTag.IdTag, target.IdTag)
	released := f.ship(t, versionTag.IdTag, notes("v3.0", constants.ReleaseOpenIssuesKeep, "notes", false))
	res := f.deleteRelease(t, released.IdRelease, model.DeleteReleaseReq{Tag: "move_keep", IdTagTarget: &target.IdTag})
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	settled := tagsHistory(movedOnDelete)
	require.Len(t, settled, 1)
	requireJSON(t, fmt.Sprintf("[%d]", versionTag.IdTag), settled[0].OldValue)
	requireJSON(t, fmt.Sprintf("[%d]", target.IdTag), settled[0].NewValue)
	alreadyMoved := tagsHistory(already)
	require.Len(t, alreadyMoved, 1)
	requireJSON(t, fmt.Sprintf("[%d]", target.IdTag), alreadyMoved[0].NewValue)
}

func TestIssueHistory_McpUpdateIsRecordedAsTheCaller(t *testing.T) {
	f := newHistoryFixture(t, "history-mcp")
	issue := f.createIssue("Via MCP", nil, 0)

	tools := &wikiAgentFixture{wikiFixture: f.wikiFixture, mcpSessions: map[string]string{}}
	result := tools.callTool(t, f.adminToken, "", "update_issue", map[string]any{
		"project_id": f.idProject, "issue_id": issue.IdIssuePublic, "title": "Renamed via MCP",
	})
	require.False(t, result.IsError, result.Text)

	title := historyOf(f.history(issue), model.IssueHistoryFieldTitle, model.IssueHistorySourceUser)
	require.Len(t, title, 1)
	require.Equal(t, f.adminId(), *title[0].IdUser)
	requireJSON(t, `"Renamed via MCP"`, title[0].NewValue)
}

func TestIssueHistory_BecomingSummaryKeepsTheOldEstimate(t *testing.T) {
	f := newHistoryFixture(t, "history-summary")
	parent := f.createIssue("Vývoj", ptr(day(20)), 40*3600)
	child := f.createIssue("Endpointy", ptr(day(1)), 2*24*3600)

	f.link(parent, child)

	rows := f.history(parent)
	require.Empty(t, historyOf(rows, model.IssueHistoryFieldEstimated, model.IssueHistorySourceUser))
	summary := historyOf(rows, model.IssueHistoryFieldIsSummary, model.IssueHistorySourceSystem)
	require.Len(t, summary, 1)
	requireJSON(t, `false`, summary[0].OldValue)
	requireJSON(t, `true`, summary[0].NewValue)
	require.Nil(t, summary[0].IdUser)
	estimated := historyOf(rows, model.IssueHistoryFieldEstimated, model.IssueHistorySourceSystem)
	require.Len(t, estimated, 1)
	requireJSON(t, fmt.Sprintf("%d", 40*3600), estimated[0].OldValue)
	requireJSON(t, fmt.Sprintf("%d", 2*24*3600), estimated[0].NewValue)
	require.Equal(t, summary[0].IdChange, estimated[0].IdChange)
}

func TestIssueHistory_RepeatedTracksMergeIntoOneSystemRow(t *testing.T) {
	f := newHistoryFixture(t, "history-merge")
	parent := f.createIssue("Vývoj", nil, 0)
	child := f.createIssue("Frontend", ptr(day(1)), 4*3600)
	f.link(parent, child)
	_, err := f.app.Pool.Exec(t.Context(),
		`UPDATE issues.issue_history SET create_at = create_at - interval '1 hour' WHERE id_issue = $1`, parent.IdIssue)
	require.NoError(t, err)

	track := func(seconds int) {
		res := Request(t, f.app, "POST", "/api/private/track", fmt.Sprintf(`{"idIssue":%d,"tracked":%d}`, child.IdIssue, seconds), f.adminToken)
		requireStatus(t, res, http.StatusOK)
	}
	track(600)
	track(1200)

	tracked := historyOf(f.history(parent), model.IssueHistoryFieldRollupTracked, model.IssueHistorySourceSystem)
	require.Len(t, tracked, 2, "the row from linking plus one merged row for both tracks")
	merged := tracked[1]
	requireJSON(t, `0`, merged.OldValue)
	requireJSON(t, `1800`, merged.NewValue)
}

func TestIssueHistory_MergedRowThatReturnsToItsOldValueIsDeleted(t *testing.T) {
	f := newHistoryFixture(t, "history-merge-noop")
	parent := f.createIssue("Vývoj", nil, 0)
	child := f.createIssue("Frontend", ptr(day(1)), 4*3600)
	f.link(parent, child)
	_, err := f.app.Pool.Exec(t.Context(),
		`UPDATE issues.issue_history SET create_at = create_at - interval '1 hour' WHERE id_issue = $1`, parent.IdIssue)
	require.NoError(t, err)

	res := f.edit(child, `{"estimated":7200}`)
	requireStatus(t, res, http.StatusOK)
	res = f.edit(child, fmt.Sprintf(`{"estimated":%d}`, 4*3600))
	requireStatus(t, res, http.StatusOK)

	rollupEstimated := historyOf(f.history(parent), model.IssueHistoryFieldRollupEstimated, model.IssueHistorySourceSystem)
	require.Len(t, rollupEstimated, 1, "only the backdated row from linking may remain")
	require.Greater(t, time.Since(rollupEstimated[0].CreateAt), 30*time.Minute)
}

func TestIssueHistory_DeletingTheIssueDeletesItsHistory(t *testing.T) {
	f := newHistoryFixture(t, "history-delete")
	issue := f.createIssue("Doomed", nil, 0)
	requireStatus(t, f.edit(issue, `{"title":"Doomed 2"}`), http.StatusOK)

	res := Request(t, f.app, "DELETE", f.issueURL(issue.IdIssuePublic), "", f.adminToken)
	requireStatus(t, res, http.StatusOK)

	var count int
	err := f.app.Pool.QueryRow(t.Context(), `SELECT count(*) FROM issues.issue_history WHERE id_issue = $1`, issue.IdIssue).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestIssueHistory_AccessAndMissingIssue(t *testing.T) {
	f := newHistoryFixture(t, "history-access")
	issue := f.createIssue("Private", nil, 0)
	stranger := f.userWithRole(t, "history-stranger@test.sk", "")

	res := Request(t, f.app, "GET", f.issueURL(issue.IdIssuePublic)+"/history", "", stranger)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	res = Request(t, f.app, "GET", f.issueURL(999999)+"/history", "", f.adminToken)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestIssueHistory_RejectedEditRecordsNothing(t *testing.T) {
	f := newHistoryFixture(t, "history-rejected")
	parent := f.createIssue("Vývoj", nil, 0)
	child := f.createIssue("Frontend", ptr(day(1)), 4*3600)
	f.link(parent, child)
	before := len(f.history(parent))

	res := f.edit(parent, `{"title":"Renamed","estimated":60}`)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SUMMARY_COMPUTED", errorCode(t, res))

	rows := f.history(parent)
	require.Len(t, rows, before)
	require.Empty(t, historyOf(rows, model.IssueHistoryFieldTitle, model.IssueHistorySourceUser))
}
