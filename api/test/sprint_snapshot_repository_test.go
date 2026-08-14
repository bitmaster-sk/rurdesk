package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/require"
)

func sprintWindowBody(name string, startOffset, endOffset int) string {
	start := time.Now().UTC().AddDate(0, 0, startOffset).Format("2006-01-02T00:00:00Z")
	end := time.Now().UTC().AddDate(0, 0, endOffset).Format("2006-01-02T00:00:00Z")
	return fmt.Sprintf(`{"name":"%s","startAt":"%s","endAt":"%s"}`, name, start, end)
}

func clearSnapshots(t *testing.T, app *issue.Application, idSprint int64) {
	_, err := app.Pool.Exec(context.Background(),
		`DELETE FROM issues.sprint_snapshot WHERE id_sprint = $1`, idSprint)
	require.NoError(t, err)
}

func insertSnapshot(t *testing.T, app *issue.Application, idSprint int64, dayOffset int, totalPoints, donePoints, totalIssues, doneIssues, pointedIssues int) {
	day := time.Now().UTC().AddDate(0, 0, dayOffset).Format("2006-01-02")
	_, err := app.Pool.Exec(context.Background(), `
		INSERT INTO issues.sprint_snapshot(
			id_sprint, day, total_points, done_points, total_issues, done_issues, pointed_issues)
		VALUES ($1, $2::date, $3, $4, $5, $6, $7)`,
		idSprint, day, totalPoints, donePoints, totalIssues, doneIssues, pointedIssues)
	require.NoError(t, err)
}

func finalStateIds(t *testing.T, app *issue.Application, token string, idProject int64) []int64 {
	states := loadProjectStates(t, app, token, idProject)
	ids := make([]int64, 0, len(states))
	for i := range states {
		if states[i].Final {
			ids = append(ids, states[i].IdState)
		}
	}
	require.NotEmpty(t, ids)
	return ids
}

func TestSprintSnapshot_UpsertKeepsOneRowPerDay(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "snap-upsert")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Running", -3, 11))

	repo := injector.GetSprintRepository()
	ctx := context.Background()
	idsFinal := finalStateIds(t, app, token, idProject)

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, done, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	require.NoError(t, repo.UpsertSnapshotToday(ctx, sprint.IdSprint, idsFinal))

	open := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, open, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	require.NoError(t, repo.UpsertSnapshotToday(ctx, sprint.IdSprint, idsFinal))

	snaps, err := repo.LoadSnapshots(ctx, sprint.IdSprint)
	require.NoError(t, err)
	require.Len(t, snaps, 1, "a second upsert on the same day updates, never appends")
	require.Equal(t, 8, snaps[0].TotalPoints)
	require.Equal(t, 3, snaps[0].DonePoints)
	require.Equal(t, 2, snaps[0].TotalIssues)
	require.Equal(t, 1, snaps[0].DoneIssues)
	require.Equal(t, 2, snaps[0].PointedIssues)
}

func TestSprintSnapshot_UpsertSkipsSprintsThatHaveNotStarted(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "snap-future")
	final, _ := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Future", 3, 17))

	repo := injector.GetSprintRepository()
	ctx := context.Background()
	idIssue := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, idIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)

	require.NoError(t, repo.UpsertSnapshotToday(ctx, sprint.IdSprint, finalStateIds(t, app, token, idProject)))

	snaps, err := repo.LoadSnapshots(ctx, sprint.IdSprint)
	require.NoError(t, err)
	require.Empty(t, snaps, "no snapshot may be recorded before the first day of the cycle")
}

func TestSprintSnapshot_UpsertOnClosedSprintIsANoOp(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "snap-closed")
	final, _ := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Closing", -5, 9))

	repo := injector.GetSprintRepository()
	ctx := context.Background()
	idsFinal := finalStateIds(t, app, token, idProject)

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 4)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, done, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	require.NoError(t, repo.UpsertSnapshotToday(ctx, sprint.IdSprint, idsFinal))
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)

	late := createIssueWithPoints(t, app, token, idProject, final.IdState, 7)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, late, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	require.NoError(t, repo.UpsertSnapshotToday(ctx, sprint.IdSprint, idsFinal))

	snaps, err := repo.LoadSnapshots(ctx, sprint.IdSprint)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	require.Equal(t, 4, snaps[0].TotalPoints, "a closed cycle's frozen row is never rewritten")
}

