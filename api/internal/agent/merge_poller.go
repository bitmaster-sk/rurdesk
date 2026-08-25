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
	stateRepo     *repository.StateRepository
	mirror        *WorkflowEventMirror
	notifier      *notify.Notifier
}

func NewMergePoller(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	projectRepo *repository.ProjectRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	issueRepo *repository.IssueRepository,
	stateRepo *repository.StateRepository,
	mirror *WorkflowEventMirror,
	notifier *notify.Notifier,
) *MergePoller {
	return &MergePoller{
		agentRunRepo:  agentRunRepo,
		agentTaskRepo: agentTaskRepo,
		projectRepo:   projectRepo,
		gitIntRepo:    gitIntRepo,
		issueRepo:     issueRepo,
		stateRepo:     stateRepo,
		mirror:        mirror,
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

		host, err := p.hostFor(ctx, encKey, *run.IdGitIntegration, run.IdProject)
		if err != nil {
			log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to build git host")
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

	p.pollManualMrs(ctx, encKey)

	return nil
}

func (p *MergePoller) notifyRunUpdate(run *model.AgentRun) {
	BroadcastRunUpdate(context.Background(), p.notifier, p.projectRepo, p.agentRunRepo, p.agentTaskRepo, run)
}

func (p *MergePoller) hostFor(ctx context.Context, encKey []byte, idGitIntegration, idProject int64) (githost.GitHost, error) {
	integration, err := p.gitIntRepo.LoadByID(ctx, idGitIntegration, idProject)
	if err != nil {
		return nil, fmt.Errorf("loading git integration %d: %w", idGitIntegration, err)
	}
	if integration == nil {
		return nil, fmt.Errorf("git integration %d not found", idGitIntegration)
	}
	token, err := githost.Decrypt(encKey, integration.TokenNonce, integration.AccessTokenEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypting token: %w", err)
	}
	return githost.NewGitHost(integration.HostType, integration.BaseUrl, integration.RepoPath, string(token))
}

// pollManualMrs pages through every issue with an unresolved manual MR (keyset
// pagination on id_issue), stopping at hardCap per cycle so one project with an
// unbounded backlog can't starve the poller's other work forever.
func (p *MergePoller) pollManualMrs(ctx context.Context, encKey []byte) {
	const batchSize = 50
	const hardCap = 500

	var afterId int64
	processed := 0
	for {
		issues, err := p.issueRepo.LoadIssuesWithOpenMr(ctx, batchSize, afterId)
		if err != nil {
			log.Error().Err(err).Msg("merge poller: failed to load issues with open mr")
			return
		}
		if len(issues) == 0 {
			return
		}

		for _, iss := range issues {
			host, err := p.hostFor(ctx, encKey, *iss.IdGitIntegration, iss.IdProject)
			if err != nil {
				log.Error().Err(err).Int64("idIssue", iss.IdIssue).Msg("merge poller: manual mr host")
				continue
			}
			status, err := host.GetMergeRequestStatus(ctx, *iss.MrId)
			if err != nil {
				log.Error().Err(err).Int64("idIssue", iss.IdIssue).Str("mrId", *iss.MrId).
					Msg("merge poller: manual mr status")
				continue
			}
			p.HandleManualMrStatus(ctx, iss, status)
		}

		processed += len(issues)
		afterId = issues[len(issues)-1].IdIssue

		if len(issues) < batchSize {
			return
		}
		if processed >= hardCap {
			remaining, err := p.issueRepo.CountIssuesWithOpenMr(ctx, afterId)
			if err != nil {
				log.Error().Err(err).Msg("merge poller: failed to count remaining manual mrs after hard cap")
				return
			}
			log.Warn().Int("processed", processed).Int64("remaining", remaining).
				Msg("merge poller: manual mr poll hit hard cap; remaining issues deferred to next cycle")
			return
		}
	}
}

// HandleManualMrStatus stamps a terminal PR outcome exactly once and, on merge,
// emits the done event so the workflow mapping moves the task — unless the task
// already sits in a final state (pre-existing merges must not reopen closed work).
func (p *MergePoller) HandleManualMrStatus(ctx context.Context, iss *model.Issue, status *githost.Status) {
	switch status.State {
	case "merged":
		final, err := p.isFinalState(ctx, iss)
		if err != nil {
			log.Error().Err(err).Int64("idIssue", iss.IdIssue).
				Msg("merge poller: checking final state before stamping merge")
			return
		}
		if err := p.issueRepo.SetMrState(ctx, iss.IdIssue, "merged"); err != nil {
			log.Error().Err(err).Int64("idIssue", iss.IdIssue).Msg("merge poller: set mr_state merged")
			return
		}
		if final {
			return
		}
		p.mirror.ApplyMirror(ctx, iss.IdProject, iss.IdIssue, constants.PhaseDone)
		BroadcastIssueUpdate(ctx, p.notifier, p.issueRepo, p.projectRepo, iss.IdIssue)
	case "closed":
		if err := p.issueRepo.SetMrState(ctx, iss.IdIssue, "closed"); err != nil {
			log.Error().Err(err).Int64("idIssue", iss.IdIssue).Msg("merge poller: set mr_state closed")
		}
	}
}

// isFinalState reports whether iss currently sits in a final state. A load
// error is returned distinctly from "not final" so callers never silently
// treat a transient DB failure as license to move a possibly-final issue.
func (p *MergePoller) isFinalState(ctx context.Context, iss *model.Issue) (bool, error) {
	if iss.IdState == nil {
		return false, nil
	}
	state, err := p.stateRepo.LoadState(ctx, iss.IdProject, *iss.IdState)
	if err != nil {
		return false, fmt.Errorf("loading state %d: %w", *iss.IdState, err)
	}
	if state == nil {
		return false, fmt.Errorf("state %d not found", *iss.IdState)
	}
	return state.Final, nil
}
