package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// StageDispatcher seams the scheduler to the real Dispatcher, kept as an
// interface for testability.
type StageDispatcher interface {
	DispatchStageExecute(ctx context.Context, run *model.AgentRun, task *model.AgentTask)
}

type Scheduler struct {
	runRepo     *repository.AgentRunRepository
	taskRepo    *repository.AgentTaskRepository
	projectRepo *repository.ProjectRepository
	dispatcher  StageDispatcher
	notifier    *notify.Notifier

	mu sync.Mutex
}

func NewScheduler(
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	projectRepo *repository.ProjectRepository,
	dispatcher StageDispatcher,
	notifier *notify.Notifier,
) *Scheduler {
	return &Scheduler{
		runRepo:     runRepo,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		dispatcher:  dispatcher,
		notifier:    notifier,
	}
}

// Start runs the scheduler loop until ctx is cancelled, ticking every second
// and claiming at most one pending dispatch per bot user per tick.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.TickOnce(ctx); err != nil {
				log.Error().Err(err).Msg("scheduler: tick error")
			}
		}
	}
}

// TickOnce performs a single dispatch pass.
func (s *Scheduler) TickOnce(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bots, err := s.runRepo.LoadActiveBotIds(ctx)
	if err != nil {
		return fmt.Errorf("loading active bots: %w", err)
	}
	for _, idBot := range bots {
		if err := s.dispatchOneForBot(ctx, idBot); err != nil {
			log.Error().Err(err).Int64("idUserBot", idBot).Msg("scheduler: dispatch error")
		}
	}
	return nil
}

// dispatchOneForBot picks the highest-priority eligible run for this bot and
// dispatches one stage attempt. Skips if the bot already has an active task —
// stages are atomic, so the scheduler waits for it to finish before claiming
// the next.
func (s *Scheduler) dispatchOneForBot(ctx context.Context, idUserBot int64) error {
	hasActive, err := s.taskRepo.BotHasActiveTask(ctx, idUserBot)
	if err != nil {
		return err
	}
	if hasActive {
		return nil
	}

	run, err := s.runRepo.LoadNextEligible(ctx, idUserBot)
	if err != nil {
		return fmt.Errorf("loading next eligible run: %w", err)
	}
	if run == nil {
		return nil
	}

	existing, err := s.taskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		return err
	}

	// A pending task can already exist (a revision or other re-queue created
	// it directly). Dispatch it — ResolveNextStage may miss it if its stage
	// already shows completed, and without this the run stays queued
	// forever, shadowing newer runs for the same bot.
	if pending := latestPendingTask(existing); pending != nil {
		return s.activateAndDispatch(ctx, run, pending)
	}

	stage, err := ResolveNextStage(run, existing)
	if err != nil {
		return err
	}
	if stage == "" {
		// Queued with nothing left to dispatch is inconsistent (e.g. a
		// re-queue after all stages completed). Mark it done so it stops
		// blocking newer runs.
		if run.Phase == constants.PhaseQueued {
			updated, terr := s.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseDone, constants.ActorTypeSystem, nil, "no stage to dispatch")
			if terr == nil {
				log.Warn().Int64("idRun", run.IdRun).Msg("scheduler: queued run had no stage to dispatch — marked done")
				BroadcastRunUpdate(ctx, s.notifier, s.projectRepo, s.runRepo, s.taskRepo, updated)
			}
		}
		return nil
	}

	attemptNo := ResolveNextAttemptNo(existing, stage)
	task, err := s.taskRepo.Insert(ctx, run.IdRun, run.IdUserBot, stage, attemptNo)
	if err != nil {
		return err
	}
	return s.activateAndDispatch(ctx, run, task)
}

// activateAndDispatch marks the task active, moves the run queued→in_progress,
// broadcasts the snapshot, and fires stage_execute.
func (s *Scheduler) activateAndDispatch(ctx context.Context, run *model.AgentRun, task *model.AgentTask) error {
	if task.Status != constants.TaskStatusActive {
		if _, err := s.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive); err != nil {
			return err
		}
	}
	broadcastRun := run
	if run.Phase == constants.PhaseQueued {
		updated, err := s.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseInProgress, constants.ActorTypeSystem, nil, "dispatch stage "+task.Stage)
		if err != nil {
			log.Debug().Err(err).Int64("idRun", run.IdRun).Msg("scheduler: queued→in_progress transition skipped")
		} else {
			broadcastRun = updated
		}
	}
	// Must fire even when the run was already in_progress (e.g. dispatching
	// the next stage right after a user approval) — otherwise the timeline
	// shows the prior stage as pending until it completes.
	BroadcastRunUpdate(ctx, s.notifier, s.projectRepo, s.runRepo, s.taskRepo, broadcastRun)
	s.dispatcher.DispatchStageExecute(ctx, run, task)
	return nil
}

// latestPendingTask returns the most recently created pending task, or nil.
func latestPendingTask(tasks []*model.AgentTask) *model.AgentTask {
	var found *model.AgentTask
	for _, t := range tasks {
		if t.Status == constants.TaskStatusPending && (found == nil || t.CreatedAt.After(found.CreatedAt)) {
			found = t
		}
	}
	return found
}
