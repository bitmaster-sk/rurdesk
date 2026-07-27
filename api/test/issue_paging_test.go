package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// --- local helpers built on the real Request-based harness ---

func createIssueIn(t *testing.T, app *issue.Application, token string, idProject int64, title string) int64 {
	body := fmt.Sprintf(`{"title":%q,"description":"desc","estimated":0}`, title)
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject), body, token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var i model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&i))
	return i.IdIssuePublic
}

func createRelation(t *testing.T, app *issue.Application, token string, idProject, from, to int64) {
	body := fmt.Sprintf(`{"idIssuePublicTo":%d,"relationType":"relates_to"}`, to)
	res := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/relation", idProject, from), body, token)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func loadIssuesEnvelope(t *testing.T, app *issue.Application, token string, idProject int64, query string) model.IssuesPageRes {
	url := fmt.Sprintf("/api/private/project/%d/issue%s", idProject, query)
	res := Request(t, app, "GET", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var page model.IssuesPageRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&page))
	return page
}

func findIssue(items []*model.Issue, idPublic int64) *model.Issue {
	for _, it := range items {
		if it.IdIssuePublic == idPublic {
			return it
		}
	}
	return nil
}

func TestRelationCount(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "paging-relcount")

	a := createIssueIn(t, app, token, pid, "A")
	b := createIssueIn(t, app, token, pid, "B")
	createRelation(t, app, token, pid, a, b)

	page := loadIssuesEnvelope(t, app, token, pid, "")
	if got := findIssue(page.Items, a); got == nil || got.RelationCount != 1 {
		t.Fatalf("A relationCount: %+v", got)
	}
	if got := findIssue(page.Items, b); got == nil || got.RelationCount != 1 {
		t.Fatalf("B relationCount: %+v", got)
	}
}

func TestNoLimitReturnsAll(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "paging-nolimit")
	for i := 0; i < 60; i++ {
		createIssueIn(t, app, token, pid, fmt.Sprintf("X-%d", i))
	}
	page := loadIssuesEnvelope(t, app, token, pid, "")
	if page.NextCursor != nil || len(page.Items) != 60 || page.Total != 60 {
		t.Fatalf("want all 60 nil cursor: items=%d cursor=%v total=%d", len(page.Items), page.NextCursor, page.Total)
	}
}

func TestKeysetPaging_WalksAllRowsNoDupes(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "paging-walk")
	for i := 0; i < 125; i++ {
		createIssueIn(t, app, token, pid, fmt.Sprintf("I-%03d", i))
	}

	seen := map[int64]bool{}
	cursor := ""
	pages := 0
	for {
		query := "?limit=50"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		page := loadIssuesEnvelope(t, app, token, pid, query)
		for _, it := range page.Items {
			if seen[it.IdIssuePublic] {
				t.Fatalf("duplicate issue %d across pages", it.IdIssuePublic)
			}
			seen[it.IdIssuePublic] = true
		}
		if page.Total != 125 {
			t.Fatalf("total = %d want 125", page.Total)
		}
		pages++
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 125 {
		t.Fatalf("walked %d rows want 125", len(seen))
	}
	if pages != 3 {
		t.Fatalf("pages = %d want 3 (50+50+25)", pages)
	}
}

func TestGroupedByState_TopNAndTotals(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "paging-grouped")
	// All issues have no state -> one (NULL) state group; exercises the window-fn path + total + cursor.
	for i := 0; i < 25; i++ {
		createIssueIn(t, app, token, pid, fmt.Sprintf("G-%02d", i))
	}

	url := fmt.Sprintf("/api/private/project/%d/issue?groupBy=state&limit=10", pid)
	res := Request(t, app, "GET", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var body struct {
		Groups []model.IssueGroupRes `json:"groups"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Len(t, body.Groups, 1)
	g := body.Groups[0]
	if len(g.Items) != 10 || g.Total != 25 || g.NextCursor == nil {
		t.Fatalf("group: items=%d total=%d cursor=%v", len(g.Items), g.Total, g.NextCursor)
	}
}

// The kanban board loads tiles via the grouped query; the toolbar sorter must reorder
// the tiles within each group (previously the window ordering was hardcoded to updateAt).
func TestGroupedByState_HonorsSortOrder(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "paging-grouped-sort")
	// Created out of title order so a real sort can't be mistaken for creation order.
	for _, title := range []string{"C", "A", "E", "B", "D"} {
		createIssueIn(t, app, token, pid, title)
	}

	loadTitles := func(query string) []string {
		url := fmt.Sprintf("/api/private/project/%d/issue?groupBy=state&limit=10&%s", pid, query)
		res := Request(t, app, "GET", url, "", token)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var body struct {
			Groups []model.IssueGroupRes `json:"groups"`
		}
		require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
		require.Len(t, body.Groups, 1)
		titles := make([]string, len(body.Groups[0].Items))
		for i, it := range body.Groups[0].Items {
			titles[i] = it.Title
		}
		return titles
	}

	require.Equal(t, []string{"A", "B", "C", "D", "E"},
		loadTitles("orderColumn=title&orderDirection=asc"))
	require.Equal(t, []string{"E", "D", "C", "B", "A"},
		loadTitles("orderColumn=title&orderDirection=desc"))
}

// A kanban column loads its first tiles from the grouped query and then pages on
// with the cursor that query hands back (the client feeds it straight to the flat
// paged endpoint — see IssueKanbanService.loadMoreColumn). So the cursor must be
// expressed in the same sort the group was ordered by. When it isn't, the second
// page is filtered on one column while being ordered by another: rows that belong
// after the boundary silently vanish and rows already shown come back.
//
// update_at is pinned to match title order so the two orderings genuinely
// disagree at the boundary — with the default updateAt/desc cursor, everything
// after "B" sorts on the wrong side of the predicate.
func TestGroupedByState_CursorFollowsSortOrder(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "paging-grouped-cursor")

	titles := []string{"A", "B", "C", "D", "E"}
	for i, title := range titles {
		idPublic := createIssueIn(t, app, token, pid, title)
		_, err := app.Pool.Exec(context.Background(), `
			UPDATE issues.issue SET update_at = now() - make_interval(mins => $1)
			WHERE id_project = $2 AND id_issue_public = $3`,
			len(titles)-i, pid, idPublic)
		require.Nil(t, err)
	}

	grouped := fmt.Sprintf(
		"/api/private/project/%d/issue?groupBy=state&limit=2&orderColumn=title&orderDirection=asc", pid)
	res := Request(t, app, "GET", grouped, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var body struct {
		Groups []model.IssueGroupRes `json:"groups"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Len(t, body.Groups, 1)

	group := body.Groups[0]
	require.NotNil(t, group.NextCursor, "5 issues at limit 2 must page")

	var walked []string
	for _, it := range group.Items {
		walked = append(walked, it.Title)
	}

	cursor := *group.NextCursor
	for page := 0; page < len(titles); page++ {
		next := loadIssuesEnvelope(t, app, token, pid,
			fmt.Sprintf("?limit=2&orderColumn=title&orderDirection=asc&cursor=%s", cursor))
		for _, it := range next.Items {
			walked = append(walked, it.Title)
		}
		if next.NextCursor == nil {
			break
		}
		cursor = *next.NextCursor
	}

	require.Equal(t, titles, walked,
		"paging a sorted group must visit every issue exactly once, in sort order")
}
