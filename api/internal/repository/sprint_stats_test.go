package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

func TestSprintStatsRejectsFilterWithoutSprintOrProject(t *testing.T) {
	repo := NewSprintRepository(nil)

	stats, err := repo.SprintStats(context.Background(), model.SprintStatsFilter{})

	if stats != nil {
		t.Fatalf("expected no stats, got %+v", stats)
	}
	if !errors.Is(err, errs.ErrUnscopedSprintStats) {
		t.Fatalf("expected ErrUnscopedSprintStats, got %v", err)
	}
}
