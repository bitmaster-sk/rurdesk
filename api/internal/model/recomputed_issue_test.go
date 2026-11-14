package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecomputedIssueHistoryChanges(t *testing.T) {
	at := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	tracked := int64(600)
	moreTracked := int64(1200)

	t.Run("no real change", func(t *testing.T) {
		values := RollupValues{ScheduledAt: &at, Estimated: 3600, RollupTracked: &tracked}
		recomputed := RecomputedIssue{Before: values, After: values}
		require.Empty(t, recomputed.HistoryChanges())
	})

	t.Run("parent becomes summary", func(t *testing.T) {
		rollupEstimated := int64(3600)
		recomputed := RecomputedIssue{
			Before: RollupValues{IsSummary: false, ScheduledAt: nil, Estimated: 7200},
			After:  RollupValues{IsSummary: true, ScheduledAt: &at, Estimated: 3600, RollupEstimated: &rollupEstimated},
		}
		require.Equal(t, []IssueHistoryChange{
			{Field: IssueHistoryFieldIsSummary, OldValue: false, NewValue: true},
			{Field: IssueHistoryFieldScheduledAt, OldValue: nil, NewValue: at},
			{Field: IssueHistoryFieldEstimated, OldValue: int64(7200), NewValue: int64(3600)},
			{Field: IssueHistoryFieldRollupEstimated, OldValue: nil, NewValue: int64(3600)},
		}, recomputed.HistoryChanges())
	})

	t.Run("only tracked rollup", func(t *testing.T) {
		recomputed := RecomputedIssue{
			Before: RollupValues{RollupTracked: &tracked},
			After:  RollupValues{RollupTracked: &moreTracked},
		}
		require.Equal(t, []IssueHistoryChange{
			{Field: IssueHistoryFieldRollupTracked, OldValue: int64(600), NewValue: int64(1200)},
		}, recomputed.HistoryChanges())
	})
}
