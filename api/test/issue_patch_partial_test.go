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

func patchIssue(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64, body string) *http.Response {
	return Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, idIssuePublic), body, token)
}

func createGitIntegration(t *testing.T, app *issue.Application, token string, idProject int64, name, repoPath string) int64 {
	body := fmt.Sprintf(
		`{"name":%q,"hostType":"github","baseUrl":"https://github.com","repoPath":%q,"accessToken":"ghp_test_token"}`,
		name, repoPath)
	res := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", idProject), body, token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var integration model.GitIntegrationRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&integration))
	return integration.IdGitIntegration
}

func seedFullyPopulatedIssue(t *testing.T, app *issue.Application, token string, idProject int64, name string) (int64, int64) {
	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)
	severities := loadProjectSeverities(t, app, token, idProject)
	require.NotEmpty(t, severities)

	idIssuePublic := createIssueInState(t, app, token, idProject, states[0].IdState)
	idGitIntegration := createGitIntegration(t, app, token, idProject, name, "org/"+name)
	idUser := idOfUser(t, app, token, "test@test.sk")

	full := fmt.Sprintf(
		`{"title":"seeded","description":"seeded desc","idState":%d,"idSeverity":%d,"assignedTo":%d,`+
			`"estimated":3600,"points":8,"scheduledAt":"2026-09-01T10:00:00Z",`+
			`"idGitIntegration":%d,"mrId":"42"}`,
		states[0].IdState, severities[0].IdSeverity, idUser, idGitIntegration)
	res := patchIssue(t, app, token, idProject, idIssuePublic, full)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	return idIssuePublic, idGitIntegration
}

func TestEditIssue_SparsePatch_PreservesUnsentFields(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-preserve")

	idIssuePublic, idGitIntegration := seedFullyPopulatedIssue(t, app, token, idProject, "preserve")
	before := loadIssue(t, app, token, idProject, idIssuePublic)

	res := patchIssue(t, app, token, idProject, idIssuePublic, `{"title":"renamed"}`)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	after := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Equal(t, "renamed", after.Title)
	require.Equal(t, before.Description, after.Description)
	require.Equal(t, before.IdState, after.IdState)
	require.Equal(t, before.IdSeverity, after.IdSeverity)
	require.Equal(t, before.AssignedTo, after.AssignedTo)
	require.Equal(t, before.Estimated, after.Estimated)
	require.Equal(t, before.Points, after.Points)
	require.Equal(t, before.ScheduledAt, after.ScheduledAt)

	require.NotNil(t, after.IdGitIntegration, "MR link must survive a sparse edit")
	require.Equal(t, idGitIntegration, *after.IdGitIntegration)
	require.NotNil(t, after.MrId)
	require.Equal(t, "42", *after.MrId)
}

func TestEditIssue_RepeatedSparsePatches_KeepMrLink(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-repeat")

	idIssuePublic, idGitIntegration := seedFullyPopulatedIssue(t, app, token, idProject, "repeat")

	for i := range 3 {
		res := patchIssue(t, app, token, idProject, idIssuePublic,
			fmt.Sprintf(`{"description":"edit %d"}`, i))
		require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	}

	after := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Equal(t, "edit 2", after.Description)
	require.NotNil(t, after.IdGitIntegration)
	require.Equal(t, idGitIntegration, *after.IdGitIntegration)
	require.NotNil(t, after.MrId)
	require.Equal(t, "42", *after.MrId)
}

func TestEditIssue_ExplicitNull_ClearsField(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-null")

	idIssuePublic, _ := seedFullyPopulatedIssue(t, app, token, idProject, "clearnull")

	res := patchIssue(t, app, token, idProject, idIssuePublic,
		`{"points":null,"assignedTo":null,"scheduledAt":null}`)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	after := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Nil(t, after.Points)
	require.Nil(t, after.AssignedTo)
	require.Nil(t, after.ScheduledAt)
	require.Equal(t, "seeded", after.Title, "an unrelated field must not move")
}

func TestEditIssue_ExplicitNull_ClearsBothMrLinkHalves(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-unlink")

	idIssuePublic, _ := seedFullyPopulatedIssue(t, app, token, idProject, "unlink")

	res := patchIssue(t, app, token, idProject, idIssuePublic,
		`{"idGitIntegration":null,"mrId":null}`)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	after := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Nil(t, after.IdGitIntegration)
	require.Nil(t, after.MrId)
}

func TestEditIssue_ClearingHalfOfMrLink_Rejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-halflink")

	idIssuePublic, idGitIntegration := seedFullyPopulatedIssue(t, app, token, idProject, "halflink")

	res := patchIssue(t, app, token, idProject, idIssuePublic, `{"mrId":null}`)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	after := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, after.IdGitIntegration)
	require.Equal(t, idGitIntegration, *after.IdGitIntegration)
	require.NotNil(t, after.MrId)
}

func TestEditIssue_OmittedTitle_Accepted(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-notitle")

	idIssuePublic, _ := seedFullyPopulatedIssue(t, app, token, idProject, "notitle")

	res := patchIssue(t, app, token, idProject, idIssuePublic, `{"description":"only the description"}`)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))

	after := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Equal(t, "only the description", after.Description)
	require.Equal(t, "seeded", after.Title)
}

func TestEditIssue_SentTitle_StillValidated(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "patch-partial-titleval")

	states := loadProjectStates(t, app, token, idProject)
	require.NotEmpty(t, states)
	idIssuePublic := createIssueInState(t, app, token, idProject, states[0].IdState)

	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{name: "empty title", body: `{"title":""}`, wantMessage: "title is required"},
		{name: "null title", body: `{"title":null}`, wantMessage: "title is required"},
		{name: "title over 100 chars", body: fmt.Sprintf(`{"title":%q}`, repeatRune('a', 101)), wantMessage: "title exceeds 100 characters"},
		{name: "empty description", body: `{"description":""}`, wantMessage: "description is required"},
		{name: "negative points", body: `{"points":-1}`, wantMessage: "points must be zero or greater"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := patchIssue(t, app, token, idProject, idIssuePublic, tc.body)
			require.Equal(t, http.StatusBadRequest, res.StatusCode)

			var body struct {
				Code         string `json:"code"`
				Message      string `json:"message"`
				TranslateKey string `json:"translateKey"`
			}
			require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
			require.Equal(t, "VALIDATION_FAILED", body.Code)
			require.Equal(t, "error.validation", body.TranslateKey)
			require.Equal(t, tc.wantMessage, body.Message)
		})
	}
}

func repeatRune(r rune, count int) string {
	out := make([]rune, count)
	for i := range out {
		out[i] = r
	}
	return string(out)
}
