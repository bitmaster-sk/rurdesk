package test

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/require"
)

// assertScheduledOrder reads the persisted ranks and asserts the public-id order.
// Ranked rows sort by rank; nil-rank rows sort last, tie-broken by IdIssuePublic —
// a deterministic tie-break for these assertions only, NOT the frontend's
// scheduledAt/title rule (that nil ordering is covered by the Task 8 unit spec).
func assertScheduledOrder(t *testing.T, idProject int64, want []int64) {
	t.Helper()
	rows, err := injector.GetIssueRepository().LoadScheduledGanttRanks(context.Background(), idProject)
	require.Nil(t, err)
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := rows[i].GanttRank, rows[j].GanttRank
		if ri == nil && rj == nil {
			return rows[i].IdIssuePublic < rows[j].IdIssuePublic
		}
		if ri == nil {
			return false
		}
		if rj == nil {
			return true
		}
		return *ri < *rj
	})
	got := make([]int64, len(rows))
	for i, r := range rows {
		got[i] = r.IdIssuePublic
	}
	require.Equal(t, want, got)
}

func TestGanttOrder_SeedThenMove(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-order-proj")
	a := createScheduledIssue(t, app, token, idProject, "A").IdIssuePublic
	b := createScheduledIssue(t, app, token, idProject, "B").IdIssuePublic
	c := createScheduledIssue(t, app, token, idProject, "C").IdIssuePublic

	// First drag: move c to the top. All ranks null → seed from order.
	body := fmt.Sprintf(`{"movedId":%d,"order":[%d,%d,%d]}`, c, c, a, b)
	res := Request(t, app, "PUT", ganttOrderURL(idProject), body, token)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	assertScheduledOrder(t, idProject, []int64{c, a, b})

	// Second drag: move a to the bottom. Project already ranked → single write.
	body2 := fmt.Sprintf(`{"movedId":%d,"order":[%d,%d,%d]}`, a, c, b, a)
	res2 := Request(t, app, "PUT", ganttOrderURL(idProject), body2, token)
	require.Equal(t, http.StatusNoContent, res2.StatusCode)
	assertScheduledOrder(t, idProject, []int64{c, b, a})
}

// A newly scheduled (unranked) row dragged mid-list must NOT reseed / rewrite the
// existing ranked rows — only the moved row gets a rank.
func TestGanttOrder_DoesNotReseedRankedRows(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-order-noreseed")
	a := createScheduledIssue(t, app, token, idProject, "A").IdIssuePublic
	b := createScheduledIssue(t, app, token, idProject, "B").IdIssuePublic

	// Seed a,b.
	seed := fmt.Sprintf(`{"movedId":%d,"order":[%d,%d]}`, a, a, b)
	require.Equal(t, http.StatusNoContent, Request(t, app, "PUT", ganttOrderURL(idProject), seed, token).StatusCode)

	repo := injector.GetIssueRepository()
	before, err := repo.LoadScheduledGanttRanks(context.Background(), idProject)
	require.Nil(t, err)
	rankOf := func(rows []repository.GanttRankRow, pub int64) *string {
		for _, r := range rows {
			if r.IdIssuePublic == pub {
				return r.GanttRank
			}
		}
		return nil
	}

	// Schedule a 3rd row (unranked), drag it between a and b.
	c := createScheduledIssue(t, app, token, idProject, "C").IdIssuePublic
	move := fmt.Sprintf(`{"movedId":%d,"order":[%d,%d,%d]}`, c, a, c, b)
	require.Equal(t, http.StatusNoContent, Request(t, app, "PUT", ganttOrderURL(idProject), move, token).StatusCode)

	after, err := repo.LoadScheduledGanttRanks(context.Background(), idProject)
	require.Nil(t, err)
	require.Equal(t, *rankOf(before, a), *rankOf(after, a), "rank of a must be untouched")
	require.Equal(t, *rankOf(before, b), *rankOf(after, b), "rank of b must be untouched")
	assertScheduledOrder(t, idProject, []int64{a, c, b})
}

// A stale/inverted anchor pair (client order contradicts persisted ranks) → 400.
func TestGanttOrder_RejectsInvertedAnchors(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-order-stale")
	a := createScheduledIssue(t, app, token, idProject, "A")
	b := createScheduledIssue(t, app, token, idProject, "B")
	c := createScheduledIssue(t, app, token, idProject, "C")

	// Force ranks a<b<c directly, then send an order whose anchors around the moved
	// row are inverted (prev=c, next=a with c>a) → guard must reject.
	repo := injector.GetIssueRepository()
	ctx := context.Background()
	require.Nil(t, repo.SetGanttRank(ctx, a.IdIssue, "b"))
	require.Nil(t, repo.SetGanttRank(ctx, b.IdIssue, "n"))
	require.Nil(t, repo.SetGanttRank(ctx, c.IdIssue, "t"))

	body := fmt.Sprintf(`{"movedId":%d,"order":[%d,%d,%d]}`, b.IdIssuePublic, c.IdIssuePublic, b.IdIssuePublic, a.IdIssuePublic)
	res := Request(t, app, "PUT", ganttOrderURL(idProject), body, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestGanttOrder_RejectsForeignIssue(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-order-A")
	other := createProject(t, app, token, "gantt-order-B")
	foreign := createScheduledIssue(t, app, token, other, "foreign").IdIssuePublic

	body := fmt.Sprintf(`{"movedId":%d,"order":[%d]}`, foreign, foreign)
	res := Request(t, app, "PUT", ganttOrderURL(idProject), body, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestGanttOrder_RejectsDuplicateIds(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-order-dup")
	a := createScheduledIssue(t, app, token, idProject, "A").IdIssuePublic
	b := createScheduledIssue(t, app, token, idProject, "B").IdIssuePublic

	body := fmt.Sprintf(`{"movedId":%d,"order":[%d,%d,%d]}`, a, a, b, a)
	res := Request(t, app, "PUT", ganttOrderURL(idProject), body, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestGanttOrder_RejectsMovedIdNotInOrder(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-order-nomoved")
	a := createScheduledIssue(t, app, token, idProject, "A").IdIssuePublic
	b := createScheduledIssue(t, app, token, idProject, "B").IdIssuePublic

	body := fmt.Sprintf(`{"movedId":%d,"order":[%d]}`, a, b) // movedId a not present in order
	res := Request(t, app, "PUT", ganttOrderURL(idProject), body, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestGanttOrder_ForbiddenForNonMember(t *testing.T) {
	app := Setup(t)
	owner := Token(t, app)
	idProject := createProject(t, app, owner, "gantt-order-priv")
	a := createScheduledIssue(t, app, owner, idProject, "A").IdIssuePublic

	// A second user who is NOT a member of the project → CanUpdateIssue is false.
	outsider := createUserAsAdmin(t, app, owner,
		`{"name":"out","email":"outsider@test.sk","password":"kreslo"}`)

	body := fmt.Sprintf(`{"movedId":%d,"order":[%d]}`, a, a)
	res := Request(t, app, "PUT", ganttOrderURL(idProject), body, outsider)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}
