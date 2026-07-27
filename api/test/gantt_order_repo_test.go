package test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestSetAndLoadGanttRanks(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-repo-proj")
	iss := createScheduledIssue(t, app, token, idProject, "scheduled A")

	repo := injector.GetIssueRepository()
	ctx := context.Background()

	// Capture audit columns before the rank write — a reorder must not bump them.
	before, err := repo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &iss.IdIssue})
	require.Nil(t, err)

	if err := repo.SetGanttRank(ctx, iss.IdIssue, "m"); err != nil {
		t.Fatalf("SetGanttRank: %v", err)
	}

	rows, err := repo.LoadScheduledGanttRanks(ctx, idProject)
	if err != nil {
		t.Fatalf("LoadScheduledGanttRanks: %v", err)
	}
	var found *string
	for _, row := range rows {
		if row.IdIssue == iss.IdIssue {
			found = row.GanttRank
		}
	}
	if found == nil || *found != "m" {
		t.Fatalf("want rank \"m\", got %v", found)
	}

	// update_at / update_by must be untouched by a pure ordering write.
	after, err := repo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &iss.IdIssue})
	require.Nil(t, err)
	require.True(t, before.UpdateAt.Equal(after.UpdateAt), "reorder must not bump update_at")
	require.Equal(t, before.UpdateBy, after.UpdateBy)
}
