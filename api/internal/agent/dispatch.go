package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/rs/zerolog/log"
)

var retryBackoffs = []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}

type Dispatcher struct {
	agentRunRepo     *repository.AgentRunRepository
	taskRepo         *repository.AgentTaskRepository
	agentGwRepo      *repository.AgentGatewayRepository
	gitIntRepo       *repository.GitIntegrationRepository
	issueRepo        *repository.IssueRepository
	messageRepo      *repository.MessageRepository
	projectRepo      *repository.ProjectRepository
	userRepo         *repository.UserRepository
	skillService     *service.SkillService
	stagePlan        *service.AgentStagePlanService
	wikiAgent        *service.WikiAgentService
	proposals        *service.WikiProposalService
	releases         *service.ReleaseService
	agentTaskConfig  *service.AgentTaskConfigService
	conversationRepo *repository.AgentConversationRepository
	conversations    *service.AgentConversationService
	chatTurns        *ChatTurns
	gwClient         *GatewayClient
	notifier         notify.INotifier
}

func NewDispatcher(
	agentRunRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	agentGwRepo *repository.AgentGatewayRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	issueRepo *repository.IssueRepository,
	messageRepo *repository.MessageRepository,
	projectRepo *repository.ProjectRepository,
	userRepo *repository.UserRepository,
	skillService *service.SkillService,
	stagePlan *service.AgentStagePlanService,
	wikiAgent *service.WikiAgentService,
	proposals *service.WikiProposalService,
	releases *service.ReleaseService,
	agentTaskConfig *service.AgentTaskConfigService,
	conversationRepo *repository.AgentConversationRepository,
	conversations *service.AgentConversationService,
	chatTurns *ChatTurns,
	gwClient *GatewayClient,
	notifier notify.INotifier,
) *Dispatcher {
	return &Dispatcher{
		agentRunRepo:     agentRunRepo,
		taskRepo:         taskRepo,
		agentGwRepo:      agentGwRepo,
		gitIntRepo:       gitIntRepo,
		issueRepo:        issueRepo,
		messageRepo:      messageRepo,
		projectRepo:      projectRepo,
		userRepo:         userRepo,
		skillService:     skillService,
		stagePlan:        stagePlan,
		wikiAgent:        wikiAgent,
		proposals:        proposals,
		releases:         releases,
		agentTaskConfig:  agentTaskConfig,
		conversationRepo: conversationRepo,
		conversations:    conversations,
		chatTurns:        chatTurns,
		gwClient:         gwClient,
		notifier:         notifier,
	}
}

// DispatchStageExecute fires the per-stage execution webhook for an agent_task,
// carrying the resolved stage plan and context bundle. On final failure it
// marks both the task and the run failed.
func (d *Dispatcher) DispatchStageExecute(_ context.Context, run *model.AgentRun, task *model.AgentTask) {
	go func() {
		bgCtx := context.Background()
		payload, wikiReads, err := d.buildStagePayload(bgCtx, run, task)
		if err != nil {
			log.Error().Err(err).Int64("idTask", task.IdTask).Msg("building stage_execute payload")
			d.failTaskAndRun(bgCtx, run, task, err.Error())
			return
		}

		event := WebhookEvent{
			IdRun:       run.IdRun,
			IdProject:   run.IdProject,
			IdIssue:     run.IdIssue,
			IdUserAgent: run.IdUserAgent,
			Event:       "stage_execute",
			Payload:     payload,
		}
		if err := d.DispatchEvent(bgCtx, run, event); err != nil {
			log.Error().Err(err).Int64("idTask", task.IdTask).Msg("dispatching stage_execute event")
			d.failTaskAndRun(bgCtx, run, task, fmt.Sprintf("dispatch failed: %v", err))
			return
		}
		if err := d.wikiAgent.RecordReads(bgCtx, wikiReads); err != nil {
			log.Error().Err(err).Int64("idTask", task.IdTask).Msg("recording wiki reads of the prompt")
		}
		BroadcastRunUpdate(bgCtx, d.notifier, d.projectRepo, d.issueRepo, d.agentRunRepo, d.taskRepo, run)
	}()
}

// DispatchChatTurn fires stage_execute for a chat turn with the conversation as history and the
// turn's run token. A turn that cannot be delivered ends with an error message.
func (d *Dispatcher) DispatchChatTurn(_ context.Context, run *model.AgentRun, task *model.AgentTask, runToken string) {
	go func() {
		bgCtx := context.Background()
		payload, err := d.buildChatPayload(bgCtx, run, task, runToken)
		if err == nil {
			err = d.DispatchEvent(bgCtx, run, WebhookEvent{
				IdRun:       run.IdRun,
				IdProject:   run.IdProject,
				IdUserAgent: run.IdUserAgent,
				Event:       "stage_execute",
				Payload:     payload,
			})
		}
		if err == nil {
			d.chatTurns.NotifyTurnStarted(bgCtx, run, task)
			return
		}
		log.Error().Err(err).Int64("idTask", task.IdTask).Msg("dispatching chat turn")
		if _, failErr := d.chatTurns.Fail(bgCtx, run, task, "dispatch_failed", err.Error()); failErr != nil {
			log.Error().Err(failErr).Int64("idTask", task.IdTask).Msg("ending undeliverable chat turn")
		}
	}()
}

func (d *Dispatcher) DispatchCancelled(ctx context.Context, run *model.AgentRun) error {
	event := WebhookEvent{
		IdRun:       run.IdRun,
		IdProject:   run.IdProject,
		IdIssue:     run.IdIssue,
		IdUserAgent: run.IdUserAgent,
		Event:       "cancelled",
		Payload:     map[string]any{},
	}
	return d.DispatchEvent(ctx, run, event)
}

// DispatchRunDiscarded tells the gateway that Restart replaced the run, so it
// stops the run's stage and removes its worktree.
func (d *Dispatcher) DispatchRunDiscarded(ctx context.Context, run *model.AgentRun) error {
	event := WebhookEvent{
		IdRun:       run.IdRun,
		IdProject:   run.IdProject,
		IdIssue:     run.IdIssue,
		IdUserAgent: run.IdUserAgent,
		Event:       "run_discarded",
		Payload:     map[string]any{},
	}
	return d.DispatchEvent(ctx, run, event)
}

func (d *Dispatcher) DispatchEvent(ctx context.Context, run *model.AgentRun, event WebhookEvent) error {
	gateway, err := d.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil || gateway == nil {
		return fmt.Errorf("no gateway configured for bot %d", run.IdUserAgent)
	}

	seq, err := d.agentRunRepo.CountEvents(ctx, run.IdRun)
	if err != nil {
		seq = 0
	}
	event.Sequence = seq + 1

	return d.gwClient.DeliverEvent(ctx, gateway, event, retryBackoffs)
}

func (d *Dispatcher) failTaskAndRun(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason string) {
	_, _ = d.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusFailed)
	_ = d.taskRepo.SetError(ctx, task.IdTask, "dispatch_failed", reason)
	updated, err := d.agentRunRepo.TransitionPhase(ctx, run.IdRun, run.Phase, constants.PhaseFailed, constants.ActorTypeSystem, nil, reason)
	if err == nil {
		BroadcastRunUpdate(ctx, d.notifier, d.projectRepo, d.issueRepo, d.agentRunRepo, d.taskRepo, updated)
	}
}

func retryWithBackoff(ctx context.Context, fn func() error, backoffs []time.Duration) error {
	var lastErr error
	for i, delay := range backoffs {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if i < len(backoffs)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return fmt.Errorf("all retries exhausted: %w", lastErr)
}
