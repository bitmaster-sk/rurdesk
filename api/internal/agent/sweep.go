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
	issueRepo     *repository.IssueRepository
	notifier      *notify.Notifier
}

func NewSweep(
	taskRepo *repository.AgentTaskRepository,
	runRepo *repository.AgentRunRepository,
	projectRepo *repository.ProjectRepository,
	issueRepo *repository.IssueRepository,
	notifier *notify.Notifier,
) *Sweep {
	return &Sweep{
		agentTaskRepo: taskRepo,
		agentRunRepo:  runRepo,
		projectRepo:   projectRepo,
		issueRepo:     issueRepo,
		notifier:      notifier,
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
			s.runHeartbeatSweep(ctx)
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
		FailRun(ctx, s.notifier, s.projectRepo, s.issueRepo, s.agentRunRepo, s.agentTaskRepo, idRun, "heartbeat_stale")
	}
}
