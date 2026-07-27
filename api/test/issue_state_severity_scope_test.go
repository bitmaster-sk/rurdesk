package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// CreateIssue/EditIssue/BulkEditIssues must reject an id_state/id_severity
// that is not mapped to the URL's project. The DB FK references the global
// state/severity tables, so without an explicit project-scope check a member of
// project A could attach a private state/severity belonging to project B —
// dropping the issue off A's board and creating a cross-project reference.

// createStateInProject creates a non-protected, project-local state and returns its id.
func createStateInProject(t *testing.T, app *issue.Application, token string, idProject int64, name string) int64 {
	body, _ := json.Marshal(model.CreateStateReq{IdProject: idProject, Name: name})
	res := Request(t, app, "POST", "/api/private/state", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var state model.State
	require.Nil(t, json.NewDecoder(res.Body).Decode(&state))
	return state.IdState
}

// createSeverityInProject creates a non-protected, project-local severity and returns its id.
func createSeverityInProject(t *testing.T, app *issue.Application, token string, idProject int64, title string) int64 {
	body, _ := json.Marshal(model.CreateSeverityReq{IdProject: idProject, Title: title, Color: "#123456"})
	res := Request(t, app, "POST", "/api/private/severity", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var sev model.Severity
	require.Nil(t, json.NewDecoder(res.Body).Decode(&sev))
	return sev.IdSeverity
}

func TestCreateIssue_ForeignState_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "bug103-create-state-A")
	projectB := createProject(t, app, token, "bug103-create-state-B")
	foreignState := createStateInProject(t, app, token, projectB, "B-only")

	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   projectA,
		IdState:     &foreignState,
		Title:       "cross-project state",
		Description: "x",
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(projectA)+"/issue", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestCreateIssue_ForeignSeverity_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "bug103-create-sev-A")
	projectB := createProject(t, app, token, "bug103-create-sev-B")
	foreignSeverity := createSeverityInProject(t, app, token, projectB, "B-only")

	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   projectA,
		IdSeverity:  &foreignSeverity,
		Title:       "cross-project severity",
		Description: "x",
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(projectA)+"/issue", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestCreateIssue_OwnProjectState_Accepted(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "bug103-create-ok")
	ownState := createStateInProject(t, app, token, projectA, "A-state")

	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   projectA,
		IdState:     &ownState,
		Title:       "own state",
		Description: "x",
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(projectA)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestEditIssue_ForeignState_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "bug103-edit-state-A")
	projectB := createProject(t, app, token, "bug103-edit-state-B")
	foreignState := createStateInProject(t, app, token, projectB, "B-only")

	// Seed a valid issue in project A.
	ownState := createStateInProject(t, app, token, projectA, "A-state")
	idIssuePublic := createIssueInState(t, app, token, projectA, ownState)

	editBody, _ := json.Marshal(model.EditIssueReq{
		IdProject:   projectA,
		IdState:     &foreignState,
		Title:       "moved to foreign state",
		Description: "x",
	})
	res := Request(t, app, "PATCH", "/api/private/project/"+itoa(projectA)+"/issue/"+itoa(idIssuePublic), string(editBody), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestEditIssue_ForeignSeverity_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "bug103-edit-sev-A")
	projectB := createProject(t, app, token, "bug103-edit-sev-B")
	foreignSeverity := createSeverityInProject(t, app, token, projectB, "B-only")

	ownState := createStateInProject(t, app, token, projectA, "A-state")
	idIssuePublic := createIssueInState(t, app, token, projectA, ownState)

	editBody, _ := json.Marshal(model.EditIssueReq{
		IdProject:   projectA,
		IdSeverity:  &foreignSeverity,
		Title:       "moved to foreign severity",
		Description: "x",
	})
	res := Request(t, app, "PATCH", "/api/private/project/"+itoa(projectA)+"/issue/"+itoa(idIssuePublic), string(editBody), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestBulkEditIssues_ForeignState_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	projectA := createProject(t, app, token, "bug103-bulk-state-A")
	projectB := createProject(t, app, token, "bug103-bulk-state-B")
	foreignState := createStateInProject(t, app, token, projectB, "B-only")

	ownState := createStateInProject(t, app, token, projectA, "A-state")
	idIssuePublic := createIssueInState(t, app, token, projectA, ownState)

	body, _ := json.Marshal(model.BulkEditIssuesReq{
		Issues: []model.BulkEditIssueEntryReq{
			{IdIssuePublic: idIssuePublic, IdState: &foreignState},
		},
	})
	res := Request(t, app, "PATCH", "/api/private/project/"+itoa(projectA)+"/issue/batch", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}
