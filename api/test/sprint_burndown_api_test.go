package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func loadBurndown(t *testing.T, app *issue.Application, token string, idSprint int64) model.SprintBurndown {
	res := Request(t, app, "GET", fmt.Sprintf("/api/private/sprint/%d/burndown", idSprint), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var burndown model.SprintBurndown
	require.Nil(t, json.NewDecoder(res.Body).Decode(&burndown))
	return burndown
}

func loadSprintStats(t *testing.T, app *issue.Application, token string, idSprint int64) model.SprintStats {
	res := Request(t, app, "GET", fmt.Sprintf("/api/private/sprint/%d/stats", idSprint), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var stats model.SprintStats
	require.Nil(t, json.NewDecoder(res.Body).Decode(&stats))
	return stats
}

func TestSprintBurndown_RecordsTodayForARunningSprint(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-running")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Running", -2, 12))

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	open := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	for _, idIssuePublic := range []int64{done, open} {
		require.Equal(t, http.StatusNoContent,
			assignSprint(t, app, token, idProject, idIssuePublic, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	}

	burndown := loadBurndown(t, app, token, sprint.IdSprint)
	require.Equal(t, sprint.IdSprint, burndown.IdSprint)
	require.NotEmpty(t, burndown.Days, "reading the burndown records today's snapshot")

	var recorded *model.SprintBurndownDay
	for i := range burndown.Days {
		if burndown.Days[i].Snapshot {
			recorded = &burndown.Days[i]
		}
	}
	require.NotNil(t, recorded, "today is a real recorded day")
	require.Equal(t, 8, *recorded.TotalPoints)
	require.Equal(t, 3, *recorded.DonePoints)
	require.Equal(t, 5, *recorded.RemainingPoints)
	require.Equal(t, 2, *recorded.TotalIssues)
	require.Equal(t, 1, *recorded.RemainingIssues)
}

func TestSprintBurndown_SprintWithoutHistoryReturnsAnEmptyArray(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-empty")
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Future", 4, 18))

	res := Request(t, app, "GET", fmt.Sprintf("/api/private/sprint/%d/burndown", sprint.IdSprint), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"days":[]`, "an empty history must not marshal to null")
}

func TestSprintBurndown_UnknownSprintAndNonMember(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-acl")
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Guarded", -1, 13))
	outsiderToken := createUserAsAdmin(t, app, token,
		`{"name":"o","email":"burndown-outsider@test.sk","password":"kreslo"}`)

	missing := Request(t, app, "GET", "/api/private/sprint/999999/burndown", "", token)
	require.Equal(t, http.StatusNotFound, missing.StatusCode)

	forbidden := Request(t, app, "GET",
		fmt.Sprintf("/api/private/sprint/%d/burndown", sprint.IdSprint), "", outsiderToken)
	require.Equal(t, http.StatusForbidden, forbidden.StatusCode)
}

func TestSprintBurndown_CloseFreezesTheScopeBeforeTheRollover(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-freeze")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Closing", -6, 8))

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	open := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	for _, idIssuePublic := range []int64{done, open} {
		require.Equal(t, http.StatusNoContent,
			assignSprint(t, app, token, idProject, idIssuePublic, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	}
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)

	stats := loadSprintStats(t, app, token, sprint.IdSprint)
	require.Equal(t, 3, stats.TotalPoints, "live totals collapse to done once the rollover has run")
	require.NotNil(t, stats.FrozenTotalPoints)
	require.Equal(t, 8, *stats.FrozenTotalPoints, "the frozen row still holds the rolled-over issue's points")
	require.NotNil(t, stats.FrozenDonePoints)
	require.Equal(t, 3, *stats.FrozenDonePoints)
	require.NotNil(t, stats.RolledOverIssues)
	require.Equal(t, 1, *stats.RolledOverIssues)
}

func TestSprintBurndown_FrozenNumbersSurviveAPostCloseChange(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-postclose")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Closing", -6, 8))

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	open := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	for _, idIssuePublic := range []int64{done, open} {
		require.Equal(t, http.StatusNoContent,
			assignSprint(t, app, token, idProject, idIssuePublic, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	}
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)

	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, done, `{"idSprint":null}`).StatusCode)

	stats := loadSprintStats(t, app, token, sprint.IdSprint)
	require.NotNil(t, stats.FrozenTotalPoints)
	require.Equal(t, 8, *stats.FrozenTotalPoints, "dragging a task out of a closed cycle cannot rewrite its history")
	require.NotNil(t, stats.RolledOverIssues)
	require.Equal(t, 1, *stats.RolledOverIssues)
}

func TestSprintBurndown_DoubleCloseDoesNotRewriteTheSnapshot(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-doubleclose")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Closing", -6, 8))

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	open := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	for _, idIssuePublic := range []int64{done, open} {
		require.Equal(t, http.StatusNoContent,
			assignSprint(t, app, token, idProject, idIssuePublic, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	}
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)
	require.Equal(t, http.StatusConflict, closeSprint(t, app, token, sprint.IdSprint).StatusCode)

	stats := loadSprintStats(t, app, token, sprint.IdSprint)
	require.NotNil(t, stats.FrozenTotalPoints)
	require.Equal(t, 8, *stats.FrozenTotalPoints)
}

func TestSprintBurndown_ClosedSprintReadDoesNotRecordANewDay(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "burndown-closed-read")
	final, _ := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Closed", -6, 8))

	idIssue := createIssueWithPoints(t, app, token, idProject, final.IdState, 4)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, idIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)

	before := loadBurndown(t, app, token, sprint.IdSprint)
	recorded := 0
	for _, day := range before.Days {
		if day.Snapshot {
			recorded++
		}
	}
	require.Equal(t, 1, recorded, "the close day is the only recorded day")

	after := loadBurndown(t, app, token, sprint.IdSprint)
	require.Equal(t, len(before.Days), len(after.Days), "reading a closed cycle records nothing")
}
