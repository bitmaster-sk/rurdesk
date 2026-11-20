package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type AgentRunController struct {
	agentRunRepo    *repository.AgentRunRepository
	agentTaskRepo   *repository.AgentTaskRepository
	tasks           *agent.TaskService
	thinking        *agent.ThinkingService
	agentGwRepo     *repository.AgentGatewayRepository
	issueRepo       *repository.IssueRepository
	projectRepo     *repository.ProjectRepository
	acl             *service.AclService
	stagePlan       *service.AgentStagePlanService
	agentTaskConfig *service.AgentTaskConfigService
	dispatcher      *agent.Dispatcher
	kinds           *agent.RunKinds
	notifier        notify.INotifier
	pool            *pgxpool.Pool
}

func NewAgentRunController(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	tasks *agent.TaskService,
	thinking *agent.ThinkingService,
	agentGwRepo *repository.AgentGatewayRepository,
	issueRepo *repository.IssueRepository,
	projectRepo *repository.ProjectRepository,
	acl *service.AclService,
	stagePlan *service.AgentStagePlanService,
	agentTaskConfig *service.AgentTaskConfigService,
	dispatcher *agent.Dispatcher,
	kinds *agent.RunKinds,
	notifier notify.INotifier,
	pool *pgxpool.Pool,
) *AgentRunController {
	return &AgentRunController{
		agentRunRepo:    agentRunRepo,
		agentTaskRepo:   agentTaskRepo,
		tasks:           tasks,
		thinking:        thinking,
		agentGwRepo:     agentGwRepo,
		issueRepo:       issueRepo,
		projectRepo:     projectRepo,
		acl:             acl,
		stagePlan:       stagePlan,
		agentTaskConfig: agentTaskConfig,
		dispatcher:      dispatcher,
		kinds:           kinds,
		notifier:        notifier,
		pool:            pool,
	}
}

func (ctrl *AgentRunController) GetRun(c *gin.Context) {
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		ResponseErr(c, errs.ErrNotFound)
		return
	}
	if err != nil {
		ResponseErr(c, err)
		return
	}

	if !ctrl.acl.CanReadProject(ctx, user.IdUser, run.IdProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	ctrl.respondRunSnapshot(ctx, c, run)
}

func (ctrl *AgentRunController) GetRunByIssue(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	idIssuePublic, err := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ctrl.acl.CanReadProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	issue, err := ctrl.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: &idIssuePublic})
	if err != nil {
		ResponseErr(c, err)
		return
	}

	run, err := ctrl.agentRunRepo.LoadActiveByIssue(ctx, issue.IdIssue)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if run == nil {
		run, err = ctrl.agentRunRepo.LoadLatestByIssue(ctx, issue.IdIssue)
		if err != nil {
			ResponseErr(c, err)
			return
		}
	}
	if run == nil {
		c.Status(http.StatusNoContent)
		return
	}

	ctrl.respondRunSnapshot(ctx, c, run)
}

func (ctrl *AgentRunController) GetRunsByProject(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ctrl.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	runs, err := ctrl.agentRunRepo.LoadByProject(ctx, idProject, 50)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []*model.AgentRun{}
	}
	c.JSON(http.StatusOK, runs)
}

