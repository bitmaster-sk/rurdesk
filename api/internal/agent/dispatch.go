package agent

import (
	"context"
	"encoding/json"
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
	agentRunRepo *repository.AgentRunRepository
	taskRepo     *repository.AgentTaskRepository
	botGwRepo    *repository.BotGatewayRepository
	issueRepo    *repository.IssueRepository
	messageRepo  *repository.MessageRepository
	projectRepo  *repository.ProjectRepository
	userRepo     *repository.UserRepository
	skillService *service.SkillService
	stagePlan    *service.StagePlanService
	gwClient     *GatewayClient
	notifier     *notify.Notifier
}

func NewDispatcher(
	agentRunRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	botGwRepo *repository.BotGatewayRepository,
	issueRepo *repository.IssueRepository,
	messageRepo *repository.MessageRepository,
	projectRepo *repository.ProjectRepository,
	userRepo *repository.UserRepository,
	skillService *service.SkillService,
	stagePlan *service.StagePlanService,
	gwClient *GatewayClient,
	notifier *notify.Notifier,
) *Dispatcher {
	return &Dispatcher{
		agentRunRepo: agentRunRepo,
		taskRepo:     taskRepo,
		botGwRepo:    botGwRepo,
		issueRepo:    issueRepo,
		messageRepo:  messageRepo,
		projectRepo:  projectRepo,
		userRepo:     userRepo,
		skillService: skillService,
		stagePlan:    stagePlan,
		gwClient:     gwClient,
		notifier:     notifier,
	}
}

// DispatchStageExecute fires the per-stage execution webhook for an agent_task,
// carrying the resolved stage plan and context bundle. On final failure it
// marks both the task and the run failed.
func (d *Dispatcher) DispatchStageExecute(ctx context.Context, run *model.AgentRun, task *model.AgentTask) {
	go func() {
		bgCtx := context.Background()
		bundle, err := d.buildContextBundle(bgCtx, run, task)
		if err != nil {
			log.Error().Err(err).Int64("idTask", task.IdTask).Msg("building context bundle")
			d.failTaskAndRun(bgCtx, run, task, fmt.Sprintf("context bundle build error: %v", err))
			return
		}

		event := WebhookEvent{
			IdRun:     run.IdRun,
			IdProject: run.IdProject,
			IdIssue:   run.IdIssue,
			IdUserBot: run.IdUserBot,
			Event:     "stage_execute",
			Payload: map[string]any{
				"idTask":        task.IdTask,
				"stage":         task.Stage,
				"attemptNo":     task.AttemptNo,
				"stagePlan":     json.RawMessage(run.StagePlan),
				"contextBundle": bundle,
			},
		}
		if err := d.DispatchEvent(bgCtx, run, event); err != nil {
			log.Error().Err(err).Int64("idTask", task.IdTask).Msg("dispatching stage_execute event")
			d.failTaskAndRun(bgCtx, run, task, fmt.Sprintf("dispatch failed: %v", err))
		}
	}()
}

func (d *Dispatcher) DispatchCancelled(ctx context.Context, run *model.AgentRun) error {
	event := WebhookEvent{
		IdRun:     run.IdRun,
		IdProject: run.IdProject,
		IdIssue:   run.IdIssue,
		IdUserBot: run.IdUserBot,
		Event:     "cancelled",
		Payload:   map[string]any{},
	}
	return d.DispatchEvent(ctx, run, event)
}

func (d *Dispatcher) DispatchEvent(ctx context.Context, run *model.AgentRun, event WebhookEvent) error {
	gateway, err := d.botGwRepo.LoadByBotUser(ctx, run.IdUserBot)
	if err != nil || gateway == nil {
		return fmt.Errorf("no gateway configured for bot %d", run.IdUserBot)
	}

	seq, err := d.agentRunRepo.CountEvents(ctx, run.IdRun)
	if err != nil {
		seq = 0
	}
	event.Sequence = seq + 1

	return retryWithBackoff(ctx, func() error {
		return d.gwClient.SendEvent(ctx, gateway, event)
	}, retryBackoffs)
}

