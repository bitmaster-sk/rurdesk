package model

import (
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/pointer"
	"github.com/bitmaster-sk/rurdesk/api/internal/timeutil"
)

type RollupValues struct {
	IsSummary       bool
	ScheduledAt     *time.Time
	Estimated       int64
	RollupEstimated *int64
	RollupTracked   *int64
}

type RecomputedIssue struct {
	IdIssue int64
	Before  RollupValues
	After   RollupValues
}

func (i RecomputedIssue) HistoryChanges() []IssueHistoryChange {
	var changes []IssueHistoryChange
	if i.Before.IsSummary != i.After.IsSummary {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldIsSummary, OldValue: i.Before.IsSummary, NewValue: i.After.IsSummary})
	}
	if !timeutil.PtrEqual(i.Before.ScheduledAt, i.After.ScheduledAt) {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldScheduledAt, OldValue: pointer.Value(i.Before.ScheduledAt), NewValue: pointer.Value(i.After.ScheduledAt)})
	}
	if i.Before.Estimated != i.After.Estimated {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldEstimated, OldValue: i.Before.Estimated, NewValue: i.After.Estimated})
	}
	if !pointer.Equal(i.Before.RollupEstimated, i.After.RollupEstimated) {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldRollupEstimated, OldValue: pointer.Value(i.Before.RollupEstimated), NewValue: pointer.Value(i.After.RollupEstimated)})
	}
	if !pointer.Equal(i.Before.RollupTracked, i.After.RollupTracked) {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldRollupTracked, OldValue: pointer.Value(i.Before.RollupTracked), NewValue: pointer.Value(i.After.RollupTracked)})
	}
	return changes
}
