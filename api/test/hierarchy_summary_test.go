package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type hierarchyFixture struct {
	*wikiFixture
	t *testing.T
}

func newHierarchyFixture(t *testing.T, name string) *hierarchyFixture {
	return &hierarchyFixture{wikiFixture: newWikiFixture(t, name), t: t}
}

var hierarchyBase = time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)

func day(n int) time.Time {
	return hierarchyBase.Add(time.Duration(n) * 24 * time.Hour)
}

func (f *hierarchyFixture) issueURL(idIssuePublic int64) string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d", f.idProject, idIssuePublic)
}

func (f *hierarchyFixture) createIssue(title string, start *time.Time, estimated int64) model.Issue {
	body := map[string]any{"title": title, "description": "d", "estimated": estimated}
	if start != nil {
		body["scheduledAt"] = start.Format(time.RFC3339)
	}
	raw, _ := json.Marshal(body)
	res := Request(f.t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/issue", f.idProject), string(raw), f.adminToken)
	requireStatus(f.t, res, http.StatusOK)
	var issue model.Issue
	require.NoError(f.t, json.NewDecoder(res.Body).Decode(&issue))
	return issue
}

func (f *hierarchyFixture) load(idIssuePublic int64) model.Issue {
	res := Request(f.t, f.app, "GET", f.issueURL(idIssuePublic), "", f.adminToken)
	require.Equal(f.t, http.StatusOK, res.StatusCode)
	var issue model.Issue
	require.NoError(f.t, json.NewDecoder(res.Body).Decode(&issue))
	return issue
}

func (f *hierarchyFixture) link(parent, child model.Issue) int64 {
	res := f.relate(parent, child, "hierarchy", "")
	requireStatus(f.t, res, http.StatusOK)
	var views []model.ReadIssueRelationRes
	require.NoError(f.t, json.NewDecoder(res.Body).Decode(&views))
	require.NotEmpty(f.t, views)
	return views[0].IdIssueRelation
}

func (f *hierarchyFixture) relate(from, to model.Issue, relationType, subType string) *http.Response {
	body := fmt.Sprintf(`{"idIssuePublicTo":%d,"relationType":%q`, to.IdIssuePublic, relationType)
	if subType != "" {
		body += fmt.Sprintf(`,"relationSubType":%q`, subType)
	}
	body += "}"
	return Request(f.t, f.app, "POST", f.issueURL(from.IdIssuePublic)+"/relation", body, f.adminToken)
}

func (f *hierarchyFixture) schedule(from, to model.Issue) *http.Response {
	return f.relate(from, to, "schedule", "finish_to_start")
}

func (f *hierarchyFixture) edit(issue model.Issue, body string) *http.Response {
	return Request(f.t, f.app, "PATCH", f.issueURL(issue.IdIssuePublic), body, f.adminToken)
}

func (f *hierarchyFixture) setParent(token string, child model.Issue, parent *model.Issue) *http.Response {
	body := `{"idParent":null}`
	if parent != nil {
		body = fmt.Sprintf(`{"idParent":%d}`, parent.IdIssuePublic)
	}
	return Request(f.t, f.app, "PUT", f.issueURL(child.IdIssuePublic)+"/parent", body, token)
}

func (f *hierarchyFixture) parentOf(child model.Issue) int64 {
	var idParent int64
	err := f.app.Pool.QueryRow(f.t.Context(), `
		SELECT COALESCE((SELECT p.id_issue_public FROM issues.issue_relation r
			INNER JOIN issues.issue p ON p.id_issue = r.id_issue_from
			WHERE r.relation_type = 'hierarchy' AND r.id_issue_to = $1), 0)
	`, child.IdIssue).Scan(&idParent)
	require.NoError(f.t, err)
	return idParent
}

func requireStatus(t *testing.T, res *http.Response, status int) {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
}

func requireSpan(t *testing.T, issue model.Issue, start time.Time, end time.Time) {
	require.NotNil(t, issue.ScheduledAt)
	require.True(t, issue.ScheduledAt.Equal(start), "start %s, want %s", issue.ScheduledAt, start)
	require.Equal(t, int64(end.Sub(start).Seconds()), issue.Estimated)
}

func TestHierarchy_ScheduledChildMakesParentSummary(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-summary")
	parent := f.createIssue("Vývoj", ptr(day(20)), 40*3600)
	child := f.createIssue("Endpointy", ptr(day(1)), 2*24*3600)

	f.link(parent, child)

	got := f.load(parent.IdIssuePublic)
	require.True(t, got.IsSummary)
	requireSpan(t, got, day(1), day(3))
}

func TestHierarchy_ParentWithOnlyBacklogChildrenKeepsItsDates(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-backlog-only")
	parent := f.createIssue("Návrh", ptr(day(4)), 3600)
	child := f.createIssue("SEO texty", nil, 7200)

	f.link(parent, child)

	got := f.load(parent.IdIssuePublic)
	require.False(t, got.IsSummary)
	requireSpan(t, got, day(4), day(4).Add(time.Hour))
	require.Equal(t, 1, got.BacklogChildren)
}

func TestHierarchy_GrandchildChangeRecomputesTheWholeChain(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-chain")
	root := f.createIssue("Vývoj", nil, 0)
	middle := f.createIssue("Backend API", nil, 0)
	leaf := f.createIssue("Endpointy", ptr(day(1)), 24*3600)
	f.link(root, middle)
	f.link(middle, leaf)

	res := f.edit(leaf, fmt.Sprintf(`{"scheduledAt":%q}`, day(5).Format(time.RFC3339)))
	requireStatus(t, res, http.StatusOK)

	requireSpan(t, f.load(middle.IdIssuePublic), day(5), day(6))
	requireSpan(t, f.load(root.IdIssuePublic), day(5), day(6))
}

func TestHierarchy_RemovingTheLastChildKeepsTheLastComputedDates(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-unlink")
	parent := f.createIssue("Vývoj", nil, 0)
	first := f.createIssue("Frontend", ptr(day(2)), 24*3600)
	second := f.createIssue("Backend", ptr(day(3)), 24*3600)
	idRelation := f.link(parent, first)
	f.link(parent, second)

	res := Request(t, f.app, "DELETE", fmt.Sprintf("%s/relation/%d", f.issueURL(parent.IdIssuePublic), idRelation), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	requireSpan(t, f.load(parent.IdIssuePublic), day(3), day(4))

	res = Request(t, f.app, "DELETE", f.issueURL(second.IdIssuePublic), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)

	got := f.load(parent.IdIssuePublic)
	require.False(t, got.IsSummary)
	requireSpan(t, got, day(3), day(4))
	require.Nil(t, got.RollupEstimated)
}

func TestHierarchy_DirectScheduleWriteOnSummaryIsRejected(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-readonly")
	parent := f.createIssue("Vývoj", nil, 0)
	child := f.createIssue("Frontend", ptr(day(2)), 24*3600)
	f.link(parent, child)
	summary := f.load(parent.IdIssuePublic)

	res := f.edit(summary, fmt.Sprintf(`{"scheduledAt":%q}`, day(9).Format(time.RFC3339)))
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SUMMARY_COMPUTED", errorCode(t, res))

	res = f.edit(summary, `{"estimated":60}`)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	res = f.edit(summary, fmt.Sprintf(`{"title":"Vývoj 2","scheduledAt":%q,"estimated":%d}`,
		summary.ScheduledAt.Format(time.RFC3339), summary.Estimated))
	requireStatus(t, res, http.StatusOK)

	bulk := fmt.Sprintf(`{"issues":[{"idIssuePublic":%d,"scheduledAt":%q}]}`,
		summary.IdIssuePublic, day(9).Format(time.RFC3339))
	res = Request(t, f.app, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/batch", f.idProject), bulk, f.adminToken)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "SUMMARY_COMPUTED", errorCode(t, res))

	tools := &wikiAgentFixture{wikiFixture: f.wikiFixture, mcpSessions: map[string]string{}}
	result := tools.callTool(t, f.adminToken, "", "update_issue", map[string]any{
		"project_id": f.idProject, "issue_id": summary.IdIssuePublic, "scheduled_at": day(9).Format(time.RFC3339),
	})
	require.True(t, result.IsError)
	require.Contains(t, result.Text, "computed from its subtasks")

	requireSpan(t, f.load(parent.IdIssuePublic), day(2), day(3))
}

func TestHierarchy_RollupCountsLeavesAndTheParentsOwnTrackedTime(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-rollup")
	parent := f.createIssue("Vývoj", nil, 0)
	estimated := f.createIssue("Frontend", ptr(day(1)), 4*3600)
	unestimated := f.createIssue("Migrácie", ptr(day(1)), 0)
	f.link(parent, estimated)
	f.link(parent, unestimated)

	track := func(idIssue int64, seconds int) {
		res := Request(t, f.app, "POST", "/api/private/track", fmt.Sprintf(`{"idIssue":%d,"tracked":%d}`, idIssue, seconds), f.adminToken)
		requireStatus(t, res, http.StatusOK)
	}
	track(parent.IdIssue, 600)
	track(estimated.IdIssue, 1800)

	got := f.load(parent.IdIssuePublic)
	require.NotNil(t, got.RollupEstimated)
	require.Equal(t, int64(4*3600+3600), *got.RollupEstimated)
	require.NotNil(t, got.RollupTracked)
	require.Equal(t, int64(2400), *got.RollupTracked)

	track(parent.IdIssue, 300)
	require.Equal(t, int64(2700), *f.load(parent.IdIssuePublic).RollupTracked)
}

func TestHierarchy_SetParentIndentsOutdentsAndRefusesCycles(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-set-parent")
	phase := f.createIssue("Fáza 1", nil, 0)
	task := f.createIssue("Testovanie", ptr(day(6)), 24*3600)
	sub := f.createIssue("Regresia", ptr(day(7)), 24*3600)
	f.link(task, sub)

	res := f.setParent(f.adminToken, task, &phase)
	requireStatus(t, res, http.StatusOK)
	require.Equal(t, phase.IdIssuePublic, f.parentOf(task))
	requireSpan(t, f.load(phase.IdIssuePublic), day(7), day(8))

	res = f.setParent(f.adminToken, phase, &sub)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, int64(0), f.parentOf(phase))

	viewer := f.userWithRole(t, "hierarchy-viewer@test.sk", string(model.RoleViewer))
	res = f.setParent(viewer, task, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.Equal(t, phase.IdIssuePublic, f.parentOf(task))

	res = f.setParent(f.adminToken, task, nil)
	requireStatus(t, res, http.StatusOK)
	require.Equal(t, int64(0), f.parentOf(task))
	require.False(t, f.load(phase.IdIssuePublic).IsSummary)
}

func TestHierarchy_ScheduleAndHierarchyMayNotFormACycle(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-mixed-cycle")
	build := f.createIssue("Vývoj", nil, 0)
	frontend := f.createIssue("Frontend", ptr(day(1)), 24*3600)
	qa := f.createIssue("Testovanie", ptr(day(3)), 24*3600)
	launch := f.createIssue("Spustenie", ptr(day(5)), 3600)
	f.link(build, frontend)
	require.Equal(t, http.StatusOK, f.schedule(build, qa).StatusCode)
	require.Equal(t, http.StatusOK, f.schedule(qa, launch).StatusCode)

	res := f.setParent(f.adminToken, launch, &frontend)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "CYCLE", errorCode(t, res))
	require.Equal(t, int64(0), f.parentOf(launch))

	res = f.relate(frontend, launch, "hierarchy", "")
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	res = f.schedule(build, frontend)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	sibling := f.createIssue("Backend", ptr(day(1)), 24*3600)
	f.link(build, sibling)
	require.Equal(t, http.StatusOK, f.schedule(sibling, frontend).StatusCode)
}

func TestHierarchy_SplitAndProjectBuilderFillTheRollup(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-split")
	parent := f.createIssue("Vývoj", nil, 0)

	res := Request(t, f.app, "POST", f.issueURL(parent.IdIssuePublic)+"/split/accept", `{"children":[
		{"title":"A","description":"a","estimatedMinutes":7200},
		{"title":"B","description":"b"}
	]}`, f.adminToken)
	requireStatus(t, res, http.StatusOK)

	got := f.load(parent.IdIssuePublic)
	require.False(t, got.IsSummary)
	require.Equal(t, 2, got.BacklogChildren)
	require.NotNil(t, got.RollupEstimated)
	require.Equal(t, int64(7200+3600), *got.RollupEstimated)

	res = Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/project-builder/accept", f.idProject), `{"issues":[
		{"ref":"P","title":"Fáza","description":"p","estimatedMinutes":0,"hierarchyParentRef":""},
		{"ref":"C","title":"Krok","description":"c","estimatedMinutes":5400,"hierarchyParentRef":"P"}
	]}`, f.adminToken)
	requireStatus(t, res, http.StatusOK)
	var accepted model.ProjectBuilderAcceptRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&accepted))
	for _, issue := range accepted.Issues {
		if issue.Title != "Fáza" {
			continue
		}
		require.NotNil(t, issue.RollupEstimated)
		require.Equal(t, int64(5400), *issue.RollupEstimated)
		require.Equal(t, 1, issue.BacklogChildren)
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestHierarchy_IndentUnderTheOwnPredecessorIsNotACycle(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-indent-predecessor")
	design := f.createIssue("Návrh", ptr(day(1)), 24*3600)
	build := f.createIssue("Implementácia", ptr(day(2)), 24*3600)
	launch := f.createIssue("Spustenie", ptr(day(4)), 3600)
	require.Equal(t, http.StatusOK, f.schedule(design, build).StatusCode)

	res := f.setParent(f.adminToken, build, &design)
	requireStatus(t, res, http.StatusOK)
	require.Equal(t, design.IdIssuePublic, f.parentOf(build))

	require.Equal(t, http.StatusOK, f.schedule(build, launch).StatusCode)
	res = f.schedule(launch, design)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Equal(t, "CYCLE", errorCode(t, res))
}

func TestHierarchy_ConcurrentSiblingChangesBothReachTheSummary(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-concurrent")
	parent := f.createIssue("Vývoj", nil, 0)
	first := f.createIssue("Backend", ptr(day(1)), 3600)
	second := f.createIssue("Frontend", ptr(day(2)), 3600)
	f.link(parent, first)
	f.link(parent, second)
	ctx := t.Context()
	repo := repository.NewIssueHierarchyRepository(f.app.Pool)
	change := func(tx pgx.Tx, child model.Issue, estimated int64) error {
		if _, err := tx.Exec(ctx, `UPDATE issues.issue SET estimated = $1 WHERE id_issue = $2`, estimated, child.IdIssue); err != nil {
			return err
		}
		_, err := repo.RecomputeNodes(extctx.WithTx(ctx, tx), []int64{parent.IdIssue})
		return err
	}

	tx1, err := f.app.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, change(tx1, first, 2*3600))
	done := make(chan error, 1)
	go func() {
		tx2, err := f.app.Pool.Begin(ctx)
		if err != nil {
			done <- err
			return
		}
		if err := change(tx2, second, 5*3600); err != nil {
			_ = tx2.Rollback(ctx)
			done <- err
			return
		}
		done <- tx2.Commit(ctx)
	}()
	time.Sleep(300 * time.Millisecond)
	require.NoError(t, tx1.Commit(ctx))
	require.NoError(t, <-done)

	got := f.load(parent.IdIssuePublic)
	require.NotNil(t, got.RollupEstimated)
	require.Equal(t, int64(7*3600), *got.RollupEstimated)
}

func TestHierarchy_SetParentMovesTheOwnEstimateToARemainingWorkSubtask(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-remaining-work")
	createCustomField(t, f.app, f.adminToken, model.CreateCustomFieldReq{
		IdProject: f.idProject, Key: "impact", Name: "Impact",
		FieldType: constants.CustomFieldTypeNumber, IsRequired: true})
	parent := createIssueWithCustomFields(t, f.app, f.adminToken, f.idProject, "Vývoj", map[string]any{"impact": 7})
	child := createIssueWithCustomFields(t, f.app, f.adminToken, f.idProject, "Endpointy", map[string]any{"impact": 1})
	requireStatus(t, f.edit(parent, fmt.Sprintf(`{"scheduledAt":%q,"estimated":%d}`, day(1).Format(time.RFC3339), 8*3600)), http.StatusOK)
	requireStatus(t, f.edit(child, fmt.Sprintf(`{"scheduledAt":%q,"estimated":%d}`, day(3).Format(time.RFC3339), 3600)), http.StatusOK)

	body := fmt.Sprintf(`{"idParent":%d,"remainingWork":{"title":"Vývoj – remaining work","description":"remaining work"}}`, parent.IdIssuePublic)
	res := Request(t, f.app, "PUT", f.issueURL(child.IdIssuePublic)+"/parent", body, f.adminToken)
	requireStatus(t, res, http.StatusOK)
	var out struct {
		Relations []model.ReadIssueRelationRes `json:"relations"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	require.Len(t, out.Relations, 4)

	var idRemaining int64
	require.NoError(t, f.app.Pool.QueryRow(t.Context(), `
		SELECT c.id_issue_public FROM issues.issue_relation r
		INNER JOIN issues.issue c ON c.id_issue = r.id_issue_to
		WHERE r.relation_type = 'hierarchy' AND r.id_issue_from = $1 AND c.id_issue <> $2
	`, parent.IdIssue, child.IdIssue).Scan(&idRemaining))
	remaining := f.load(idRemaining)
	require.Equal(t, "Vývoj – remaining work", remaining.Title)
	requireSpan(t, remaining, day(1), day(1).Add(8*time.Hour))
	require.Equal(t, float64(7), loadIssueCustomFields(t, f.app, f.adminToken, f.idProject, idRemaining)["impact"])

	got := f.load(parent.IdIssuePublic)
	require.True(t, got.IsSummary)
	require.NotNil(t, got.RollupEstimated)
	require.Equal(t, int64(9*3600), *got.RollupEstimated)
}

func TestHierarchy_SetParentWithoutRemainingWorkDropsTheOwnEstimate(t *testing.T) {
	f := newHierarchyFixture(t, "hierarchy-drop-estimate")
	parent := f.createIssue("Vývoj", ptr(day(1)), 8*3600)
	child := f.createIssue("Endpointy", ptr(day(3)), 3600)

	requireStatus(t, f.setParent(f.adminToken, child, &parent), http.StatusOK)

	got := f.load(parent.IdIssuePublic)
	require.True(t, got.IsSummary)
	require.NotNil(t, got.RollupEstimated)
	require.Equal(t, int64(3600), *got.RollupEstimated)
}
