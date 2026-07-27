package test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

func TestUnscheduleClearsGanttRank(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "gantt-unschedule-proj")
	iss := createScheduledIssue(t, app, token, idProject, "scheduled A")

	repo := injector.GetIssueRepository()
	ctx := context.Background()
	if err := repo.SetGanttRank(ctx, iss.IdIssue, "m"); err != nil {
		t.Fatalf("SetGanttRank: %v", err)
	}

	// Reload, clear scheduledAt, update → should drop the rank.
	full, err := repo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &iss.IdIssue})
	if err != nil {
		t.Fatalf("LoadIssue: %v", err)
	}
	full.ScheduledAt = nil
	if _, err := repo.UpdateIssue(ctx, full); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}

	got, err := repo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &iss.IdIssue})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.GanttRank != nil {
		t.Fatalf("want gantt_rank cleared, got %q", *got.GanttRank)
	}
}
