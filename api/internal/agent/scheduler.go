package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// StageDispatcher seams the scheduler to the real Dispatcher, kept as an
// interface for testability.
type StageDispatcher interface {
	DispatchStage(ctx context.Context, dispatch *Dispatch)
}

type Scheduler struct {
	pool        *pgxpool.Pool
	runRepo     *repository.AgentRunRepository
	taskRepo    *repository.AgentTaskRepository
	projectRepo *repository.ProjectRepository
	issueRepo   *repository.IssueRepository
	dispatcher  StageDispatcher
	kinds       *RunKinds
	notifier    notify.INotifier

	mu sync.Mutex
}

func NewScheduler(
	pool *pgxpool.Pool,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	projectRepo *repository.ProjectRepository,
	issueRepo *repository.IssueRepository,
	dispatcher StageDispatcher,
	kinds *RunKinds,
	notifier notify.INotifier,
) *Scheduler {
	return &Scheduler{
		pool:        pool,
		runRepo:     runRepo,
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		issueRepo:   issueRepo,
		dispatcher:  dispatcher,
		kinds:       kinds,
		notifier:    notifier,
	}
}

// Start runs the scheduler loop until ctx is cancelled, ticking every second
// and claiming at most one pending dispatch per agent user per tick.
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

	agents, err := s.runRepo.LoadActiveAgentIds(ctx)
	if err != nil {
		return fmt.Errorf("loading active agents: %w", err)
	}
	for _, idAgent := range agents {
		if err := s.dispatchOneForAgent(ctx, idAgent); err != nil {
			log.Error().Err(err).Int64("idUserAgent", idAgent).Msg("scheduler: dispatch error")
		}
	}
	return nil
}

// dispatchOneForAgent picks the highest-priority eligible run for this agent and
// dispatches one stage attempt. Skips if the agent already has an active task —
// stages are atomic, so the scheduler waits for it to finish before claiming
// the next.
func (s *Scheduler) dispatchOneForAgent(ctx context.Context, idUserAgent int64) error {
	hasActive, err := s.taskRepo.BotHasActiveTask(ctx, idUserAgent)
	if err != nil {
		return err
	}
	if hasActive {
		return nil
	}

	run, err := s.runRepo.LoadNextEligible(ctx, idUserAgent)
	if err != nil {
		return fmt.Errorf("loading next eligible run: %w", err)
	}
	if run == nil {
		return nil
	}

	// Hold the run row lock: SetShortWorkflow edits the stage plan under the same lock.
	var task *model.AgentTask
	isDrained := false
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		locked, err := s.runRepo.LoadByIdForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		existing, err := s.taskRepo.LoadByRun(ctx, run.IdRun)
		if err != nil {
			return err
		}
		task, err = s.kinds.For(locked).NextTask(ctx, locked, existing)
		isDrained = err == nil && task == nil
		return err
	})
	if err != nil {
		return err
	}
	if isDrained {
		// Queued with nothing left to dispatch is inconsistent (e.g. a
		// re-queue after all stages completed). Mark it done so it stops
		// blocking newer runs.
		if run.Phase == constants.PhaseQueued {
			updated, terr := s.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseDone, constants.ActorTypeSystem, nil, "no stage to dispatch")
			if terr == nil {
				log.Warn().Int64("idRun", run.IdRun).Msg("scheduler: queued run had no stage to dispatch — marked done")
				BroadcastRunUpdate(ctx, s.notifier, s.projectRepo, s.issueRepo, s.runRepo, s.taskRepo, updated)
			}
		}
		return nil
	}
	dispatch, err := s.kinds.For(run).Activate(ctx, run, task)
	if err != nil || dispatch == nil {
		return err
	}
	s.dispatcher.DispatchStage(ctx, dispatch)
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
