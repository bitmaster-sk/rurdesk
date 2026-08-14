package service

import (
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestNextName(t *testing.T) {
	if got := NextName(11); got != "Sprint 12" {
		t.Fatalf("got %q, want Sprint 12", got)
	}
	if got := NextName(0); got != "Sprint 1" {
		t.Fatalf("got %q, want Sprint 1", got)
	}
}

func TestDefaultWindow(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	latest := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	start, end := DefaultWindow(&latest, now)
	if !start.Equal(latest) || end.Sub(start) != 14*24*time.Hour {
		t.Fatalf("future latest end: expected latest+14d, got %v..%v", start, end)
	}

	start, end = DefaultWindow(nil, now)
	if !start.Equal(now) || end.Sub(start) != 14*24*time.Hour {
		t.Fatalf("no prior sprint: expected now+14d, got %v..%v", start, end)
	}

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start, _ = DefaultWindow(&past, now)
	if !start.Equal(now) {
		t.Fatalf("past latest end must not push start into the past, got %v", start)
	}
}

func TestDefaultWindowTruncatesToUtcDay(t *testing.T) {
	now := time.Date(2026, 3, 1, 17, 42, 13, 500, time.UTC)
	start, end := DefaultWindow(nil, now)

	want := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(want) {
		t.Fatalf("start: got %v, want %v", start, want)
	}
	if end.Sub(start) != 14*24*time.Hour {
		t.Fatalf("window length changed: %v", end.Sub(start))
	}
}

func snapshotAt(day time.Time, totalPoints, donePoints, totalIssues, doneIssues int) *model.SprintSnapshot {
	return &model.SprintSnapshot{
		Day:         day,
		TotalPoints: totalPoints, DonePoints: donePoints,
		TotalIssues: totalIssues, DoneIssues: doneIssues,
	}
}

func burndownFixture(startDay, endDay int) *model.Sprint {
	return &model.Sprint{
		IdSprint: 1,
		StartAt:  time.Date(2026, 5, startDay, 0, 0, 0, 0, time.UTC),
		EndAt:    time.Date(2026, 5, endDay, 0, 0, 0, 0, time.UTC),
	}
}

func TestBuildBurndownDaysWithoutSnapshotsIsEmpty(t *testing.T) {
	days := BuildBurndownDays(burndownFixture(1, 15), nil)
	require.NotNil(t, days)
	require.Empty(t, days)
}

func TestBuildBurndownDaysLeavesHolesBeforeTheFirstSnapshot(t *testing.T) {
	sprint := burndownFixture(1, 15)
	snaps := []*model.SprintSnapshot{
		snapshotAt(time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), 20, 5, 10, 2),
	}

	days := BuildBurndownDays(sprint, snaps)

	require.Len(t, days, 15, "labels span the planned window, up to and including the end day")
	for i := 0; i < 3; i++ {
		require.Nil(t, days[i].RemainingPoints, "day %d is before recording began", i+1)
		require.False(t, days[i].Snapshot)
	}
	require.Equal(t, 15, *days[3].RemainingPoints)
	require.Equal(t, 8, *days[3].RemainingIssues)
	require.True(t, days[3].Snapshot)
}

func TestBuildBurndownDaysCarriesGapDaysInsideTheRecordedRange(t *testing.T) {
	sprint := burndownFixture(1, 15)
	snaps := []*model.SprintSnapshot{
		snapshotAt(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 20, 0, 10, 0),
		snapshotAt(time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), 20, 8, 10, 4),
	}

	days := BuildBurndownDays(sprint, snaps)

	require.Equal(t, 20, *days[1].RemainingPoints, "gap day carries the previous value")
	require.False(t, days[1].Snapshot)
	require.True(t, days[3].Snapshot)
	require.Nil(t, days[4].RemainingPoints, "nothing is carried past the last recorded day")
	require.Nil(t, days[6].RemainingPoints)
}

func TestBuildBurndownDaysFoldsAPreStartBaselineOntoDayOne(t *testing.T) {
	sprint := burndownFixture(10, 24)
	snaps := []*model.SprintSnapshot{
		snapshotAt(time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC), 99, 0, 40, 0),
		snapshotAt(time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), 30, 2, 12, 1),
	}

	days := BuildBurndownDays(sprint, snaps)

	require.Equal(t, 28, *days[0].RemainingPoints, "the latest pre-start row is the baseline")
	require.False(t, days[0].Snapshot, "a folded pre-start row is not a real day")
	require.Nil(t, days[1].RemainingPoints, "a folded baseline is one point, not a flat line")
	require.Len(t, days, 15)
}

func TestBuildBurndownDaysPrefersARealFirstDayOverPreStartRows(t *testing.T) {
	sprint := burndownFixture(10, 24)
	snaps := []*model.SprintSnapshot{
		snapshotAt(time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), 99, 0, 40, 0),
		snapshotAt(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), 30, 0, 12, 0),
	}

	days := BuildBurndownDays(sprint, snaps)

	require.Equal(t, 30, *days[0].RemainingPoints, "an in-window baseline drops every pre-start row")
	require.True(t, days[0].Snapshot)
}

func TestBuildBurndownDaysExtendPastAPlannedEnd(t *testing.T) {
	sprint := burndownFixture(1, 8)
	snaps := []*model.SprintSnapshot{
		snapshotAt(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 20, 0, 10, 0),
		snapshotAt(time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC), 20, 15, 10, 8),
	}

	days := BuildBurndownDays(sprint, snaps)

	require.Len(t, days, 11, "labels run to the last recorded day, not the planned end")
	require.True(t, days[10].Snapshot)
	require.Equal(t, 5, *days[10].RemainingPoints)
}

func TestBuildBurndownDaysDoNotTrailAfterAClosedCycle(t *testing.T) {
	sprint := burndownFixture(1, 15)
	snaps := []*model.SprintSnapshot{
		snapshotAt(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 20, 0, 10, 0),
		snapshotAt(time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), 20, 15, 10, 8),
	}

	days := BuildBurndownDays(sprint, snaps)

	require.Len(t, days, 15)
	require.Equal(t, 5, *days[7].RemainingPoints, "the close day is the last real value")
	require.True(t, days[7].Snapshot)
	for i := 8; i < len(days); i++ {
		require.Nil(t, days[i].RemainingPoints, "day %d is after the cycle stopped being recorded", i+1)
	}
}
