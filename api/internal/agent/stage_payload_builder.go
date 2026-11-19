package agent

import (
	"context"
	"fmt"
	"sort"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/rs/zerolog/log"
)

// buildStagePayload builds the stage_execute payload of an issue or release notes stage and
// returns the wiki pages its prompt carries, recorded once the payload is delivered.
func (d *Dispatcher) buildStagePayload(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (map[string]any, []*model.AgentRunWikiRead, error) {
	bundle, wikiReads, err := d.buildContextBundle(ctx, run, task)
	if err != nil {
		return nil, nil, fmt.Errorf("context bundle build error: %w", err)
	}

	var repo *model.AgentRunGitRepo
	prBranch := ""
	if run.IdIssue != nil {
		repo, err = d.agentRunGitRepo(ctx, run)
		if err != nil {
			return nil, nil, err
		}
		prBranch, err = d.openPrBranch(ctx, run)
		if err != nil {
			log.Warn().Err(err).Int64("idRun", run.IdRun).Str("prBranch", prBranch).Msg("resolving the issue's open PR branch")
		}
	}

	payload := map[string]any{
		"idTask":        task.IdTask,
		"stage":         task.Stage,
		"attemptNo":     task.AttemptNo,
		"stagePlan":     run.StagePlan,
		"contextBundle": bundle,
		"prBranch":      prBranch,
		"repo":          repo,
	}
	if constants.IsSkillStage(task.Stage) {
		taskConfig, err := d.resolveAgentTaskConfig(ctx, run, task)
		if err != nil {
			return nil, nil, fmt.Errorf("model resolution error: %w", err)
		}
		payload["maxTurns"] = taskConfig.MaxTurns
		if taskConfig.Model != nil {
			payload["model"] = *taskConfig.Model
		}
		if taskConfig.Effort != nil {
			payload["effort"] = *taskConfig.Effort
		}
	}
	return payload, wikiReads, nil
}

// buildContextBundle assembles the per-stage context for a stage attempt.
func (d *Dispatcher) buildContextBundle(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (map[string]any, []*model.AgentRunWikiRead, error) {
	project, err := d.projectRepo.LoadProject(ctx, run.IdProject)
	if err != nil {
		return nil, nil, fmt.Errorf("loading project: %w", err)
	}
	agent, err := d.userRepo.LoadUser(ctx, run.IdUserAgent)
	if err != nil {
		return nil, nil, fmt.Errorf("loading agent user: %w", err)
	}
	// Must not be swallowed: without tasks the prompt would
	// tell the agent to implement an approved plan and attach nothing.
	priorTasks, err := d.taskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		return nil, nil, fmt.Errorf("loading prior tasks: %w", err)
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

	// Like a missing skill, an unreadable wiki must never fail a run.
	wiki, wikiReads, err := d.wikiAgent.BuildPromptContext(ctx, run, task)
	if err != nil {
		log.Warn().Err(err).Int64("idRun", run.IdRun).Msg("building wiki context — dispatching without it")
		wiki, wikiReads = nil, nil
	}

	var proposals []model.WikiProposalAgentView
	if task.Stage == constants.StageImplementation {
		proposals, err = d.proposals.LoadForPrompt(ctx, run.IdRun)
		if err != nil {
			log.Warn().Err(err).Int64("idRun", run.IdRun).Msg("loading wiki proposals — dispatching without them")
			proposals = nil
		}
	}

	bundle := map[string]any{
		"project":           project,
		"agent":             agent,
		"approvedMockupRef": derefStringOrNil(run.ApprovedMockupRef),
		"skills":            stageSkills,
		"wiki":              wiki,
		"wikiProposals":     proposals,
		"isShortWorkflow":   d.stagePlan.IsShortWorkflow(run.StagePlan),
	}
	if run.IdIssue != nil {
		issue, err := d.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: run.IdIssue})
		if err != nil {
			return nil, nil, fmt.Errorf("loading issue: %w", err)
		}
		messages, err := d.messageRepo.LoadIssueMessages(ctx, *run.IdIssue, 0, nil)
		if err != nil {
			messages = nil
		}
		artifacts := stageArtifactContext(task.Stage, priorTasks, messages)
		bundle["issue"] = issue
		bundle["reviewThread"] = reviewThread(messages)
		bundle["approvedDesign"] = artifacts.ApprovedDesign
		bundle["approvedImplPlan"] = artifacts.ApprovedImplPlan
		bundle["rejectedOutput"] = artifacts.RejectedOutput
	}
	if run.IdRelease != nil {
		release, err := d.releases.PromptContext(ctx, run)
		if err != nil {
			return nil, nil, fmt.Errorf("loading release: %w", err)
		}
		bundle["release"] = release
	}
	bundle["resumedAfter"], bundle["isRevision"] = ResolveAttemptContext(priorTasks, task)
	return bundle, wikiReads, nil
}

