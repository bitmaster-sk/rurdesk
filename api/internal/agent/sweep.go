package agent

import (
	"context"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

type Sweep struct {
	agentTaskRepo *repository.AgentTaskRepository
	agentRunRepo  *repository.AgentRunRepository
	projectRepo   *repository.ProjectRepository
	dedupRepo     *repository.WebhookDedupRepository
	notifier      *notify.Notifier
}

func NewSweep(
	taskRepo *repository.AgentTaskRepository,
	runRepo *repository.AgentRunRepository,
	projectRepo *repository.ProjectRepository,
	dedupRepo *repository.WebhookDedupRepository,
	notifier *notify.Notifier,
) *Sweep {
	return &Sweep{
		agentTaskRepo: taskRepo,
		agentRunRepo:  runRepo,
		projectRepo:   projectRepo,
		dedupRepo:     dedupRepo,
		notifier:      notifier,
	}
}

// RunCrashRecovery fails every active task on startup and its parent run
// (FailRun, reason "crash_recovery"), freeing the bot's scheduler gate after
// an API restart. A still-live gateway later reporting complete_stage for
// such a task is reconciled, not rejected — the controller accepts a late
// completion whose error_reason is in the recoverable allowlist.
// Keep the blanket-fail: reconcile depends on the run being `failed` with a
// recoverable reason.
func (s *Sweep) RunCrashRecovery(ctx context.Context) error {
	runIds, err := s.agentTaskRepo.FailStaleHeartbeats(ctx, 0)
	if err != nil {
		return err
	}
	for _, idRun := range runIds {
		log.Info().Int64("idRun", idRun).Msg("crash recovery: failing orphaned run")
		FailRun(ctx, s.notifier, s.projectRepo, s.agentRunRepo, s.agentTaskRepo, idRun, "crash_recovery")
	}
	return nil
}

// StartHeartbeatSweep runs every 2 minutes and fails tasks with no heartbeat for 10+ minutes.
func (s *Sweep) StartHeartbeatSweep(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runHeartbeatSweep(ctx)
		case <-cleanupTicker.C:
			if _, err := s.dedupRepo.Cleanup(ctx, 24*time.Hour); err != nil {
				log.Error().Err(err).Msg("dedup cleanup error")
			}
		}
	}
}

func (s *Sweep) runHeartbeatSweep(ctx context.Context) {
	runIds, err := s.agentTaskRepo.FailStaleHeartbeats(ctx, 10*time.Minute)
	if err != nil {
		log.Error().Err(err).Msg("heartbeat sweep error")
		return
	}
	for _, idRun := range runIds {
		log.Warn().Int64("idRun", idRun).Msg("heartbeat sweep: failing run with stale heartbeat")
		FailRun(ctx, s.notifier, s.projectRepo, s.agentRunRepo, s.agentTaskRepo, idRun, "heartbeat_stale")
	}
}