func TestSprintVelocity_PlannedComesFromTheBaselineSnapshot(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "velocity-baseline")
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Measured", -10, 4))
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)

	clearSnapshots(t, app, sprint.IdSprint)
	insertSnapshot(t, app, sprint.IdSprint, -12, 99, 0, 40, 0, 40)
	insertSnapshot(t, app, sprint.IdSprint, -10, 20, 5, 12, 3, 12)
	insertSnapshot(t, app, sprint.IdSprint, -6, 26, 21, 15, 11, 15)

	repo := injector.GetSprintRepository()
	entries, err := repo.VelocityByProject(context.Background(), idProject,
		finalStateIds(t, app, token, idProject), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.True(t, entries[0].Frozen)
	require.NotNil(t, entries[0].PlannedPoints)
	require.Equal(t, 20, *entries[0].PlannedPoints, "the plan is the first in-window snapshot, not the pre-start row")
	require.NotNil(t, entries[0].PlannedIssues)
	require.Equal(t, 12, *entries[0].PlannedIssues)
	require.Equal(t, 21, entries[0].DonePoints, "done is the close-scope row")
	require.Equal(t, 11, entries[0].DoneIssues)
}

func TestSprintVelocity_PreFeatureClosedSprintHasNoPlan(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "velocity-prefeature")
	final, _ := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Legacy", -20, -6))

	idIssue := createIssueWithPoints(t, app, token, idProject, final.IdState, 6)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, idIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, sprint.IdSprint).StatusCode)
	clearSnapshots(t, app, sprint.IdSprint)

	repo := injector.GetSprintRepository()
	entries, err := repo.VelocityByProject(context.Background(), idProject,
		finalStateIds(t, app, token, idProject), 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.False(t, entries[0].Frozen)
	require.Nil(t, entries[0].PlannedPoints, "a cycle closed before snapshots existed draws no planned bar")
	require.Nil(t, entries[0].PlannedIssues)
	require.Equal(t, 6, entries[0].DonePoints, "done falls back to the live aggregate")
	require.Equal(t, 1, entries[0].DoneIssues)
}

func TestSprintStats_FrozenFieldsOnlyForAClosedSprintWithASnapshot(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "stats-frozen")
	repo := injector.GetSprintRepository()
	ctx := context.Background()
	idsFinal := finalStateIds(t, app, token, idProject)

	statsOf := func(idSprint int64) *model.SprintStats {
		stats, err := repo.SprintStats(ctx, idSprint, idsFinal, []int64{})
		require.NoError(t, err)
		return stats
	}
	requireNoFrozenFields := func(stats *model.SprintStats, why string) {
		require.Nil(t, stats.RolledOverIssues, why)
		require.Nil(t, stats.FrozenTotalPoints, why)
		require.Nil(t, stats.FrozenDonePoints, why)
		require.Nil(t, stats.FrozenTotalIssues, why)
		require.Nil(t, stats.FrozenDoneIssues, why)
		require.Nil(t, stats.FrozenPointedIssues, why)
	}

	open := createSprint(t, app, token, idProject, sprintWindowBody("Open", -2, 12))
	require.NoError(t, repo.UpsertSnapshotToday(ctx, open.IdSprint, idsFinal))
	requireNoFrozenFields(statsOf(open.IdSprint), "an open cycle renders live numbers even though it has snapshots")

	bare := createSprint(t, app, token, idProject, sprintWindowBody("Bare", -30, -16))
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, bare.IdSprint).StatusCode)
	clearSnapshots(t, app, bare.IdSprint)
	requireNoFrozenFields(statsOf(bare.IdSprint), "a cycle closed before snapshots existed keeps the degraded rendering")

	frozen := createSprint(t, app, token, idProject, sprintWindowBody("Frozen", -40, -26))
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, frozen.IdSprint).StatusCode)
	clearSnapshots(t, app, frozen.IdSprint)
	insertSnapshot(t, app, frozen.IdSprint, -30, 31, 17, 13, 8, 11)
	insertSnapshot(t, app, frozen.IdSprint, -27, 34, 19, 14, 9, 12)

	stats := statsOf(frozen.IdSprint)
	require.NotNil(t, stats.FrozenTotalPoints)
	require.Equal(t, 34, *stats.FrozenTotalPoints)
	require.NotNil(t, stats.FrozenDonePoints)
	require.Equal(t, 19, *stats.FrozenDonePoints)
	require.NotNil(t, stats.FrozenTotalIssues)
	require.Equal(t, 14, *stats.FrozenTotalIssues)
	require.NotNil(t, stats.FrozenDoneIssues)
	require.Equal(t, 9, *stats.FrozenDoneIssues)
	require.NotNil(t, stats.FrozenPointedIssues)
	require.Equal(t, 12, *stats.FrozenPointedIssues)
	require.NotNil(t, stats.RolledOverIssues)
	require.Equal(t, 5, *stats.RolledOverIssues)
}

