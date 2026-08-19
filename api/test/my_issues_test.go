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

func currentUser(t *testing.T, app *issue.Application, token string) model.User {
	res := Request(t, app, "GET", "/api/private/user", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var user model.User
	require.Nil(t, json.NewDecoder(res.Body).Decode(&user))
	return user
}

func createAssignedIssue(t *testing.T, app *issue.Application, token string, idProject, idState, idUser int64, title string) int64 {
	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   idProject,
		IdState:     &idState,
		Title:       title,
		Description: "x",
		AssignedTo:  &idUser,
	})
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject), string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created.IdIssuePublic
}

func TestMyIssues_ReturnsOpenAssignedIssuesOnly(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "my-issues-open")
	me := currentUser(t, app, token)
	final, nonFinal := finalAndNonFinalState(t, app, token, pid)

	open := createAssignedIssue(t, app, token, pid, nonFinal.IdState, me.IdUser, "open one")
	done := createAssignedIssue(t, app, token, pid, final.IdState, me.IdUser, "closed one")

	res := Request(t, app, "GET", "/api/private/my-issues?limit=50", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var issues []*model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&issues))

	require.NotNil(t, findIssue(issues, open), "issue in a non-final state must be listed")
	require.Nil(t, findIssue(issues, done), "issue in a final state must be excluded")
}

func TestMyIssues_ExcludesIssuesAssignedToSomeoneElse(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "my-issues-others")
	me := currentUser(t, app, token)
	_, nonFinal := finalAndNonFinalState(t, app, token, pid)

	otherToken := registerViewer(t, app, token, pid, "my-issues-other@rurdesk.test")
	other := currentUser(t, app, otherToken)

	mine := createAssignedIssue(t, app, token, pid, nonFinal.IdState, me.IdUser, "mine")
	theirs := createAssignedIssue(t, app, token, pid, nonFinal.IdState, other.IdUser, "theirs")

	res := Request(t, app, "GET", "/api/private/my-issues", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var issues []*model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&issues))

	require.NotNil(t, findIssue(issues, mine))
	require.Nil(t, findIssue(issues, theirs))
}

func TestIssueList_ExcludeFinalStatesFilter(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	pid := createProject(t, app, token, "exclude-final-filter")
	me := currentUser(t, app, token)
	final, nonFinal := finalAndNonFinalState(t, app, token, pid)

	open := createAssignedIssue(t, app, token, pid, nonFinal.IdState, me.IdUser, "open")
	done := createAssignedIssue(t, app, token, pid, final.IdState, me.IdUser, "done")

	page := loadIssuesEnvelope(t, app, token, pid, "?excludeFinalStates=true")
	require.NotNil(t, findIssue(page.Items, open))
	require.Nil(t, findIssue(page.Items, done))
}
