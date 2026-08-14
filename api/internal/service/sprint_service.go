package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/timeutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const defaultSprintWindow = 14 * 24 * time.Hour

// Raising this multiplies the burndown payload: it is one row per calendar day.
const maxSprintWindow = 400 * 24 * time.Hour

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
	start = timeutil.TruncateClock(start.UTC())
	return start, start.Add(defaultSprintWindow)
}

func sprintWindowOf(startAt, endAt time.Time) (time.Time, time.Time, error) {
	start := timeutil.TruncateClock(startAt.UTC())
	end := timeutil.TruncateClock(endAt.UTC())
	if !end.After(start) || end.Sub(start) > maxSprintWindow {
		return time.Time{}, time.Time{}, errs.ErrSprintWindow
	}
	return start, end, nil
}

func BuildBurndownDays(sprint *model.Sprint, snaps []*model.SprintSnapshot) []model.SprintBurndownDay {
	days := []model.SprintBurndownDay{}
	if len(snaps) == 0 {
		return days
	}
	startDay := timeutil.TruncateClock(sprint.StartAt.UTC())
	endDay := timeutil.TruncateClock(sprint.EndAt.UTC())

	byDay := make(map[time.Time]*model.SprintSnapshot, len(snaps))
	var preStart *model.SprintSnapshot
	lastDay := startDay
	lastRecorded := startDay
	for _, sn := range snaps {
		day := timeutil.TruncateClock(sn.Day.UTC())
		if day.Before(startDay) {
			if preStart == nil || day.After(timeutil.TruncateClock(preStart.Day.UTC())) {
				preStart = sn
			}
			continue
		}
		byDay[day] = sn
		if day.After(lastDay) {
			lastDay = day
			lastRecorded = day
		}
	}
	if len(byDay) > 0 {
		preStart = nil
	}
	if lastDay.Before(endDay) {
		lastDay = endDay
	}

	var carried *model.SprintSnapshot
	for day := startDay; !day.After(lastDay); day = day.AddDate(0, 0, 1) {
		sn, real := byDay[day]
		switch {
		case real:
			carried = sn
			days = append(days, burndownDay(day, sn, true))
		case day.Equal(startDay) && preStart != nil:
			carried = preStart
			days = append(days, burndownDay(day, preStart, false))
		case carried != nil && !day.After(lastRecorded):
			days = append(days, burndownDay(day, carried, false))
		default:
			days = append(days, model.SprintBurndownDay{Day: day})
		}
	}
	return days
}

func burndownDay(day time.Time, sn *model.SprintSnapshot, real bool) model.SprintBurndownDay {
	totalPoints, donePoints := sn.TotalPoints, sn.DonePoints
	totalIssues, doneIssues := sn.TotalIssues, sn.DoneIssues
	remainingPoints := totalPoints - donePoints
	remainingIssues := totalIssues - doneIssues
	return model.SprintBurndownDay{
		Day:             day,
		TotalPoints:     &totalPoints,
		DonePoints:      &donePoints,
		RemainingPoints: &remainingPoints,
		TotalIssues:     &totalIssues,
		DoneIssues:      &doneIssues,
		RemainingIssues: &remainingIssues,
		Snapshot:        real,
	}
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
		var err error
		start, end, err = sprintWindowOf(*req.StartAt, *req.EndAt)
		if err != nil {
			return nil, err
		}
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
		State:     constants.SprintStatePlanned,
	}, idUser)
}

func (s *SprintService) Edit(ctx context.Context, sprint *model.Sprint, req model.EditSprintReq, idUser int64) (*model.Sprint, error) {
	if sprint.State == constants.SprintStateClosed {
		return nil, errs.ErrSprintClosed
	}
	start, end, err := sprintWindowOf(req.StartAt, req.EndAt)
	if err != nil {
		return nil, err
	}
	sprint.Name = req.Name
	sprint.StartAt = start
	sprint.EndAt = end
	return s.sprintRepo.Update(ctx, sprint, idUser)
}

// A failed snapshot must not fail the read.
func (s *SprintService) Burndown(ctx context.Context, sprint *model.Sprint) (*model.SprintBurndown, error) {
	if sprint.State != constants.SprintStateClosed {
		finalIds, err := s.stateRepo.FinalStateIds(ctx, sprint.IdProject)
		if err != nil {
			log.Warn().Err(err).Int64("idSprint", sprint.IdSprint).Msg("sprint snapshot skipped: final states unavailable")
		} else if err := s.sprintRepo.UpsertSnapshotToday(ctx, sprint.IdSprint, finalIds); err != nil {
			log.Warn().Err(err).Int64("idSprint", sprint.IdSprint).Msg("sprint snapshot upsert failed")
		}
	}
	snaps, err := s.sprintRepo.LoadSnapshots(ctx, sprint.IdSprint)
	if err != nil {
		return nil, err
	}
	return &model.SprintBurndown{
		IdSprint: sprint.IdSprint,
		StartAt:  sprint.StartAt,
		EndAt:    sprint.EndAt,
		State:    sprint.State,
		Days:     BuildBurndownDays(sprint, snaps),
	}, nil
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
		if sprint.State == constants.SprintStateClosed {
			return errs.ErrSprintClosed
		}
		finalIds, err := s.stateRepo.FinalStateIds(ctx, sprint.IdProject)
		if err != nil {
			return err
		}
		if err := s.sprintRepo.UpsertSnapshotToday(ctx, idSprint, finalIds); err != nil {
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
		sprint.State = constants.SprintStateClosed
		_, err = s.sprintRepo.Update(ctx, sprint, idUser)
		return err
	})
	return moved, err
}