func TestSprintSnapshot_ScheduledRunCoversEveryOpenStartedCycle(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "snap-scheduled")
	final, nonFinal := finalAndNonFinalState(t, app, token, idProject)

	running := createSprint(t, app, token, idProject, sprintWindowBody("Running", -2, 12))
	future := createSprint(t, app, token, idProject, sprintWindowBody("Future", 5, 19))
	closed := createSprint(t, app, token, idProject, sprintWindowBody("Closed", -20, -6))

	done := createIssueWithPoints(t, app, token, idProject, final.IdState, 3)
	open := createIssueWithPoints(t, app, token, idProject, nonFinal.IdState, 5)
	for _, idIssuePublic := range []int64{done, open} {
		require.Equal(t, http.StatusNoContent,
			assignSprint(t, app, token, idProject, idIssuePublic, fmt.Sprintf(`{"idSprint":%d}`, running.IdSprint)).StatusCode)
	}
	require.Equal(t, http.StatusOK, closeSprint(t, app, token, closed.IdSprint).StatusCode)
	clearSnapshots(t, app, closed.IdSprint)

	repo := injector.GetSprintRepository()
	ctx := context.Background()
	written, err := repo.UpsertSnapshotsForOpenSprints(ctx)
	require.NoError(t, err)
	require.Positive(t, written, "the running cycle is recorded without anyone opening the charts")

	snaps, err := repo.LoadSnapshots(ctx, running.IdSprint)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	require.Equal(t, 8, snaps[0].TotalPoints)
	require.Equal(t, 3, snaps[0].DonePoints, "final states are resolved per project, not passed in")
	require.Equal(t, 2, snaps[0].TotalIssues)
	require.Equal(t, 1, snaps[0].DoneIssues)

	futureSnaps, err := repo.LoadSnapshots(ctx, future.IdSprint)
	require.NoError(t, err)
	require.Empty(t, futureSnaps, "a cycle that has not started records nothing")

	closedSnaps, err := repo.LoadSnapshots(ctx, closed.IdSprint)
	require.NoError(t, err)
	require.Empty(t, closedSnaps, "a closed cycle is never rewritten by the scheduler")
}

func TestSprintSnapshot_ScheduledRunIsIdempotent(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "snap-scheduled-twice")
	final, _ := finalAndNonFinalState(t, app, token, idProject)
	sprint := createSprint(t, app, token, idProject, sprintWindowBody("Running", -1, 13))

	idIssue := createIssueWithPoints(t, app, token, idProject, final.IdState, 4)
	require.Equal(t, http.StatusNoContent,
		assignSprint(t, app, token, idProject, idIssue, fmt.Sprintf(`{"idSprint":%d}`, sprint.IdSprint)).StatusCode)

	repo := injector.GetSprintRepository()
	ctx := context.Background()
	require.Positive(t, mustUpsertOpen(t, repo, ctx))
	require.Zero(t, mustUpsertOpen(t, repo, ctx), "an unchanged day costs no write")

	snaps, err := repo.LoadSnapshots(ctx, sprint.IdSprint)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
}

func mustUpsertOpen(t *testing.T, repo *repository.SprintRepository, ctx context.Context) int64 {
	written, err := repo.UpsertSnapshotsForOpenSprints(ctx)
	require.NoError(t, err)
	return written
}
