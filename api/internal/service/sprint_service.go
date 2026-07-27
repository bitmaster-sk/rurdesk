package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sprintWindow = 14 * 24 * time.Hour

// ErrSprintClosed is returned when closing a sprint that is already closed, so
// the controller can map it to 409 and a double-close never re-runs the rollover.
var ErrSprintClosed = errors.New("sprint already closed")

// NextName produces the auto-suggested name for the next sprint of a project.
func NextName(maxSeq int) string {
	return fmt.Sprintf("Sprint %d", maxSeq+1)
}

// DefaultWindow picks a 2-week window starting after the project's latest cycle
// end (or now, if there is no prior cycle or its end is in the past).
func DefaultWindow(latestEnd *time.Time, now time.Time) (time.Time, time.Time) {
	start := now
	if latestEnd != nil && latestEnd.After(now) {
		start = *latestEnd
	}
	return start, start.Add(sprintWindow)
}

type SprintService struct {
	pool       *pgxpool.Pool
	sprintRepo *repository.SprintRepository
	stateRepo  *repository.StateRepository
}

func NewSprintService(pool *pgxpool.Pool, sprintRepo *repository.SprintRepository, stateRepo *repository.StateRepository) *SprintService {
	return &SprintService{pool: pool, sprintRepo: sprintRepo, stateRepo: stateRepo}
}

func (s *SprintService) Create(ctx context.Context, idProject int64, req model.CreateSprintReq, idUser int64) (*model.Sprint, error) {
	name := req.Name
	if name == "" {
		maxSeq, err := s.sprintRepo.MaxNameSeq(ctx, idProject)
		if err != nil {
			return nil, err
		}
		name = NextName(maxSeq)
	}
	var start, end time.Time
	if req.StartAt != nil && req.EndAt != nil {
		start, end = *req.StartAt, *req.EndAt
	} else {
		latest, err := s.sprintRepo.LatestEnd(ctx, idProject)
		if err != nil {
			return nil, err
		}
		start, end = DefaultWindow(latest, time.Now().UTC())
	}
	return s.sprintRepo.Insert(ctx, &model.Sprint{
		IdProject: idProject,
		Name:      name,
		StartAt:   start,
		EndAt:     end,
		State:     "planned",
	}, idUser)
}

// Close is transactional and idempotent: closing an already-closed sprint returns
// ErrSprintClosed (→ 409), so a double- or concurrent close never re-runs the rollover.
// Unfinished issues move to the next planned cycle, or to Backlog if none exists.
func (s *SprintService) Close(ctx context.Context, idSprint int64, idUser int64) (int64, error) {
	var moved int64
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		sprint, err := s.sprintRepo.LoadOne(ctx, idSprint)
		if err != nil {
			return err
		}
		if sprint.State == "closed" {
			return ErrSprintClosed
		}
		finalIds, err := s.stateRepo.FinalStateIds(ctx, sprint.IdProject)
		if err != nil {
			return err
		}
		next, err := s.sprintRepo.NextPlanned(ctx, sprint.IdProject, idSprint)
		if err != nil {
			return err
		}
		var target *int64
		if next != nil {
			target = &next.IdSprint
		}
		moved, err = s.sprintRepo.MoveUnfinished(ctx, idSprint, target, finalIds)
		if err != nil {
			return err
		}
		sprint.State = "closed"
		_, err = s.sprintRepo.Update(ctx, sprint, idUser)
		return err
	})
	return moved, err
}
