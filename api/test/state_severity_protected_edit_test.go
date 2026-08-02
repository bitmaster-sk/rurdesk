package test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// editing/reordering a protected (default) state/severity forks it into a
// project-local copy. The fork must (a) migrate the project's issues onto the new
// row so they are not orphaned on the now-unmapped shared row, and (b) keep the
// requested order_rank. Deleting a protected state must unassign (NULL) the
// project's issues rather than orphan them.

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func loadProjectStates(t *testing.T, app *issue.Application, token string, idProject int64) []model.State {
	res := Request(t, app, "GET", "/api/private/state", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var all []model.State
	require.Nil(t, json.NewDecoder(res.Body).Decode(&all))
	out := make([]model.State, 0)
	for _, s := range all {
		if s.IdProject == idProject {
			out = append(out, s)
		}
	}
	return out
}

func loadProjectSeverities(t *testing.T, app *issue.Application, token string, idProject int64) []model.Severity {
	res := Request(t, app, "GET", "/api/private/severity", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var all []model.Severity
	require.Nil(t, json.NewDecoder(res.Body).Decode(&all))
	out := make([]model.Severity, 0)
	for _, s := range all {
		if s.IdProject == idProject {
			out = append(out, s)
		}
	}
	return out
}

func idStateSet(states []model.State) map[int64]bool {
	set := make(map[int64]bool, len(states))
	for _, s := range states {
		set[s.IdState] = true
	}
	return set
}

func createIssueInState(t *testing.T, app *issue.Application, token string, idProject, idState int64) int64 {
	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   idProject,
		IdState:     &idState,
		Title:       "bug25 issue",
		Description: "x",
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created.IdIssuePublic
}

func createIssueInSeverity(t *testing.T, app *issue.Application, token string, idProject, idSeverity int64) int64 {
	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   idProject,
		IdSeverity:  &idSeverity,
		Title:       "bug25 issue",
		Description: "x",
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created.IdIssuePublic
}

func loadIssue(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64) model.Issue {
	res := Request(t, app, "GET", "/api/private/project/"+itoa(idProject)+"/issue?limit=200", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var page model.IssuesPageRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&page))
	for _, it := range page.Items {
		if it.IdIssuePublic == idIssuePublic {
			return *it
		}
	}
	t.Fatalf("issue %d not found in project %d", idIssuePublic, idProject)
	return model.Issue{}
}

func TestEditProtectedState_MigratesIssuesAndKeepsOrder(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "bug25-state-edit")

	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states, "project should have seeded default states")
	orig := states[0]
	require.True(t, orig.Protected, "expected a protected default state")
	origRank := orig.OrderRank

	idIssuePublic := createIssueInState(t, app, token, idProject, orig.IdState)

	editBody, _ := json.Marshal(model.EditStateReq{
		IdState:   orig.IdState,
		IdProject: idProject,
		Name:      "Renamed",
		Start:     orig.Start,
		Final:     orig.Final,
		OrderRank: origRank,
	})
	res := Request(t, app, "PATCH", "/api/private/state/"+itoa(orig.IdState), string(editBody), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var forked model.State
	require.Nil(t, json.NewDecoder(res.Body).Decode(&forked))
	require.NotEqual(t, orig.IdState, forked.IdState, "fork must produce a new id_state")

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, issue.IdState, "issue must not be unassigned by an edit")
	require.Equal(t, forked.IdState, *issue.IdState, "issue must follow the fork")

	mapped := idStateSet(loadProjectStates(t, app, token, idProject))
	require.Contains(t, mapped, *issue.IdState, "issue's state must stay mapped to the project (not orphaned)")
	require.NotContains(t, mapped, orig.IdState, "old shared state must no longer be mapped")

	for _, s := range loadProjectStates(t, app, token, idProject) {
		if s.IdState == forked.IdState {
			require.Equal(t, origRank, s.OrderRank, "forked state must keep the original order_rank")
		}
	}
}

func TestDeleteProtectedState_UnassignsIssues(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "bug25-state-delete")

	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)
	target := states[0]
	require.True(t, target.Protected)

	idIssuePublic := createIssueInState(t, app, token, idProject, target.IdState)

	res := Request(t, app, "DELETE", "/api/private/state/"+itoa(target.IdState)+"/project/"+itoa(idProject)+"?migrateTo=null", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Nil(t, issue.IdState, "deleting the protected state must unassign the issue")

	mapped := idStateSet(loadProjectStates(t, app, token, idProject))
	require.NotContains(t, mapped, target.IdState, "deleted state must no longer be mapped")
}

func TestEditProtectedSeverity_MigratesIssues(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "bug25-sev-edit")

	sevs := loadProjectSeverities(t, app, token, idProject)
	require.NotEmpty(t, sevs)
	orig := sevs[0]
	require.True(t, orig.Protected)

	idIssuePublic := createIssueInSeverity(t, app, token, idProject, orig.IdSeverity)

	editBody, _ := json.Marshal(model.EditSeverityReq{
		IdSeverity: orig.IdSeverity,
		IdProject:  idProject,
		Title:      "Renamed",
		Color:      orig.Color,
		OrderRank:  orig.OrderRank,
	})
	res := Request(t, app, "PATCH", "/api/private/severity/"+itoa(orig.IdSeverity), string(editBody), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var forked model.Severity
	require.Nil(t, json.NewDecoder(res.Body).Decode(&forked))
	require.NotEqual(t, orig.IdSeverity, forked.IdSeverity)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, issue.IdSeverity)
	require.Equal(t, forked.IdSeverity, *issue.IdSeverity, "issue must follow the severity fork")
}
