package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewOptionalIfSet_LeavesNilUndefined(t *testing.T) {
	require.False(t, NewOptionalIfSet[int64](nil).IsDefined)
	value := int64(7)
	set := NewOptionalIfSet(&value)
	require.True(t, set.IsDefined)
	require.Equal(t, int64(7), *set.Value)
}

func TestIssueChangesSchedule(t *testing.T) {
	at := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	later := at.Add(time.Hour)
	issue := &Issue{ScheduledAt: &at, Estimated: 3600}

	require.False(t, issue.ChangesSchedule(Optional[time.Time]{}, Optional[int64]{}))
	require.False(t, issue.ChangesSchedule(NewOptional(at), NewOptional(int64(3600))))
	require.True(t, issue.ChangesSchedule(NewOptional(later), Optional[int64]{}))
	require.True(t, issue.ChangesSchedule(NewOptionalNull[time.Time](), Optional[int64]{}))
	require.True(t, issue.ChangesSchedule(Optional[time.Time]{}, NewOptional(int64(7200))))
}

func TestIssueRollupInputsDiffer(t *testing.T) {
	at := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	base := Issue{ScheduledAt: &at, Estimated: 3600, Tracked: 60, Title: "a"}

	renamed := base
	renamed.Title = "b"
	require.False(t, base.RollupInputsDiffer(&renamed))

	tracked := base
	tracked.Tracked = 120
	require.True(t, base.RollupInputsDiffer(&tracked))

	unscheduled := base
	unscheduled.ScheduledAt = nil
	require.True(t, base.RollupInputsDiffer(&unscheduled))
}
