package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// --- helpers ---------------------------------------------------------------

func createSprint(t *testing.T, app *issue.Application, token string, idProject int64, body string) model.Sprint {
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/sprint", idProject), body, token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var sprint model.Sprint
	require.Nil(t, json.NewDecoder(res.Body).Decode(&sprint))
	return sprint
}

func assignSprint(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64, body string) *http.Response {
	return Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d/sprint", idProject, idIssuePublic), body, token)
}

func closeSprint(t *testing.T, app *issue.Application, token string, idSprint int64) *http.Response {
	return Request(t, app, "POST", fmt.Sprintf("/api/private/sprint/%d/close", idSprint), "", token)
}

// finalAndNonFinalState returns one final and one non-final state from the
// project's seeded defaults.
func finalAndNonFinalState(t *testing.T, app *issue.Application, token string, idProject int64) (model.State, model.State) {
	states := loadProjectStates(t, app, token, idProject)
	var final, nonFinal *model.State
	for i := range states {
		if states[i].Final && final == nil {
			final = &states[i]
		}
		if !states[i].Final && nonFinal == nil {
			nonFinal = &states[i]
		}
	}
	require.NotNil(t, final, "project must have a final default state")
	require.NotNil(t, nonFinal, "project must have a non-final default state")
	return *final, *nonFinal
}

// createIssueWithPoints creates an issue in a given state with story points.
func createIssueWithPoints(t *testing.T, app *issue.Application, token string, idProject, idState int64, points int) int64 {
	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   idProject,
		IdState:     &idState,
		Title:       "pts issue",
		Description: "x",
		Points:      &points,
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created.IdIssuePublic
}

// registerViewer registers a fresh user, adds them to the project as viewer,
// and returns their token.
func registerViewer(t *testing.T, app *issue.Application, ownerToken string, idProject int64, email string) string {
	Request(t, app, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"v","email":"%s","password":"kreslo"}`, email), ownerToken)
	loginRes := Request(t, app, "POST", "/api/public/login",
		fmt.Sprintf(`{"email":"%s","password":"kreslo"}`, email), "")
	var tk struct{ Token string }
	require.Nil(t, json.NewDecoder(loginRes.Body).Decode(&tk))

	userRes := Request(t, app, "GET", "/api/private/user", "", tk.Token)
	var user model.User
	require.Nil(t, json.NewDecoder(userRes.Body).Decode(&user))

	Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"viewer"}`, user.IdUser), ownerToken)
	return tk.Token
}

// --- tests -----------------------------------------------------------------

func TestSprintApi_CreateAutoNames(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-auto")

	s1 := createSprint(t, app, token, idProject, `{}`)
	require.Equal(t, "Sprint 1", s1.Name)
	require.Equal(t, "planned", s1.State)

	s2 := createSprint(t, app, token, idProject, `{}`)
	require.Equal(t, "Sprint 2", s2.Name)
}

func TestSprintApi_CreateForbiddenForViewer(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-acl")
	viewerToken := registerViewer(t, app, token, idProject, "sprint-viewer@test.sk")

	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/sprint", idProject), `{}`, viewerToken)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestSprintApi_AssignRejectsForeignSprint(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "sprint-assign-a")
	idProjectB := createProject(t, app, token, "sprint-assign-b")

	statesA := loadProjectStates(t, app, token, idProjectA)
	require.NotEmpty(t, statesA)
	issueA := createIssueInState(t, app, token, idProjectA, statesA[0].IdState)
	sprintB := createSprint(t, app, token, idProjectB, `{}`)

	res := assignSprint(t, app, token, idProjectA, issueA, fmt.Sprintf(`{"idSprint":%d}`, sprintB.IdSprint))
	require.Equal(t, http.StatusBadRequest, res.StatusCode, "cross-project sprint assignment must be rejected")
}

func TestSprintApi_CloseRollsOverUnfinished(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-rollover")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)

	sprintA := createSprint(t, app, token, idProject, `{}`)
	sprintB := createSprint(t, app, token, idProject, `{}`) // next planned

	doneIssue := createIssueInState(t, app, token, idProject, final.IdState)
	openIssue := createIssueInState(t, app, token, idProject, nonFinal.IdState)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, doneIssue, fmt.Sprintf(`{"idSprint":%d}`, sprintA.IdSprint)).StatusCode)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, openIssue, fmt.Sprintf(`{"idSprint":%d}`, sprintA.IdSprint)).StatusCode)

	closeRes := closeSprint(t, app, token, sprintA.IdSprint)
	require.Equal(t, http.StatusOK, closeRes.StatusCode)
	var closeBody struct{ Moved int64 }
	require.Nil(t, json.NewDecoder(closeRes.Body).Decode(&closeBody))
	require.Equal(t, int64(1), closeBody.Moved, "only the unfinished issue rolls over")

	movedIssue := loadIssue(t, app, token, idProject, openIssue)
	require.Equal(t, sprintB.IdSprint, *movedIssue.IdSprint)
	require.Equal(t, 1, movedIssue.CarryoverCount, "rolled-over issue's carry-over count increments")
	require.Equal(t, sprintA.IdSprint, *loadIssue(t, app, token, idProject, doneIssue).IdSprint)
	require.Equal(t, 0, loadIssue(t, app, token, idProject, doneIssue).CarryoverCount, "finished issue is not carried over")
}

