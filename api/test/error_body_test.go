package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tests verifying that error responses carry a non-empty body with a "code"
// field, as required by the canonical c.Error + c.Status error path. The
// ErrorRenderer middleware renders {code, message, translateKey} when a typed
// error is attached; without it the client gets an empty body and falls back
// to a generic "internal error" toast.

// --- sprint ---------------------------------------------------------------

func TestSprintApi_List_NonNumericProject_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/project/abc/sprint", "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

func TestSprintApi_List_OutsiderForbidden_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-acl")
	outsider := createUserAsAdmin(t, app, token,
		`{"name":"sprint-outsider","email":"sprint-outsider@test.sk","password":"kreslo"}`)
	res := Request(t, app, "GET", fmt.Sprintf("/api/private/project/%d/sprint", idProject), "", outsider)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

func TestSprintApi_Velocity_BadLimit_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sprint-velocity")
	res := Request(t, app, "GET",
		fmt.Sprintf("/api/private/project/%d/sprint/velocity?limit=999999", idProject), "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

// --- skill -----------------------------------------------------------------

func TestSkillApi_Get_NonNumericId_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/admin/skills/abc", "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

// --- saved view -----------------------------------------------------------

func TestSavedViewApi_List_NonNumericProject_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/project/abc/saved-view", "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

func TestSavedViewApi_Create_EmptyName_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "sv-empty-name")
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/saved-view", idProject),
		`{"name":"  ","viewType":"table","config":{"v":1},"isShared":false}`, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

// --- agent run ------------------------------------------------------------

func TestAgentRun_GetRun_NonNumericId_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/agent/run/abc", "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

func TestAgentRun_GetRun_NotFound_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/agent/run/999999999", "", token)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

func TestAgentRun_Cancel_NotFound_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "POST", "/api/private/agent/run/999999999/cancel", "", token)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

// --- agent overview -------------------------------------------------------

func TestAgentOverview_NonNumericProject_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/project/abc/agents/overview", "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}

// --- workflow event map ---------------------------------------------------

func TestWorkflowEventMap_GetMappings_NonNumericProject_HasErrorBody(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	res := Request(t, app, "GET", "/api/private/project/abc/workflow-event-state-map", "", token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	require.NotEmpty(t, errorCode(t, res))
}