// Approve transitions awaiting_approval → in_progress. The scheduler picks up
// the next stage on its next tick.
func (ctrl *AgentRunController) Approve(c *gin.Context) {
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	// Body is optional: plain approve sends {} or nothing; approving a specific
	// mockup carries {"mockupRef": "..."}.
	var body struct {
		MockupRef *string `json:"mockupRef"`
	}
	_ = c.ShouldBindJSON(&body)

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !ctrl.acl.CanUpdateIssue(ctx, user.IdUser, run.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if run.Phase != constants.PhaseAwaitingApproval {
		_ = c.Error(errs.ErrRunNotAwaitingApproval)
		c.Status(http.StatusConflict)
		return
	}

	// Persist the mockup ref BEFORE the phase transition so the two writes can't
	// leave the run advanced-but-without-a-ref: on transition failure the run
	// stays awaiting_approval and the ref is simply overwritten by the next
	// approve. TransitionPhase's RETURNING then carries the ref into `updated`.
	if body.MockupRef != nil && *body.MockupRef != "" {
		if err := ctrl.agentRunRepo.SetApprovedMockupRef(ctx, idRun, *body.MockupRef); err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	idUser := user.IdUser
	updated, err := ctrl.agentRunRepo.TransitionPhase(
		ctx, idRun, constants.PhaseAwaitingApproval, constants.PhaseInProgress,
		constants.ActorTypeUser, &idUser, "approved",
	)
	if err == repository.ErrPhaseMismatch {
		_ = c.Error(errs.ErrPhaseMismatch)
		c.Status(http.StatusConflict)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	ctrl.notifyRunUpdate(updated)
	ctrl.respondRunSnapshot(ctx, c, updated)
}

// Cancel transitions the run to cancelled and cancels any non-terminal tasks.
func (ctrl *AgentRunController) Cancel(c *gin.Context) {
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !ctrl.acl.CanUpdateIssue(ctx, user.IdUser, run.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if constants.TerminalPhases[run.Phase] {
		_ = c.Error(errs.ErrRunTerminal)
		c.Status(http.StatusConflict)
		return
	}

	idUser := user.IdUser
	updated, err := ctrl.agentRunRepo.TransitionPhase(ctx, idRun, run.Phase, constants.PhaseCancelled, constants.ActorTypeUser, &idUser, "user cancelled")
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if idsTask, err := ctrl.agentTaskRepo.CancelNonTerminalForRun(ctx, idRun); err != nil {
		log.Warn().Err(err).Int64("idRun", idRun).Msg("cancel: failed to cancel non-terminal tasks")
	} else {
		ctrl.thinking.Compact(ctx, idsTask...)
	}

	go func() {
		_ = ctrl.dispatcher.DispatchCancelled(context.Background(), run)
	}()

	ctrl.notifyRunUpdate(updated)
	ctrl.respondRunSnapshot(ctx, c, updated)
}

// Continue creates a fresh pending agent_task for the failed stage and
// returns the run to phase=queued so the scheduler picks it up.
func (ctrl *AgentRunController) Continue(c *gin.Context) {
	ctx := c.Request.Context()
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !ctrl.acl.CanUpdateIssue(ctx, user.IdUser, run.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if run.Phase != constants.PhaseFailed && run.Phase != constants.PhaseCancelled {
		_ = c.Error(errs.ErrRunContinueInvalid)
		c.Status(http.StatusConflict)
		return
	}

	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, idRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	stage, err := agent.ResolveNextStage(run, tasks)
	if err != nil || stage == "" {
		_ = c.Error(errs.ErrNoStageToContinue)
		c.Status(http.StatusConflict)
		return
	}
	idUser := user.IdUser
	updated, err := ctrl.agentRunRepo.TransitionPhase(ctx, idRun, run.Phase, constants.PhaseQueued, constants.ActorTypeUser, &idUser, "continue")
	if err != nil {
		_ = c.Error(errs.ErrConflict.WithMessage(err.Error()))
		c.Status(http.StatusConflict)
		return
	}
	ctrl.notifyRunUpdate(updated)
	c.JSON(http.StatusOK, gin.H{"idRun": idRun, "nextStage": stage})
}

// Restart soft-cancels the existing run and its tasks, then creates a fresh
// run for the same issue/agent/project. Message log is preserved.
func (ctrl *AgentRunController) Restart(c *gin.Context) {
	ctx := c.Request.Context()
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	user, _ := extctx.GetUser(ctx)

	oldRun, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		ResponseErr(c, errs.ErrNotFound)
		return
	}
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if !ctrl.acl.CanUpdateIssue(ctx, user.IdUser, oldRun.IdProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	// refuse to restart a run that already has a PR — it would push a new
	// branch and open a duplicate PR while orphaning the existing one. Guard on
	// pr_id, not phase: `phase == pr_open` alone misses edge phases.
	if oldRun.PrId != nil && *oldRun.PrId != "" {
		ResponseErr(c, errs.ErrRunHasPr)
		return
	}

	idUser := user.IdUser
	if !constants.TerminalPhases[oldRun.Phase] {
		_, _ = ctrl.agentRunRepo.TransitionPhase(ctx, idRun, oldRun.Phase, constants.PhaseCancelled, constants.ActorTypeUser, &idUser, "restart")
	}
	if idsTask, err := ctrl.agentTaskRepo.CancelNonTerminalForRun(ctx, idRun); err != nil {
		log.Warn().Err(err).Int64("idRun", idRun).Msg("restart: failed to cancel non-terminal tasks")
	} else {
		ctrl.thinking.Compact(ctx, idsTask...)
	}
	// The gateway's retention removes the worktree anyway, so a failed delivery
	// only delays the cleanup.
	go func() {
		if err := ctrl.dispatcher.DispatchRunDiscarded(context.Background(), oldRun); err != nil {
			log.Warn().Err(err).Int64("idRun", idRun).Msg("restart: failed to tell the gateway to discard the run")
		}
	}()

	stagePlan, err := ctrl.stagePlan.BuildFromChoices(oldRun.Kind, ctrl.stagePlan.Choices(oldRun.StagePlan))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	newRun, err := ctrl.agentRunRepo.Insert(ctx, oldRun, stagePlan)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ctrl.notifyRunUpdate(newRun)
	ResponseOk(c, gin.H{"oldIdRun": idRun, "newIdRun": newRun.IdRun})
}

func (ctrl *AgentRunController) GetStagePlan(c *gin.Context) {
	run, ok := ctrl.loadRunForStagePlan(c, false)
	if !ok {
		return
	}
	ctrl.respondRunStagePlan(c, run)
}

func (ctrl *AgentRunController) PatchStagePlanEntry(c *gin.Context) {
	ctx := c.Request.Context()
	run, ok := ctrl.loadRunForStagePlan(c, true)
	if !ok {
		return
	}

	var dto model.UpdateAgentRunStagePlanEntryReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}

	gateway, err := ctrl.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if gateway == nil {
		gateway = &model.AgentGateway{}
	}
	plan, err := ctrl.stagePlan.Parse(run.StagePlan)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if err := ctrl.agentTaskConfig.ValidateChoices(gateway, stagePlanEntryChoices(dto, plan)); err != nil {
		ResponseErr(c, err)
		return
	}

	dispatched, err := ctrl.dispatchedStages(ctx, run.IdRun)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	// The stage's prompt was already built and sent, so its settings are final.
	if dispatched[dto.Stage] {
		ResponseErr(c, errs.ErrStageDispatched)
		return
	}

	updated, err := ctrl.stagePlan.SetStagePlanEntry(ctx, run.IdRun, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ctrl.notifyRunUpdate(updated)
	ctrl.respondRunStagePlan(c, updated)
}

// A model saved before the gateway stopped offering it is not re-checked: dispatch
// already falls back to the default, and rejecting it would block every other edit.
func stagePlanEntryChoices(dto model.UpdateAgentRunStagePlanEntryReq, plan model.AgentStagePlan) model.RunChoices {
	choices := model.RunChoices{IdsSkillByStage: map[string][]int64{dto.Stage: dto.IdsSkill}}
	if dto.Model != "" && dto.Model != savedStageModel(plan, dto.Stage) {
		choices.ModelByStage = map[string]string{dto.Stage: dto.Model}
	}
	if dto.MaxTurns != 0 {
		choices.MaxTurnsByStage = map[string]int{dto.Stage: dto.MaxTurns}
	}
	if dto.Effort != "" {
		choices.EffortByStage = map[string]string{dto.Stage: dto.Effort}
	}
	return choices
}

func savedStageModel(plan model.AgentStagePlan, stage string) string {
	for _, entry := range plan.Stages {
		if entry.Name == stage {
			return entry.Model
		}
	}
	return ""
}

func (ctrl *AgentRunController) PatchWorkflow(c *gin.Context) {
	ctx := c.Request.Context()
	run, ok := ctrl.loadRunForStagePlan(c, true)
	if !ok {
		return
	}

	var dto model.UpdateAgentRunWorkflowReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}

	updated, err := ctrl.stagePlan.SetShortWorkflow(ctx, run.IdRun, dto.IsShortWorkflow)
	if err != nil {
		ResponseErr(c, err)
		return
	}

	ctrl.notifyRunUpdate(updated)
	ctrl.respondRunSnapshot(ctx, c, updated)
}

func (ctrl *AgentRunController) respondRunStagePlan(c *gin.Context, run *model.AgentRun) {
	ctx := c.Request.Context()
	dispatched, err := ctrl.dispatchedStages(ctx, run.IdRun)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	plan, err := ctrl.stagePlan.Parse(run.StagePlan)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	gateway, err := ctrl.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if gateway == nil {
		gateway = &model.AgentGateway{Models: []string{}}
	}

	res := model.AgentRunStagePlanRes{
		Stages:            make([]model.AgentRunStagePlanEntry, 0, len(plan.Stages)),
		AgentModelChoices: gateway.ModelChoices(),
		MaxTurnsLimit:     gateway.MaxTurnsLimit,
	}
	res.DefaultsByStage = ctrl.agentTaskConfig.GetStageDefaults(gateway)
	res.ModelsWithoutThinking, err = ctrl.agentTaskConfig.ModelsWithoutThinking(ctx, gateway)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	for _, entry := range plan.Stages {
		if !constants.IsSkillStage(entry.Name) {
			continue
		}
		stage := model.AgentRunStagePlanEntry{
			Name:       entry.Name,
			IdsSkill:   entry.IdsSkill,
			Skip:       entry.Skip,
			Dispatched: dispatched[entry.Name],
		}
		if stage.IdsSkill == nil {
			stage.IdsSkill = []int64{}
		}
		if entry.Model != "" {
			stage.Model = &entry.Model
		}
		if entry.MaxTurns != 0 {
			stage.MaxTurns = &entry.MaxTurns
		}
		if entry.Effort != "" {
			stage.Effort = &entry.Effort
		}
		res.Stages = append(res.Stages, stage)
	}
	ResponseOk(c, res)
}

func (ctrl *AgentRunController) loadRunForStagePlan(c *gin.Context, isWrite bool) (*model.AgentRun, bool) {
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return nil, false
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		ResponseErr(c, errs.ErrNotFound)
		return nil, false
	}
	if err != nil {
		ResponseErr(c, err)
		return nil, false
	}

	hasAccess := ctrl.acl.CanReadProject(ctx, user.IdUser, run.IdProject)
	if isWrite {
		hasAccess = ctrl.acl.CanUpdateIssue(ctx, user.IdUser, run.IdProject)
	}
	if !hasAccess {
		ResponseErr(c, errs.ErrForbidden)
		return nil, false
	}
	return run, true
}

func (ctrl *AgentRunController) dispatchedStages(ctx context.Context, idRun int64) (map[string]bool, error) {
	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, idRun)
	if err != nil {
		return nil, fmt.Errorf("loading run tasks: %w", err)
	}
	return service.DispatchedStages(tasks), nil
}

// Stats returns aggregate counters across all agent_task rows for the run.
func (ctrl *AgentRunController) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.loadAgentRun(ctx, idRun)
	if err == repository.ErrRunNotFound {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !ctrl.acl.CanReadProject(ctx, user.IdUser, run.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	stats, err := ctrl.agentTaskRepo.StatsForRun(ctx, idRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// requireAssignedAgent authorizes a callback addressed by task id, writing the
// response when the caller is not the task's agent. A task that does not exist
// is a 404, not a refusal — a typo in a task id must not read as revoked access.
func (ctrl *AgentRunController) requireAssignedAgent(c *gin.Context, idTask, idUser int64) bool {
	isAgent, err := ctrl.tasks.IsAgentAssignedToTask(c.Request.Context(), idUser, idTask)
	switch {
	case errors.Is(err, repository.ErrTaskNotFound):
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case err != nil:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	case !isAgent:
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
	default:
		return true
	}
	return false
}

// CompleteStage is the callback endpoint both the gateway and the MCP
// complete_stage tool target.
func (ctrl *AgentRunController) CompleteStage(c *gin.Context) {
	ctx := c.Request.Context()
	idTask, err := strconv.ParseInt(c.Param("idTask"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}

	var body model.CompleteStageReq
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(errs.ErrValidation.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	task, err := ctrl.agentTaskRepo.LoadById(ctx, idTask)
	if err != nil {
		_ = c.Error(errs.ErrTaskNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	// A run token completes the task of its own turn and nothing else.
	if scope, isScoped := extctx.GetRunScope(ctx); isScoped && task.IdRun != scope.IdRun {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	run, err := ctrl.agentRunRepo.LoadById(ctx, task.IdRun)
	if err != nil {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	user, _ := extctx.GetUser(ctx)
	if user.IdUser != run.IdUserAgent {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if err := validateOutcomeForStage(body.Outcome, task.Stage); err != nil {
		_ = c.Error(errs.ErrBadRequest.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}
	res, err := ctrl.kinds.For(run).Complete(ctx, run, task, &body)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GatewayRecovered is called when a gateway (re)starts. Its in-flight
// subprocesses died with it, so every active task for this agent is orphaned —
// fail them and their runs so the user gets Continue/Restart instead of a
// silently-stuck run.
func (ctrl *AgentRunController) GatewayRecovered(c *gin.Context) {
	ctx := c.Request.Context()
	agentUser, ok := extctx.GetUser(ctx)
	if !ok {
		ResponseErr(c, errs.ErrUnauthorized)
		return
	}
	var report model.GatewayRecoveredReq
	if err := c.ShouldBindJSON(&report); err != nil && !errors.Is(err, io.EOF) {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	if report.Models != nil {
		if err := ctrl.agentGwRepo.SetModels(ctx, agentUser.IdUser, report); err != nil {
			ResponseErr(c, err)
			return
		}
	}

	failed, err := ctrl.agentTaskRepo.FailActiveForAgent(ctx, agentUser.IdUser)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	idsRun := map[int64]bool{}
	for _, failedTask := range failed {
		idsRun[failedTask.IdRun] = true
		if err := agent.InterruptFailedTask(ctx, ctrl.agentRunRepo, ctrl.agentTaskRepo, ctrl.kinds, failedTask, agent.InterruptGatewayRestart); err != nil {
			log.Error().Err(err).Int64("idRun", failedTask.IdRun).Msg("interrupting a run after a gateway restart")
		}
	}
	ResponseOk(c, gin.H{"failedRuns": len(idsRun)})
}

func (ctrl *AgentRunController) GatewayModelThinking(c *gin.Context) {
	ctx := c.Request.Context()
	agentUser, ok := extctx.GetUser(ctx)
	if !ok {
		ResponseErr(c, errs.ErrUnauthorized)
		return
	}
	var report model.GatewayModelThinkingReq
	if err := c.ShouldBindJSON(&report); err != nil {
		ResponseErr(c, errs.ErrValidation.WithMessage(err.Error()))
		return
	}
	if err := ctrl.agentGwRepo.SetModelThinking(ctx, agentUser.IdUser, report.IsThinkingByModel); err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, gin.H{"ok": true})
}

// TaskStats accepts a stats backfill from the gateway adapter and broadcasts
// an agent_run notice so the open stats panel refetches, and stores the commit a chat turn's worktree is at.
func (ctrl *AgentRunController) TaskStats(c *gin.Context) {
	ctx := c.Request.Context()
	idTask, err := strconv.ParseInt(c.Param("idTask"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	var body struct {
		TokensUsed       *int     `json:"tokensUsed"`
		DurationMs       *int     `json:"durationMs"`
		ToolCallsCount   *int     `json:"toolCallsCount"`
		InputTokens      *int     `json:"inputTokens"`
		OutputTokens     *int     `json:"outputTokens"`
		CacheReadTokens  *int     `json:"cacheReadTokens"`
		CacheWriteTokens *int     `json:"cacheWriteTokens"`
		CostUsd          *float64 `json:"costUsd"`
		CostEstimated    bool     `json:"costEstimated"`
		HeadCommit       *string  `json:"headCommit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(errs.ErrValidation.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}
	user, _ := extctx.GetUser(ctx)
	if !ctrl.requireAssignedAgent(c, idTask, user.IdUser) {
		return
	}
	if err := ctrl.agentTaskRepo.UpdateStats(ctx, idTask,
		body.TokensUsed, body.DurationMs, body.ToolCallsCount,
		body.InputTokens, body.OutputTokens, body.CacheReadTokens, body.CacheWriteTokens,
		body.CostUsd, body.CostEstimated,
	); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	// Broadcast the aggregated stats so an open run-stats panel patches
	// directly — no client refetch needed.
	if task, err := ctrl.agentTaskRepo.LoadById(ctx, idTask); err == nil {
		if run, err := ctrl.agentRunRepo.LoadById(ctx, task.IdRun); err == nil {
			report := &agent.TaskReport{}
			if body.HeadCommit != nil {
				report.HeadCommit = *body.HeadCommit
			}
			if err := ctrl.kinds.For(run).ApplyReport(ctx, run, task, report); err != nil {
				ResponseErr(c, err)
				return
			}
			if stats, err := ctrl.agentTaskRepo.StatsForRun(ctx, task.IdRun); err == nil {
				ctrl.sendStatsNotice(ctx, run, &model.AgentStatsNotice{IdRun: task.IdRun, Stats: stats})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// sendStatsNotice tells the run's audience about new stats; nothing when the audience cannot be resolved.
func (ctrl *AgentRunController) sendStatsNotice(ctx context.Context, run *model.AgentRun, payload *model.AgentStatsNotice) {
	idsUser, err := ctrl.kinds.For(run).Audience(ctx, run)
	if err != nil {
		return
	}
	if len(idsUser) == 0 {
		return
	}
	ctrl.notifier.Send(&notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectAgentStats,
		Action:  notify.ActionUpdate,
		Payload: payload,
	})
}

// TaskHeartbeat records that the active task is still alive.
func (ctrl *AgentRunController) TaskHeartbeat(c *gin.Context) {
	ctx := c.Request.Context()
	idTask, err := strconv.ParseInt(c.Param("idTask"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	user, _ := extctx.GetUser(ctx)
	if !ctrl.requireAssignedAgent(c, idTask, user.IdUser) {
		return
	}
	if err := ctrl.agentTaskRepo.RecordHeartbeat(ctx, idTask); err != nil {
		if errors.Is(err, repository.ErrTaskStatusMismatch) {
			_ = c.Error(errs.ErrTaskNotRunningByAgent)
			c.Status(errs.ErrTaskNotRunningByAgent.HttpStatus())
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func validateOutcomeForStage(outcome, stage string) error {
	switch outcome {
	case constants.StageOutcomeQuestionAsked:
		if stage != constants.StageBrainstorming {
			return fmt.Errorf("question_asked is only valid for brainstorming, got %s", stage)
		}
	case constants.StageOutcomeNoActionNeeded:
		if stage != constants.StageBrainstorming && stage != constants.StagePickup {
			return fmt.Errorf("no_action_needed is only valid for brainstorming and pickup, got %s", stage)
		}
	case constants.StageOutcomeOutputSubmitted, constants.StageOutcomeErrored:
		// allowed for any stage
	default:
		return fmt.Errorf("unknown outcome %q", outcome)
	}
	return nil
}

func (ctrl *AgentRunController) notifyRunUpdate(run *model.AgentRun) {
	agent.BroadcastRunUpdate(context.Background(), ctrl.notifier, ctrl.projectRepo, ctrl.issueRepo, ctrl.agentRunRepo, ctrl.agentTaskRepo, run)
}

// respondRunSnapshot returns the same snapshot shape as the GET endpoints and
// the websocket broadcast. Mutating endpoints must use this — the bare run
// drops `stages`, blanking the client's timeline until the next notice.
func (ctrl *AgentRunController) respondRunSnapshot(ctx context.Context, c *gin.Context, run *model.AgentRun) {
	events, err := ctrl.agentRunRepo.LoadEvents(ctx, run.IdRun)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	idAssignee, err := agent.LoadRunAssignee(ctx, ctrl.issueRepo, run)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, agent.BuildRunSnapshot(run, events, tasks, idAssignee))
}

// loadAgentRun loads a run for the issue and release run endpoints; a private run has its own
// endpoints and does not exist here.
func (ctrl *AgentRunController) loadAgentRun(ctx context.Context, idRun int64) (*model.AgentRun, error) {
	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
	if err != nil {
		return nil, err
	}
	if run.IsPrivate() {
		return nil, repository.ErrRunNotFound
	}
	return run, nil
}