func TestSprintApi_DoubleCloseConflicts(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-doubleclose")
	_, nonFinal := finalAndNonFinalState(t, app, token, idProject)

	sprintA := createSprint(t, app, token, idProject, `{}`)
	sprintB := createSprint(t, app, token, idProject, `{}`)
	openIssue := createIssueInState(t, app, token, idProject, nonFinal.IdState)
	assignSprint(t, app, token, idProject, openIssue, fmt.Sprintf(`{"idSprint":%d}`, sprintA.IdSprint))

	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprintA.IdSprint).StatusCode)
	require.Equal(t, http.StatusConflict, closeSprint(t, app, token, sprintA.IdSprint).StatusCode)

	// The rollover must not have run twice: the issue sits in B, unchanged.
	require.Equal(t, sprintB.IdSprint, *loadIssue(t, app, token, idProject, openIssue).IdSprint)
}

func TestSprintApi_CloseLastSprintClearsToBacklog(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-last")
	_, nonFinal := finalAndNonFinalState(t, app, token, idProject)

	sprint := createSprint(t, app, token, idProject, `{}`)
	openIssue := createIssueInState(t, app, token, idProject, nonFinal.IdState)
	assignSprint(t, app, token, idProject, openIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint))

	closeRes := closeSprint(t, app, token, sprint.IdSprint)
	require.Equal(t, http.StatusOK, closeRes.StatusCode)
	var closeBody struct{ Moved int64 }
	require.Nil(t, json.NewDecoder(closeRes.Body).Decode(&closeBody))
	require.Equal(t, int64(1), closeBody.Moved)

	require.Nil(t, loadIssue(t, app, token, idProject, openIssue).IdSprint, "no next planned → cleared to Backlog")
}

func TestSprintApi_Edit(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-edit")
	sprint := createSprint(t, app, token, idProject, `{}`)

	body := `{"name":"Renamed","startAt":"2026-04-01T00:00:00Z","endAt":"2026-04-15T00:00:00Z"}`
	res := Request(t, app, "PATCH", fmt.Sprintf("/api/private/sprint/%d", sprint.IdSprint), body, token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var updated model.Sprint
	require.Nil(t, json.NewDecoder(res.Body).Decode(&updated))
	require.Equal(t, "Renamed", updated.Name)

	// Persisted: reload via the project list.
	listRes := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/sprint", idProject), "", token)
	var list []model.Sprint
	require.Nil(t, json.NewDecoder(listRes.Body).Decode(&list))
	require.Len(t, list, 1)
	require.Equal(t, "Renamed", list[0].Name)
}

func TestSprintApi_Delete(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-delete")
	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)

	sprint := createSprint(t, app, token, idProject, `{}`)
	issue := createIssueInState(t, app, token, idProject, states[0].IdState)
	assignSprint(t, app, token, idProject, issue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint))

	res := Request(t, app, "DELETE", fmt.Sprintf("/api/private/sprint/%d", sprint.IdSprint), "", token)
	require.Equal(t, http.StatusNoContent, res.StatusCode)

	// Sprint gone, and its issue fell back to the Backlog (id_sprint NULL).
	listRes := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/sprint", idProject), "", token)
	var list []model.Sprint
	require.Nil(t, json.NewDecoder(listRes.Body).Decode(&list))
	require.Empty(t, list)
	require.Nil(t, loadIssue(t, app, token, idProject, issue).IdSprint)
}

func TestSprintApi_EditForbiddenForViewer(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-edit-acl")
	sprint := createSprint(t, app, token, idProject, `{}`)
	viewerToken := registerViewer(t, app, token, idProject, "sprint-edit-viewer@test.sk")

	body := `{"name":"X","startAt":"2026-04-01T00:00:00Z","endAt":"2026-04-15T00:00:00Z"}`
	editRes := Request(t, app, "PATCH", fmt.Sprintf("/api/private/sprint/%d", sprint.IdSprint), body, viewerToken)
	require.Equal(t, http.StatusForbidden, editRes.StatusCode)

	delRes := Request(t, app, "DELETE", fmt.Sprintf("/api/private/sprint/%d", sprint.IdSprint), "", viewerToken)
	require.Equal(t, http.StatusForbidden, delRes.StatusCode)
}

func TestSprintApi_Stats(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-stats")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)

	sprint := createSprint(t, app, token, idProject, `{}`)
	doneIssue := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	openIssue := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	assignSprint(t, app, token, idProject, doneIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint))
	assignSprint(t, app, token, idProject, openIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint))

	res := Request(t, app, "GET", fmt.Sprintf("/api/private/sprint/%d/stats", sprint.IdSprint), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var stats model.SprintStats
	require.Nil(t, json.NewDecoder(res.Body).Decode(&stats))
	require.Equal(t, 8, stats.TotalPoints)
	require.Equal(t, 3, stats.DonePoints, "velocity = points in a final state")
	require.Equal(t, 2, stats.TotalIssues)
	require.Equal(t, 1, stats.DoneIssues)
}
