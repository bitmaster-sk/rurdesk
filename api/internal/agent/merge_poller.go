package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// MergePoller polls GitHost.GetMergeRequestStatus for runs in pr_open phase,
// advancing them to merged or failed based on PR state. Runs as a background
// goroutine on a 60s interval.
type MergePoller struct {
	agentRunRepo  *repository.AgentRunRepository
	agentTaskRepo *repository.AgentTaskRepository
	projectRepo   *repository.ProjectRepository
	gitIntRepo    *repository.GitIntegrationRepository
	issueRepo     *repository.IssueRepository
	notifier      *notify.Notifier
}

func NewMergePoller(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	projectRepo *repository.ProjectRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	issueRepo *repository.IssueRepository,
	notifier *notify.Notifier,
) *MergePoller {
	return &MergePoller{
		agentRunRepo:  agentRunRepo,
		agentTaskRepo: agentTaskRepo,
		projectRepo:   projectRepo,
		gitIntRepo:    gitIntRepo,
		issueRepo:     issueRepo,
		notifier:      notifier,
	}
}

func (p *MergePoller) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.PollOnce(ctx); err != nil {
				log.Error().Err(err).Msg("merge poller: poll failed")
			}
		}
	}
}

// PollOnce runs a single poll cycle; exported for testing.
func (p *MergePoller) PollOnce(ctx context.Context) error {
	runs, err := p.agentRunRepo.LoadPrOpenRuns(ctx, 50)
	if err != nil {
		return fmt.Errorf("loading pr_open runs: %w", err)
	}

	var merged, closed, open int

	encKey, err := githost.LoadEncryptionKey()
	if err != nil {
		return fmt.Errorf("loading encryption key: %w", err)
	}

	for _, run := range runs {
		if run.IdGitIntegration == nil {
			log.Warn().Int64("idRun", run.IdRun).Int64("idProject", run.IdProject).
				Msg("merge poller: run has no git integration — cannot poll PR status")
			continue
		}

		integration, err := p.gitIntRepo.LoadByID(ctx, *run.IdGitIntegration, run.IdProject)
		if err != nil {
			log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to load git integration")
			continue
		}
		if integration == nil {
			log.Warn().Int64("idRun", run.IdRun).Int64("idGitIntegration", *run.IdGitIntegration).
				Msg("merge poller: git integration not found — cannot poll PR status")
			continue
		}

		token, err := githost.Decrypt(encKey, integration.TokenNonce, integration.AccessTokenEnc)
		if err != nil {
			log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to decrypt token")
			continue
		}

		host, err := githost.NewGitHost(integration.HostType, integration.BaseUrl, integration.RepoPath, string(token))
		if err != nil {
			log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to create git host")
			continue
		}

		status, err := host.GetMergeRequestStatus(ctx, *run.PrId)
		if err != nil {
			log.Error().Err(err).Int64("idRun", run.IdRun).Str("prId", *run.PrId).
				Msg("merge poller: failed to get PR status")
			continue
		}

		switch status.State {
		case "merged":
			updated, err := p.agentRunRepo.TransitionPhase(ctx, run.IdRun, constants.PhasePrOpen, constants.PhaseDone, constants.ActorTypeSystem, nil, "PR merged")
			if err != nil {
				log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to transition to done")
				continue
			}
			p.notifyRunUpdate(updated)
			BroadcastIssueUpdate(ctx, p.notifier, p.issueRepo, p.projectRepo, run.IdIssue)
			merged++
			log.Info().Int64("idRun", run.IdRun).Msg("merge poller: run transitioned to done")

		case "closed":
			updated, err := p.agentRunRepo.TransitionPhase(ctx, run.IdRun, constants.PhasePrOpen, constants.PhaseFailed, constants.ActorTypeSystem, nil, "PR closed without merge")
			if err != nil {
				log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to transition to failed")
				continue
			}
			if setErr := p.agentRunRepo.SetErrorMessage(ctx, run.IdRun, "PR closed without merge"); setErr != nil {
				log.Error().Err(setErr).Int64("idRun", run.IdRun).Msg("merge poller: failed to set error message")
			}
			p.notifyRunUpdate(updated)
			BroadcastIssueUpdate(ctx, p.notifier, p.issueRepo, p.projectRepo, run.IdIssue)
			closed++
			log.Info().Int64("idRun", run.IdRun).Msg("merge poller: run transitioned to failed (PR closed)")

		default:
			open++
		}
	}

	log.Debug().
		Int("merged", merged).
		Int("closed", closed).
		Int("open", open).
		Int("total", len(runs)).
		Msg("merge poller: poll complete")

	return nil
}

func (p *MergePoller) notifyRunUpdate(run *model.AgentRun) {
	BroadcastRunUpdate(context.Background(), p.notifier, p.projectRepo, p.agentRunRepo, p.agentTaskRepo, run)
}
