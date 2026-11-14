package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func int64Ptr(value int64) *int64 {
	return &value
}

func TestDiffIssue(t *testing.T) {
	at := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	bratislava, err := time.LoadLocation("Europe/Bratislava")
	require.NoError(t, err)
	sameInstant := at.In(bratislava)
	base := Issue{
		IdState:     int64Ptr(1),
		Title:       "Old title",
		Description: "text",
		Estimated:   3600,
		IdsTag:      []int64{2, 1},
	}

	tests := []struct {
		name     string
		mutate   func(issue *Issue)
		expected []IssueHistoryChange
	}{
		{
			name:     "no change",
			mutate:   func(_ *Issue) {},
			expected: nil,
		},
		{
			name: "title and state",
			mutate: func(issue *Issue) {
				issue.Title = "New title"
				issue.IdState = int64Ptr(2)
			},
			expected: []IssueHistoryChange{
				{Field: IssueHistoryFieldState, OldValue: int64(1), NewValue: int64(2)},
				{Field: IssueHistoryFieldTitle, OldValue: "Old title", NewValue: "New title"},
			},
		},
		{
			name:     "tag order only",
			mutate:   func(issue *Issue) { issue.IdsTag = []int64{1, 2} },
			expected: nil,
		},
		{
			name:   "tag added",
			mutate: func(issue *Issue) { issue.IdsTag = []int64{3, 1, 2} },
			expected: []IssueHistoryChange{
				{Field: IssueHistoryFieldTags, OldValue: []int64{1, 2}, NewValue: []int64{1, 2, 3}},
			},
		},
		{
			name:   "scheduled from nil to a date",
			mutate: func(issue *Issue) { issue.ScheduledAt = &at },
			expected: []IssueHistoryChange{
				{Field: IssueHistoryFieldScheduledAt, OldValue: nil, NewValue: at},
			},
		},
		{
			name:   "assignee set",
			mutate: func(issue *Issue) { issue.AssignedTo = int64Ptr(5) },
			expected: []IssueHistoryChange{
				{Field: IssueHistoryFieldAssignee, OldValue: nil, NewValue: int64(5)},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			changed := base
			changed.IdsTag = append([]int64(nil), base.IdsTag...)
			testCase.mutate(&changed)
			require.Equal(t, testCase.expected, DiffIssue(&base, &changed))
		})
	}

	t.Run("same instant in another location", func(t *testing.T) {
		old := Issue{ScheduledAt: &at}
		changed := Issue{ScheduledAt: &sameInstant}
		require.Empty(t, DiffIssue(&old, &changed))
	})

	t.Run("assignee removed", func(t *testing.T) {
		old := Issue{AssignedTo: int64Ptr(5)}
		changed := Issue{}
		require.Equal(t, []IssueHistoryChange{
			{Field: IssueHistoryFieldAssignee, OldValue: int64(5), NewValue: nil},
		}, DiffIssue(&old, &changed))
	})

	t.Run("nil and empty tags are equal", func(t *testing.T) {
		old := Issue{IdsTag: nil}
		changed := Issue{IdsTag: []int64{}}
		require.Empty(t, DiffIssue(&old, &changed))
	})

	t.Run("removing the last tag stores an empty array", func(t *testing.T) {
		old := Issue{IdsTag: []int64{4}}
		changed := Issue{IdsTag: nil}
		require.Equal(t, []IssueHistoryChange{
			{Field: IssueHistoryFieldTags, OldValue: []int64{4}, NewValue: []int64{}},
		}, DiffIssue(&old, &changed))
	})
}

func TestIssueHistoryUserIds(t *testing.T) {
	history := []IssueHistory{
		{Field: IssueHistoryFieldTitle, IdUser: int64Ptr(1), OldValue: []byte(`"a"`), NewValue: []byte(`"b"`)},
		{Field: IssueHistoryFieldAssignee, IdUser: int64Ptr(1), OldValue: []byte(`null`), NewValue: []byte(`7`)},
		{Field: IssueHistoryFieldAssignee, IdUser: nil, OldValue: []byte(`7`), NewValue: []byte(`9`)},
		{Field: IssueHistoryFieldState, IdUser: nil, OldValue: []byte(`3`), NewValue: []byte(`4`)},
	}

	require.Equal(t, []int64{1, 7, 9}, IssueHistoryUserIds(history))
}
