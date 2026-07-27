package test

import (
	"fmt"
	"net/http"
	"testing"
)

// After GanttRank is added to model.Issue, the strict-scan queries (kanban
// grouped and the paged/backlog list) must still load without a scan error.
func TestGanttRank_StrictScanPathsStillLoad(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-rank-proj")
	createIssueIn(t, app, token, idProject, "issue A")

	base := fmt.Sprintf("/api/private/project/%d/issue", idProject)

	// Grouped (kanban) strict-scan path
	resGrouped := Request(t, app, "GET", base+"?groupBy=state", "", token)
	if resGrouped.StatusCode != http.StatusOK {
		t.Fatalf("grouped load: want 200, got %d", resGrouped.StatusCode)
	}
	// Flat/paged strict-scan path
	resList := Request(t, app, "GET", base, "", token)
	if resList.StatusCode != http.StatusOK {
		t.Fatalf("list load: want 200, got %d", resList.StatusCode)
	}
}
