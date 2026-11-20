package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// InterruptFailedTask hands a task a bulk update already failed to its run's kind.
func InterruptFailedTask(
	ctx context.Context,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	kinds *RunKinds,
	failed repository.FailedTask,
	reason InterruptReason,
) error {
	run, err := runRepo.LoadById(ctx, failed.IdRun)
	if err != nil {
		return fmt.Errorf("loading run %d: %w", failed.IdRun, err)
	}
	task, err := taskRepo.LoadById(ctx, failed.IdTask)
	if err != nil {
		return fmt.Errorf("loading task %d: %w", failed.IdTask, err)
	}
	return kinds.For(run).Interrupt(ctx, run, task, reason, "")
}

type Sweep struct {
	agentTaskRepo *repository.AgentTaskRepository
	agentRunRepo  *repository.AgentRunRepository
	kinds         *RunKinds
}

func NewSweep(
	taskRepo *repository.AgentTaskRepository,
	runRepo *repository.AgentRunRepository,
	kinds *RunKinds,
) *Sweep {
	return &Sweep{
		agentTaskRepo: taskRepo,
		agentRunRepo:  runRepo,
		kinds:         kinds,
	}
}

// StartHeartbeatSweep runs every 2 minutes and fails tasks with no heartbeat for 10+ minutes.
func (s *Sweep) StartHeartbeatSweep(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.HeartbeatSweepOnce(ctx)
		}
	}
}

// HeartbeatSweepOnce fails the active tasks whose heartbeat is older than 10 minutes and interrupts their runs.
func (s *Sweep) HeartbeatSweepOnce(ctx context.Context) {
	failed, err := s.agentTaskRepo.FailStaleHeartbeats(ctx, 10*time.Minute)
	if err != nil {
		log.Error().Err(err).Msg("heartbeat sweep error")
		return
	}
	for _, failedTask := range failed {
		log.Warn().Int64("idRun", failedTask.IdRun).Msg("heartbeat sweep: interrupting run with stale heartbeat")
		if err := InterruptFailedTask(ctx, s.agentRunRepo, s.agentTaskRepo, s.kinds, failedTask, InterruptHeartbeatStale); err != nil {
			log.Error().Err(err).Int64("idRun", failedTask.IdRun).Msg("heartbeat sweep: interrupting run")
		}
	}
}
