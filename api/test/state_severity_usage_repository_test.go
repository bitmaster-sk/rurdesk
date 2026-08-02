package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"

	"github.com/stretchr/testify/require"
)

func TestStateRepository_LoadStateUsage(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "usage-state")
	states := loadProjectStates(t, app, token, idProject)
	target := states[0]

	createIssueInState(t, app, token, idProject, target.IdState)
	createIssueInState(t, app, token, idProject, target.IdState)

	usage, err := injector.GetStateRepository().LoadStateUsage(context.Background(), idProject, target.IdState)
	require.Nil(t, err)
	require.Equal(t, 2, usage.Issues)
}

func TestStateRepository_UsageCountsProjectDefault(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "usage-default")

	// PATCH /api/private/project has no :idProject segment and requires name too
	// (router.go:129, model/project.go:17-24)
	states := loadProjectStates(t, app, token, idProject)
	target := states[0]
	res := Request(t, app, "PATCH", "/api/private/project",
		fmt.Sprintf(`{"idProject":%d,"name":"usage-default","idStateDefault":%d}`, idProject, target.IdState), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	usage, err := injector.GetStateRepository().LoadStateUsage(context.Background(), idProject, target.IdState)
	require.Nil(t, err)
	require.True(t, usage.IsProjectDefault)
}

// no API path maps a non-protected row into a second project — insert directly;
// this pins the count, Task 4's TestDeleteState_SharedMappingRowSurvives covers the delete
func TestStateRepository_MappingsCountsAllProjects(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "usage-multimap-a")
	idProjectB := createProject(t, app, token, "usage-multimap-b")
	idState := createStateInProject(t, app, token, idProjectA, "shared-local")

	// second mapping inserted directly (no API path exists for this)
	pool, err := injector.GetDb()
	require.Nil(t, err)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO projects.project_issue_state (id_project, id_state, order_rank) VALUES ($1, $2, 99)`,
		idProjectB, idState)
	require.Nil(t, err)

	usage, err := injector.GetStateRepository().LoadStateUsage(context.Background(), idProjectA, idState)
	require.Nil(t, err)
	require.Equal(t, 2, usage.Mappings)
}

func TestSeverityRepository_LoadSeverityUsage(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "usage-severity")
	severities := loadProjectSeverities(t, app, token, idProject)
	target := severities[0]

	createIssueInSeverity(t, app, token, idProject, target.IdSeverity)
	createIssueInSeverity(t, app, token, idProject, target.IdSeverity)

	usage, err := injector.GetSeverityRepository().LoadSeverityUsage(context.Background(), idProject, target.IdSeverity)
	require.Nil(t, err)
	require.Equal(t, 2, usage.Issues)
}

func TestSeverityRepository_UsageCountsProjectDefault(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "usage-severity-default")

	// PATCH /api/private/project has no :idProject segment and requires name too
	// (router.go:129, model/project.go:17-24)
	severities := loadProjectSeverities(t, app, token, idProject)
	target := severities[0]
	res := Request(t, app, "PATCH", "/api/private/project",
		fmt.Sprintf(`{"idProject":%d,"name":"usage-severity-default","idSeverityDefault":%d}`, idProject, target.IdSeverity), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	usage, err := injector.GetSeverityRepository().LoadSeverityUsage(context.Background(), idProject, target.IdSeverity)
	require.Nil(t, err)
	require.True(t, usage.IsProjectDefault)
}

// no API path maps a non-protected row into a second project — insert directly;
// mirrors TestStateRepository_MappingsCountsAllProjects
func TestSeverityRepository_MappingsCountsAllProjects(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProjectA := createProject(t, app, token, "usage-sev-multimap-a")
	idProjectB := createProject(t, app, token, "usage-sev-multimap-b")
	idSeverity := createSeverityInProject(t, app, token, idProjectA, "shared-local")

	// second mapping inserted directly (no API path exists for this)
	pool, err := injector.GetDb()
	require.Nil(t, err)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO projects.project_issue_severity (id_project, id_severity, order_rank) VALUES ($1, $2, 99)`,
		idProjectB, idSeverity)
	require.Nil(t, err)

	usage, err := injector.GetSeverityRepository().LoadSeverityUsage(context.Background(), idProjectA, idSeverity)
	require.Nil(t, err)
	require.Equal(t, 2, usage.Mappings)
}