// reviewThread returns the whole issue conversation oldest-first. Never trim it
// to the current round: an attempt that cannot see the instruction behind the
// code it is revising reverts that code back to the approved plan.
func reviewThread(messages []*model.Message) []*model.Message {
	out := make([]*model.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, withoutArtifactBody(message))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].IdMessage < out[j].IdMessage
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// withoutArtifactBody drops the body of the two kinds the bundle already ships
// whole as approvedDesign / approvedImplPlan, keeping the thread under the
// gateway's 1 MiB webhook limit. Copies: those fields alias these messages.
func withoutArtifactBody(message *model.Message) *model.Message {
	switch message.MessageKind {
	case constants.MessageKindDesign, constants.MessageKindImplementationPlan:
		trimmed := *message
		trimmed.Message = ""
		return &trimmed
	default:
		return message
	}
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
		if task.Stage != stage || task.Status != constants.TaskStatusCompleted || task.IdResultMessage == nil {
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
	return findMessageById(messages, *latest.IdResultMessage)
}

func findMessageById(messages []*model.Message, idMessage int64) *model.Message {
	for _, message := range messages {
		if message.IdMessage == idMessage {
			return message
		}
	}
	return nil
}

func (d *Dispatcher) resolveAgentTaskConfig(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (service.AgentTaskConfig, error) {
	gateway, err := d.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil {
		return service.AgentTaskConfig{}, fmt.Errorf("loading gateway: %w", err)
	}
	if gateway == nil {
		return service.AgentTaskConfig{}, fmt.Errorf("no gateway configured for agent %d", run.IdUserAgent)
	}
	plan, err := d.stagePlan.Parse(run.StagePlan)
	if err != nil {
		log.Warn().Err(err).Int64("idRun", run.IdRun).Msg("unparseable stage plan, dispatching with defaults")
	}

	modelsWithoutThinking, err := d.agentTaskConfig.ModelsWithoutThinking(ctx, gateway)
	if err != nil {
		return service.AgentTaskConfig{}, err
	}
	taskConfig := d.agentTaskConfig.BuildAgentTaskConfig(plan, task.Stage, gateway, modelsWithoutThinking)
	if taskConfig.FallbackFrom != nil {
		used := ""
		if taskConfig.Model != nil {
			used = *taskConfig.Model
		}
		log.Warn().Int64("idTask", task.IdTask).Str("requested", *taskConfig.FallbackFrom).Str("used", used).Msg("model no longer offered by the gateway, falling back to its default")
		fallback := model.ModelFallbackPayload{IdTask: task.IdTask, Requested: *taskConfig.FallbackFrom, Used: used}
		if err := d.agentRunRepo.InsertEvent(ctx, run.IdRun, run.Phase, run.Phase, constants.ActorTypeSystem, nil, constants.RunEventModelFallback, fallback); err != nil {
			return service.AgentTaskConfig{}, err
		}
	}
	if err := d.taskRepo.SetRunConfig(ctx, task.IdTask, taskConfig.Model, taskConfig.MaxTurns, taskConfig.Effort); err != nil {
		return service.AgentTaskConfig{}, err
	}
	task.Model, task.MaxTurns = taskConfig.Model, &taskConfig.MaxTurns
	task.Effort = taskConfig.Effort
	return taskConfig, nil
}

func (d *Dispatcher) openPrBranch(ctx context.Context, run *model.AgentRun) (string, error) {
	prRun, err := d.agentRunRepo.LoadLatestWithPrByIssue(ctx, *run.IdIssue)
	if err != nil {
		return "", fmt.Errorf("loading the issue's latest PR run: %w", err)
	}
	if prRun == nil {
		return "", nil
	}
	isOpen, err := d.isPrOpen(ctx, prRun)
	if err != nil {
		// A fresh branch here would stick for the whole run and open a duplicate PR.
		return *prRun.BranchName, err
	}
	if !isOpen {
		return "", nil
	}
	return *prRun.BranchName, nil
}

func (d *Dispatcher) isPrOpen(ctx context.Context, prRun *model.AgentRun) (bool, error) {
	integration, err := d.gitIntRepo.LoadByID(ctx, *prRun.IdGitIntegration, prRun.IdProject)
	if err != nil {
		return false, fmt.Errorf("loading git integration %d: %w", *prRun.IdGitIntegration, err)
	}
	if integration == nil {
		return false, fmt.Errorf("git integration %d not found", *prRun.IdGitIntegration)
	}
	host, err := githost.BuildFromIntegration(integration)
	if err != nil {
		return false, fmt.Errorf("building git host: %w", err)
	}
	status, err := host.GetMergeRequestStatus(ctx, *prRun.PrId)
	if err != nil {
		return false, fmt.Errorf("loading PR %s status: %w", *prRun.PrId, err)
	}
	return status.State == constants.MrStateOpen, nil
}