// buildContextBundle assembles the per-stage context for a stage attempt.
func (d *Dispatcher) buildContextBundle(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (map[string]any, error) {
	issue, err := d.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &run.IdIssue})
	if err != nil {
		return nil, fmt.Errorf("loading issue: %w", err)
	}
	project, err := d.projectRepo.LoadProject(ctx, run.IdProject)
	if err != nil {
		return nil, fmt.Errorf("loading project: %w", err)
	}
	bot, err := d.userRepo.LoadUser(ctx, run.IdUserBot)
	if err != nil {
		return nil, fmt.Errorf("loading bot user: %w", err)
	}

	messages, err := d.messageRepo.LoadIssueMessages(ctx, run.IdIssue, 0, nil)
	if err != nil {
		messages = nil
	}
	// Must not be swallowed like the load above: without tasks the prompt would
	// tell the agent to implement an approved plan and attach nothing.
	priorTasks, err := d.taskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		return nil, fmt.Errorf("loading prior tasks: %w", err)
	}

	// A skill deleted between planning and dispatch is simply absent: a missing
	// skill must never fail a run.
	var stageSkills []*model.Skill
	if ids := d.stagePlan.IdsSkillForStage(run.StagePlan, task.Stage); len(ids) > 0 {
		loaded, skillErr := d.skillService.LoadByIds(ctx, ids)
		if skillErr != nil {
			log.Warn().Err(skillErr).Int64("idRun", run.IdRun).Msg("loading stage skills — dispatching without them")
		}
		stageSkills = loaded
	}

	artifacts := stageArtifactContext(task.Stage, priorTasks, messages)
	bundle := map[string]any{
		"issue":             issue,
		"project":           project,
		"bot":               bot,
		"pendingComments":   filterCommentsAfterLastAttempt(messages, priorTasks, task.Stage),
		"approvedDesign":    artifacts.ApprovedDesign,
		"approvedImplPlan":  artifacts.ApprovedImplPlan,
		"rejectedOutput":    artifacts.RejectedOutput,
		"approvedMockupRef": derefStringOrNil(run.ApprovedMockupRef),
		"skills":            stageSkills,
	}
	if task.AttemptNo > 1 {
		for _, t := range priorTasks {
			if t.Stage == task.Stage && t.IdTask != task.IdTask && t.Status == constants.TaskStatusFailed {
				bundle["previousFailedAttempt"] = t
			}
		}
	}
	return bundle, nil
}

// filterCommentsAfterLastAttempt returns comments on the issue after the most
// recent completed attempt of `stage`, or all comments if there's no prior
// completed attempt — the first attempt sees the full conversation.
func filterCommentsAfterLastAttempt(messages []*model.Message, priorTasks []*model.AgentTask, stage string) []*model.Message {
	var cutoff time.Time
	for _, t := range priorTasks {
		if t.Stage == stage && t.Status == constants.TaskStatusCompleted && t.FinishedAt != nil {
			if t.FinishedAt.After(cutoff) {
				cutoff = *t.FinishedAt
			}
		}
	}
	var out []*model.Message
	for _, m := range messages {
		if m.MessageKind != constants.MessageKindComment {
			continue
		}
		if !cutoff.IsZero() && !m.CreatedAt.After(cutoff) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// derefStringOrNil returns the string or nil so it serialises as a JSON string
// or null, never an empty-string placeholder.
func derefStringOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// stageArtifacts splits prior outputs per stage: earlier stages' artifacts are
// genuinely approved (the run only advances on approval), while the latest
// output of the stage being (re-)executed is by definition rejected —
// re-entering a stage means the user sent it back.
type stageArtifacts struct {
	ApprovedDesign   *model.Message
	ApprovedImplPlan *model.Message
	RejectedOutput   *model.Message
}

// stageArtifactContext maps prior outputs to approved/rejected per stage; see
// stageArtifacts for the rule. Resolve through the run's task rows, never by
// scanning the issue for a message_kind — the issue also holds older revisions
// and earlier runs' output.
func stageArtifactContext(stage string, tasks []*model.AgentTask, messages []*model.Message) stageArtifacts {
	latestDesign := latestStageOutput(constants.StageDesign, tasks, messages)
	latestImplPlan := latestStageOutput(constants.StageImplementationPlan, tasks, messages)

	switch stage {
	case constants.StageDesign:
		return stageArtifacts{RejectedOutput: latestDesign}
	case constants.StageImplementationPlan:
		return stageArtifacts{ApprovedDesign: latestDesign, RejectedOutput: latestImplPlan}
	case constants.StageImplementation:
		return stageArtifacts{ApprovedDesign: latestDesign, ApprovedImplPlan: latestImplPlan}
	default: // brainstorming, pickup — no prior artifacts are relevant
		return stageArtifacts{}
	}
}

// latestStageOutput returns the output message of the run's newest completed
// attempt of stage. Picks by explicit max, not slice position — neither input
// has a guaranteed order.
func latestStageOutput(stage string, tasks []*model.AgentTask, messages []*model.Message) *model.Message {
	var latest *model.AgentTask
	for _, task := range tasks {
		if task.Stage != stage || task.Status != constants.TaskStatusCompleted || task.IdOutputMessage == nil {
			continue
		}
		if latest == nil || task.AttemptNo > latest.AttemptNo ||
			(task.AttemptNo == latest.AttemptNo && task.CreatedAt.After(latest.CreatedAt)) {
			latest = task
		}
	}
	if latest == nil {
		return nil
	}
	return findMessageById(messages, *latest.IdOutputMessage)
}

func findMessageById(messages []*model.Message, idMessage int64) *model.Message {
	for _, message := range messages {
		if message.IdMessage == idMessage {
			return message
		}
	}
	return nil
}

func (d *Dispatcher) failTaskAndRun(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason string) {
	_, _ = d.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusFailed)
	_ = d.taskRepo.SetError(ctx, task.IdTask, "dispatch_failed", reason)
	updated, err := d.agentRunRepo.TransitionPhase(ctx, run.IdRun, run.Phase, constants.PhaseFailed, constants.ActorTypeSystem, nil, reason)
	if err == nil {
		BroadcastRunUpdate(ctx, d.notifier, d.projectRepo, d.agentRunRepo, d.taskRepo, updated)
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
