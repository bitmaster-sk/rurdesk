package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"

	"github.com/stretchr/testify/require"
)

func stateUsageURL(idState, idProject int64) string {
	return fmt.Sprintf("/api/private/state/%d/project/%d/usage", idState, idProject)
}

func stateDeleteURL(idState, idProject int64, intent string) string {
	u := fmt.Sprintf("/api/private/state/%d/project/%d", idState, idProject)
	if intent != "" {
		u += "?" + intent
	}
	return u
}

func severityUsageURL(idSeverity, idProject int64) string {
	return fmt.Sprintf("/api/private/severity/%d/project/%d/usage", idSeverity, idProject)
}

func severityDeleteURL(idSeverity, idProject int64, intent string) string {
	u := fmt.Sprintf("/api/private/severity/%d/project/%d", idSeverity, idProject)
	if intent != "" {
		u += "?" + intent
	}
	return u
}

func TestStateUsage_ReportsIssueCount(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-usage")
	states := loadProjectStates(t, app, token, idProject)
	target := states[0]
	createIssueInState(t, app, token, idProject, target.IdState)

	res := Request(t, app, "GET", stateUsageURL(target.IdState, idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	require.Nil(t, err)

	var usage model.StateUsage
	require.Nil(t, json.Unmarshal(body, &usage))
	require.Equal(t, 1, usage.Issues)

	// pin the wire field names (camelCase) and the absence of a "mappings" field —
	// this is the frontend's whole contract for hasDeleteUsage()/the fast delete path.
	var raw map[string]any
	require.Nil(t, json.Unmarshal(body, &raw))
	require.Contains(t, raw, "issues")
	require.Contains(t, raw, "isProjectDefault")
	require.Contains(t, raw, "agentPhases")
	require.NotContains(t, raw, "mappings")
}

func TestDeleteState_WithIssuesAndNoIntent_Conflicts(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-conflict")
	states := loadProjectStates(t, app, token, idProject)
	target := states[0]
	idIssuePublic := createIssueInState(t, app, token, idProject, target.IdState)

	res := Request(t, app, "DELETE", stateDeleteURL(target.IdState, idProject, ""), "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
	var body struct {
		Code string `json:"code"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Equal(t, "STATE_IN_USE", body.Code)

	// nothing changed
	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, issue.IdState)
}

func TestDeleteState_MigratesIssuesToTarget(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-migrate")
	states := loadProjectStates(t, app, token, idProject)
	from, to := states[0], states[1]
	idIssuePublic := createIssueInState(t, app, token, idProject, from.IdState)

	res := Request(t, app, "DELETE",
		stateDeleteURL(from.IdState, idProject, fmt.Sprintf("migrateTo=%d", to.IdState)), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, issue.IdState)
	require.Equal(t, to.IdState, *issue.IdState)

	mapped := idStateSet(loadProjectStates(t, app, token, idProject))
	require.NotContains(t, mapped, from.IdState)
}

func TestDeleteState_ExplicitUnassign(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-unassign")
	idLocal := createStateInProject(t, app, token, idProject, "unassign-local")
	idIssuePublic := createIssueInState(t, app, token, idProject, idLocal)

	res := Request(t, app, "DELETE", stateDeleteURL(idLocal, idProject, "migrateTo=null"), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Nil(t, issue.IdState)
}

func TestDeleteState_CrossProjectTargetRejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "mig-cross-a")
	idProjectB := createProject(t, app, token, "mig-cross-b")
	statesA := loadProjectStates(t, app, token, idProjectA)
	target := statesA[0]
	createIssueInState(t, app, token, idProjectA, target.IdState)
	// a project-local (non-protected) state in B only — helper returns int64
	idForeign := createStateInProject(t, app, token, idProjectB, "b-only")

	res := Request(t, app, "DELETE",
		stateDeleteURL(target.IdState, idProjectA, fmt.Sprintf("migrateTo=%d", idForeign)), "", token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	var body struct {
		Code string `json:"code"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Equal(t, "INVALID_MIGRATION_TARGET", body.Code)
}

func TestDeleteState_ZeroIssues_NoIntentNeeded(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-zero")
	idLocal := createStateInProject(t, app, token, idProject, "empty-local")

	res := Request(t, app, "DELETE", stateDeleteURL(idLocal, idProject, ""), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

// zero issues but the state is the project default — bare DELETE must 409
func TestDeleteState_DefaultOnlyUsage_Conflicts(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-default-only")
	idLocal := createStateInProject(t, app, token, idProject, "default-local")
	res := Request(t, app, "PATCH", "/api/private/project",
		fmt.Sprintf(`{"idProject":%d,"name":"mig-default-only","idStateDefault":%d}`, idProject, idLocal), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "DELETE", stateDeleteURL(idLocal, idProject, ""), "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
}

// explicit unassign on a zero-issue default must succeed and null the default
func TestDeleteState_UnassignZeroIssueDefault_NullsDefault(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-default-unassign")
	idLocal := createStateInProject(t, app, token, idProject, "default-local-2")
	res := Request(t, app, "PATCH", "/api/private/project",
		fmt.Sprintf(`{"idProject":%d,"name":"mig-default-unassign","idStateDefault":%d}`, idProject, idLocal), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "DELETE", stateDeleteURL(idLocal, idProject, "migrateTo=null"), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	// assert via direct SQL — no assumption about the project GET payload
	pool, err := injector.GetDb()
	require.Nil(t, err)
	var idDefault *int64
	require.Nil(t, pool.QueryRow(context.Background(),
		`SELECT id_state_default FROM projects.project WHERE id_project = $1`, idProject).Scan(&idDefault))
	require.Nil(t, idDefault)
}

// zero issues but an agent phase maps to the state (no API path — insert directly)
func TestDeleteState_AgentPhaseOnlyUsage_Conflicts(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-phase-only")
	idLocal := createStateInProject(t, app, token, idProject, "phase-local")

	pool, err := injector.GetDb()
	require.Nil(t, err)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO projects.agent_phase_state_map (id_project, phase, id_state) VALUES ($1, 'implementing', $2)`,
		idProject, idLocal)
	require.Nil(t, err)

	res := Request(t, app, "DELETE", stateDeleteURL(idLocal, idProject, ""), "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
}

// the shared row and the other project's mapping must survive the delete
func TestDeleteState_SharedMappingRowSurvives(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "mig-shared-a")
	idProjectB := createProject(t, app, token, "mig-shared-b")
	idState := createStateInProject(t, app, token, idProjectA, "shared-local-2")

	pool, err := injector.GetDb()
	require.Nil(t, err)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO projects.project_issue_state (id_project, id_state, order_rank) VALUES ($1, $2, 99)`,
		idProjectB, idState)
	require.Nil(t, err)

	res := Request(t, app, "DELETE", stateDeleteURL(idState, idProjectA, "migrateTo=null"), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var rowCount, mapCount int
	require.Nil(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM issues.state WHERE id_state = $1`, idState).Scan(&rowCount))
	require.Nil(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM projects.project_issue_state WHERE id_project = $1 AND id_state = $2`,
		idProjectB, idState).Scan(&mapCount))
	require.Equal(t, 1, rowCount, "shared state row must survive")
	require.Equal(t, 1, mapCount, "other project's mapping must survive")
}

// migrateTo that is neither an id nor the literal null → 400 (empty included)
func TestDeleteState_MalformedMigrateTo_BadRequest(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-badvalue")
	idLocal := createStateInProject(t, app, token, idProject, "badvalue-local")

	for _, intent := range []string{"migrateTo=undefined", "migrateTo="} {
		res := Request(t, app, "DELETE", stateDeleteURL(idLocal, idProject, intent), "", token)
		require.Equal(t, http.StatusBadRequest, res.StatusCode, intent)
	}
}

// DELETE a state id that exists but is not mapped to this project → 404, not 500.
func TestDeleteState_NotMappedToProject_NotFound(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "mig-notmapped-a")
	idProjectB := createProject(t, app, token, "mig-notmapped-b")
	idForeign := createStateInProject(t, app, token, idProjectB, "notmapped-b-only")

	res := Request(t, app, "DELETE", stateDeleteURL(idForeign, idProjectA, ""), "", token)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

// ---- Severity mirrors ----

func TestSeverityUsage_ReportsIssueCount(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-usage")
	severities := loadProjectSeverities(t, app, token, idProject)
	target := severities[0]
	createIssueInSeverity(t, app, token, idProject, target.IdSeverity)

	res := Request(t, app, "GET", severityUsageURL(target.IdSeverity, idProject), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var usage model.SeverityUsage
	require.Nil(t, json.NewDecoder(res.Body).Decode(&usage))
	require.Equal(t, 1, usage.Issues)
}

func TestDeleteSeverity_WithIssuesAndNoIntent_Conflicts(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-conflict")
	severities := loadProjectSeverities(t, app, token, idProject)
	target := severities[0]
	idIssuePublic := createIssueInSeverity(t, app, token, idProject, target.IdSeverity)

	res := Request(t, app, "DELETE", severityDeleteURL(target.IdSeverity, idProject, ""), "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
	var body struct {
		Code string `json:"code"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Equal(t, "SEVERITY_IN_USE", body.Code)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, issue.IdSeverity)
}

func TestDeleteSeverity_MigratesIssuesToTarget(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-migrate")
	severities := loadProjectSeverities(t, app, token, idProject)
	from, to := severities[0], severities[1]
	idIssuePublic := createIssueInSeverity(t, app, token, idProject, from.IdSeverity)

	res := Request(t, app, "DELETE",
		severityDeleteURL(from.IdSeverity, idProject, fmt.Sprintf("migrateTo=%d", to.IdSeverity)), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.NotNil(t, issue.IdSeverity)
	require.Equal(t, to.IdSeverity, *issue.IdSeverity)
}

func TestDeleteSeverity_ExplicitUnassign(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-unassign")
	idLocal := createSeverityInProject(t, app, token, idProject, "unassign-local")
	idIssuePublic := createIssueInSeverity(t, app, token, idProject, idLocal)

	res := Request(t, app, "DELETE", severityDeleteURL(idLocal, idProject, "migrateTo=null"), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	issue := loadIssue(t, app, token, idProject, idIssuePublic)
	require.Nil(t, issue.IdSeverity)
}

func TestDeleteSeverity_CrossProjectTargetRejected(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "mig-sev-cross-a")
	idProjectB := createProject(t, app, token, "mig-sev-cross-b")
	severitiesA := loadProjectSeverities(t, app, token, idProjectA)
	target := severitiesA[0]
	createIssueInSeverity(t, app, token, idProjectA, target.IdSeverity)
	idForeign := createSeverityInProject(t, app, token, idProjectB, "b-only")

	res := Request(t, app, "DELETE",
		severityDeleteURL(target.IdSeverity, idProjectA, fmt.Sprintf("migrateTo=%d", idForeign)), "", token)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	var body struct {
		Code string `json:"code"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Equal(t, "INVALID_MIGRATION_TARGET", body.Code)
}

// zero issues but the severity is the project default — bare DELETE must 409
func TestDeleteSeverity_DefaultOnlyUsage_Conflicts(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-default-only")
	idLocal := createSeverityInProject(t, app, token, idProject, "default-local")
	res := Request(t, app, "PATCH", "/api/private/project",
		fmt.Sprintf(`{"idProject":%d,"name":"mig-sev-default-only","idSeverityDefault":%d}`, idProject, idLocal), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "DELETE", severityDeleteURL(idLocal, idProject, ""), "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
}

// explicit unassign on a zero-issue default must succeed and null the default
func TestDeleteSeverity_UnassignZeroIssueDefault_NullsDefault(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-default-unassign")
	idLocal := createSeverityInProject(t, app, token, idProject, "default-local-2")
	res := Request(t, app, "PATCH", "/api/private/project",
		fmt.Sprintf(`{"idProject":%d,"name":"mig-sev-default-unassign","idSeverityDefault":%d}`, idProject, idLocal), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "DELETE", severityDeleteURL(idLocal, idProject, "migrateTo=null"), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	pool, err := injector.GetDb()
	require.Nil(t, err)
	var idDefault *int64
	require.Nil(t, pool.QueryRow(context.Background(),
		`SELECT id_severity_default FROM projects.project WHERE id_project = $1`, idProject).Scan(&idDefault))
	require.Nil(t, idDefault)
}

func TestDeleteSeverity_MalformedMigrateTo_BadRequest(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "mig-sev-badvalue")
	idLocal := createSeverityInProject(t, app, token, idProject, "badvalue-local")

	for _, intent := range []string{"migrateTo=undefined", "migrateTo="} {
		res := Request(t, app, "DELETE", severityDeleteURL(idLocal, idProject, intent), "", token)
		require.Equal(t, http.StatusBadRequest, res.StatusCode, intent)
	}
}
