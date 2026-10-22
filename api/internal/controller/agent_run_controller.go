package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type AgentRunController struct {
	agentRunRepo      *repository.AgentRunRepository
	agentTaskRepo     *repository.AgentTaskRepository
	tasks             *agent.TaskService
	thinking          *agent.ThinkingService
	agentGwRepo       *repository.AgentGatewayRepository
	issueRepo         *repository.IssueRepository
	projectRepo       *repository.ProjectRepository
	messageRepo       *repository.MessageRepository
	attachmentLinkSvc *service.AttachmentLinkService
	gitIntRepo        *repository.GitIntegrationRepository
	acl               *service.AclService
	stagePlan         *service.AgentStagePlanService
	agentTaskConfig   *service.AgentTaskConfigService
	dispatcher        *agent.Dispatcher
	notifier          *notify.Notifier
	pool              *pgxpool.Pool
}

func NewAgentRunController(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	tasks *agent.TaskService,
	thinking *agent.ThinkingService,
	agentGwRepo *repository.AgentGatewayRepository,
	issueRepo *repository.IssueRepository,
	projectRepo *repository.ProjectRepository,
	messageRepo *repository.MessageRepository,
	attachmentLinkSvc *service.AttachmentLinkService,
	gitIntRepo *repository.GitIntegrationRepository,
	acl *service.AclService,
	stagePlan *service.AgentStagePlanService,
	agentTaskConfig *service.AgentTaskConfigService,
	dispatcher *agent.Dispatcher,
	notifier *notify.Notifier,
	pool *pgxpool.Pool,
) *AgentRunController {
	return &AgentRunController{
		agentRunRepo:      agentRunRepo,
		agentTaskRepo:     agentTaskRepo,
		tasks:             tasks,
		thinking:          thinking,
		agentGwRepo:       agentGwRepo,
		issueRepo:         issueRepo,
		projectRepo:       projectRepo,
		messageRepo:       messageRepo,
		attachmentLinkSvc: attachmentLinkSvc,
		gitIntRepo:        gitIntRepo,
		acl:               acl,
		stagePlan:         stagePlan,
		agentTaskConfig:   agentTaskConfig,
		dispatcher:        dispatcher,
		notifier:          notifier,
		pool:              pool,
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

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

	oldRun, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

	stagePlan, err := ctrl.stagePlan.BuildFromChoices(ctrl.stagePlan.Choices(oldRun.StagePlan))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	newRun, err := ctrl.agentRunRepo.Insert(ctx, oldRun.IdIssue, oldRun.IdUserAgent, oldRun.IdProject, stagePlan)
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
		Stages:        make([]model.AgentRunStagePlanEntry, 0, len(plan.Stages)),
		Models:        gateway.Models,
		ModelDefault:  gateway.ModelDefault,
		MaxTurnsLimit: gateway.MaxTurnsLimit,
	}
	res.MaxTurnsPlan, res.MaxTurnsImplement = ctrl.agentTaskConfig.GetDefaultMaxTurns(gateway)
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

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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

// errReconcileSuperseded signals a concurrent Restart/Continue moved the run
// out of `failed` between the pre-tx guard and the reconcile CAS. The tx rolls
// back and the caller responds 200 no-op, not 500.
var errReconcileSuperseded = errors.New("reconcile superseded by concurrent run transition")

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
	if err := validateMessageKind(body.Message, body.MessageKind); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}

	agentUser, _ := extctx.GetUser(ctx)

	// reconcilability gate. A still-live gateway may report completion for a
	// task an API/gateway restart already blanket-failed (crash recovery). Decide
	// how to treat the task's status BEFORE any host I/O, so we never create an
	// orphan PR for a completion we won't apply.
	switch {
	case task.Status == constants.TaskStatusActive:
		// normal path — fall through to resolve + apply
	case task.Status == constants.TaskStatusCompleted:
		// Idempotent no-op: a retried complete_stage after a successful one.
		// Return the terminal state instead of erroring.
		c.JSON(http.StatusOK, model.CompleteStageRes{IdTask: idTask, Status: task.Status, NextPhase: run.Phase})
		return
	case task.Status == constants.TaskStatusFailed && run.Phase == constants.PhaseFailed &&
		task.ErrorReason != nil && constants.RecoverableFailReasons[*task.ErrorReason]:
		// Reconcile a restart-orphaned task, but ONLY while the run is still `failed`.
		// If Continue already moved it out, or the user Restarted (a newer
		// non-terminal run exists), reconciling would resurrect/duplicate: no-op.
		// (The in-tx CAS is the final authority for the race after this check.)
		if active, aerr := ctrl.agentRunRepo.LoadActiveByIssue(ctx, run.IdIssue); aerr == nil && active != nil && active.IdRun != run.IdRun {
			c.JSON(http.StatusOK, model.CompleteStageRes{IdTask: idTask, Status: task.Status, NextPhase: run.Phase})
			return
		}
	default:
		// Genuine failure or cancellation: no-op, no PR — don't resurrect an
		// abandoned run.
		c.JSON(http.StatusOK, model.CompleteStageRes{IdTask: idTask, Status: task.Status, NextPhase: run.Phase})
		return
	}
	reconcile := task.Status == constants.TaskStatusFailed

	if isImplementationWithoutPr(run, task, &body) {
		_ = c.Error(errs.ErrImplementationNoBranch)
		c.Status(errs.ErrImplementationNoBranch.HttpStatus())
		return
	}

	// Resolve the PR OUTSIDE the DB transaction — it makes git-host HTTP calls
	// and must not hold a transaction open. A host failure converts to an
	// `errored` outcome so the normal failure path records it.
	pr := ctrl.resolvePrForCompletion(ctx, run, task, &body)

	var res completeStageResult
	txErr := extctx.RunInTx(ctx, ctrl.pool, func(ctx context.Context) error {
		var err error
		res, err = ctrl.applyCompleteStage(ctx, run, task, &body, pr, reconcile, &agentUser)
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, errReconcileSuperseded) {
			// A concurrent Restart/Continue moved the run out of `failed` mid-reconcile;
			// the tx rolled back — respond 200 no-op.
			c.JSON(http.StatusOK, model.CompleteStageRes{IdTask: idTask, Status: task.Status, NextPhase: run.Phase})
			return
		}
		if errors.Is(txErr, errs.ErrAttachmentLinkInvalid) {
			ResponseErr(c, txErr)
			return
		}
		_ = c.Error(errs.ErrInternal.WithMessage(txErr.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}

	ctrl.thinking.Compact(ctx, idTask)

	members, _ := ctrl.projectRepo.LoadProjectsMembers(ctx, []int64{run.IdProject})
	var idsUser []int64
	if len(members) > 0 {
		idsUser = make([]int64, len(members))
		for i, m := range members {
			idsUser[i] = m.IdUser
		}
	}

	if res.notifierMsg != nil {
		ctrl.notifier.Send <- &notify.Notice{
			IdsUser: idsUser,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionCreate,
			Payload: res.notifierMsg,
			Source:  "agent",
		}
	}
	// The local `run` is the pre-transition snapshot; reload so the broadcast
	// carries the phase applyCompleteStage set.
	if updated, loadErr := ctrl.agentRunRepo.LoadById(ctx, run.IdRun); loadErr == nil {
		ctrl.notifyRunUpdate(updated)
	}
	// applyCompleteStage mutates the issue out-of-band (PR link, phase→state
	// mirror); push the fresh issue so the detail page updates live.
	agent.BroadcastIssueUpdate(context.Background(), ctrl.notifier, ctrl.issueRepo, ctrl.projectRepo, run.IdIssue)

	c.JSON(http.StatusOK, model.CompleteStageRes{
		IdTask:    idTask,
		Status:    res.targetStatus,
		NextPhase: res.nextPhase,
	})
}

func (ctrl *AgentRunController) applyCompleteStage(
	ctx context.Context,
	run *model.AgentRun,
	task *model.AgentTask,
	body *model.CompleteStageReq,
	pr *prComputed,
	reconcile bool,
	agentUser *model.User,
) (completeStageResult, error) {
	var res completeStageResult
	var idMessage *int64
	if body.Message != "" {
		msg, err := ctrl.messageRepo.InsertIssueAgentMessage(
			ctx, body.Message, agentUser, run.IdIssue, constants.MessageKind(body.MessageKind),
		)
		if err != nil {
			return completeStageResult{}, fmt.Errorf("writing message: %w", err)
		}
		if err := ctrl.attachmentLinkSvc.LinkMessageAttachments(ctx, msg); err != nil {
			return completeStageResult{}, fmt.Errorf("linking message attachments: %w", err)
		}
		id := msg.IdMessage
		idMessage = &id
		res.notifierMsg = msg
	}

	prWritten := false
	if pr != nil {
		if err := ctrl.writePrInfo(ctx, run, pr, body.BranchName); err != nil {
			// The run may have been moved out of `failed` by a concurrent
			// Restart/Continue between the pre-tx guard and this CAS — treat that
			// as a superseded no-op (roll back, respond 200), not a 500.
			if reconcile && errors.Is(err, repository.ErrPhaseMismatch) {
				return completeStageResult{}, errReconcileSuperseded
			}
			return completeStageResult{}, fmt.Errorf("applying PR info: %w", err)
		}
		prWritten = true
	}

	targetStatus := constants.TaskStatusCompleted
	if body.Outcome == constants.StageOutcomeErrored {
		targetStatus = constants.TaskStatusFailed
	}
	if err := ctrl.agentTaskRepo.SetResultAndStats(ctx, task.IdTask, idMessage, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
		return completeStageResult{}, err
	}
	// CompleteReconcilable accepts the task from `active` (normal completion) or
	// from `failed` with a recoverable error_reason (a late completion reconciled
	// after a restart orphaned it); genuine/cancelled tasks never reach
	// here (gated in CompleteStage). MUST run BEFORE SetError: SetError overwrites
	// error_reason, which could push the row out of the recoverable allowlist and
	// mismatch this CAS.
	if _, err := ctrl.agentTaskRepo.CompleteReconcilable(ctx, task.IdTask, targetStatus, constants.RecoverableFailReasonList); err != nil {
		// A concurrent complete_stage (dual caller: gateway + MCP tool, or a retry)
		// already completed or reconciled this task after our pre-tx gate. The
		// winner is recording the work — no-op this call instead of 500.
		if errors.Is(err, repository.ErrTaskStatusMismatch) {
			return completeStageResult{}, errReconcileSuperseded
		}
		return completeStageResult{}, fmt.Errorf("transitioning task status: %w", err)
	}
	// Record the error reason/detail AFTER the status CAS (errored outcome only).
	if body.Outcome == constants.StageOutcomeErrored {
		reason := "agent_reported_error"
		if body.ErrorReason != nil {
			reason = *body.ErrorReason
		}
		detail := ""
		if body.ErrorDetail != nil {
			detail = *body.ErrorDetail
		}
		if err := ctrl.agentTaskRepo.SetError(ctx, task.IdTask, reason, detail); err != nil {
			return completeStageResult{}, err
		}
	}

	nextPhase := decideNextRunPhase(task.Stage, body.Outcome)
	// When PR info was written, SetPrInfoFrom already performed the phase transition
	// to pr_open; a second one here would fail the phase guard.
	if !prWritten {
		if reconcile && !constants.TerminalPhases[nextPhase] {
			// Reconcile a crash-orphaned run: move it out of `failed` back into the
			// pipeline, clearing the stale failure stamp. If a concurrent
			// Restart/Continue already moved the run, the CAS mismatches →
			// superseded no-op, not a partial commit.
			if _, err := ctrl.agentRunRepo.ReconcileToPhase(ctx, run.IdRun, run.Phase, nextPhase, constants.ActorTypeAgent, "complete_stage:reconcile:"+body.Outcome); err != nil {
				if errors.Is(err, repository.ErrPhaseMismatch) {
					return completeStageResult{}, errReconcileSuperseded
				}
				return completeStageResult{}, fmt.Errorf("reconcile phase transition: %w", err)
			}
		} else if _, err := ctrl.agentRunRepo.TransitionPhase(ctx, run.IdRun, run.Phase, nextPhase, constants.ActorTypeAgent, nil, "complete_stage:"+body.Outcome); err != nil {
			log.Debug().Err(err).Int64("idRun", run.IdRun).Str("toPhase", nextPhase).Msg("phase transition tolerated")
		}
	}

	return completeStageResult{notifierMsg: res.notifierMsg, targetStatus: targetStatus, nextPhase: nextPhase}, nil
}

type completeStageResult struct {
	notifierMsg  *model.Message
	targetStatus string
	nextPhase    string
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

	runIds, err := ctrl.agentTaskRepo.FailActiveForAgent(ctx, agentUser.IdUser)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	for _, idRun := range runIds {
		agent.FailRun(ctx, ctrl.notifier, ctrl.projectRepo, ctrl.issueRepo, ctrl.agentRunRepo, ctrl.agentTaskRepo, idRun, "gateway_restart")
	}
	ResponseOk(c, gin.H{"failedRuns": len(runIds)})
}

// TaskStats accepts a stats backfill from the gateway adapter and broadcasts
// an agent_run notice so the open stats panel refetches.
func (ctrl *AgentRunController) TaskStats(c *gin.Context) {
	ctx := c.Request.Context()
	idTask, err := strconv.ParseInt(c.Param("idTask"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	var body struct {
		TokensUsed     *int `json:"tokensUsed"`
		DurationMs     *int `json:"durationMs"`
		ToolCallsCount *int `json:"toolCallsCount"`
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
	if err := ctrl.agentTaskRepo.UpdateStats(ctx, idTask, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	// Broadcast the aggregated stats so an open run-stats panel patches
	// directly — no client refetch needed.
	if task, err := ctrl.agentTaskRepo.LoadById(ctx, idTask); err == nil {
		if run, err := ctrl.agentRunRepo.LoadById(ctx, task.IdRun); err == nil {
			if stats, err := ctrl.agentTaskRepo.StatsForRun(ctx, task.IdRun); err == nil {
				members, _ := ctrl.projectRepo.LoadProjectsMembers(ctx, []int64{run.IdProject})
				var idsUser []int64
				if len(members) > 0 {
					idsUser = make([]int64, len(members))
					for i, m := range members {
						idsUser[i] = m.IdUser
					}
				}
				ctrl.notifier.Send <- &notify.Notice{
					IdsUser: idsUser,
					Subject: notify.SubjectAgentStats,
					Action:  notify.ActionUpdate,
					Payload: &model.AgentStatsNotice{IdRun: task.IdRun, Stats: stats},
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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

func validateMessageKind(message, kind string) error {
	if message == "" {
		return nil
	}
	allowed := strings.Join(constants.AgentMessageKinds, ", ")
	switch {
	case slices.Contains(constants.AgentMessageKinds, kind):
		return nil
	case kind == "":
		return fmt.Errorf("messageKind is required when message is set, use one of: %s", allowed)
	default:
		return fmt.Errorf("unknown messageKind %q, use one of: %s", kind, allowed)
	}
}

func decideNextRunPhase(stage, outcome string) string {
	if outcome == constants.StageOutcomeErrored {
		return constants.PhaseFailed
	}
	if outcome == constants.StageOutcomeNoActionNeeded {
		return constants.PhaseInProgress
	}
	switch stage {
	case constants.StagePickup:
		return constants.PhaseInProgress
	case constants.StageBrainstorming:
		if outcome == constants.StageOutcomeQuestionAsked {
			return constants.PhaseAwaitingInput
		}
		return constants.PhaseInProgress
	case constants.StageDesign, constants.StageImplementationPlan:
		return constants.PhaseAwaitingApproval
	case constants.StageImplementation:
		return constants.PhasePrOpen
	default:
		return constants.PhaseInProgress
	}
}

// prComputed holds the PR identity resolved (created or found) on the git host
// before the DB transaction.
type prComputed struct {
	prId             string
	prUrl            string
	hostType         string
	idGitIntegration int64
}

func isImplementationWithoutPr(run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) bool {
	if task.Stage != constants.StageImplementation || body.Outcome != constants.StageOutcomeOutputSubmitted {
		return false
	}
	if body.BranchName != "" || body.PrUrl != "" {
		return false
	}
	return run.PrId == nil || *run.PrId == ""
}

// resolvePrForCompletion resolves PR identity for an implementation completion,
// outside any DB transaction (it makes git-host HTTP calls). Returns nil when no
// PR applies; on host failure it converts body to an `errored` outcome so the
// normal failure path records it.
func (ctrl *AgentRunController) resolvePrForCompletion(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) *prComputed {
	if task.Stage != constants.StageImplementation || body.Outcome != constants.StageOutcomeOutputSubmitted {
		return nil
	}
	var pr *prComputed
	var err error
	switch {
	case body.PrUrl != "":
		// Deprecated path: agent supplied a URL directly. Guard and use it.
		var hostType, prId string
		hostType, prId, err = derivePrHostAndId(body.PrUrl)
		if err == nil {
			var idGi int64
			if idGi, err = ctrl.resolveIntegrationId(ctx, run); err == nil {
				pr = &prComputed{prId: prId, prUrl: body.PrUrl, hostType: hostType, idGitIntegration: idGi}
			}
		}
	case body.BranchName != "":
		pr, err = ctrl.createOrFindPr(ctx, run, body)
	default:
		return nil // no branch, no URL — nothing to open
	}
	if err != nil {
		log.Error().Err(err).Int64("idRun", run.IdRun).Msg("PR creation failed; marking stage errored")
		body.Outcome = constants.StageOutcomeErrored
		reason := "pr_creation_failed"
		detail := err.Error()
		body.ErrorReason = &reason
		body.ErrorDetail = &detail
		return nil
	}
	return pr
}

// createOrFindPr opens (or reuses, for a re-invoked run) the PR/MR for the run's
// pushed branch. Host I/O — must be called OUTSIDE the DB transaction.
func (ctrl *AgentRunController) createOrFindPr(ctx context.Context, run *model.AgentRun, body *model.CompleteStageReq) (*prComputed, error) {
	idGi, err := ctrl.resolveIntegrationId(ctx, run)
	if err != nil {
		return nil, err
	}
	integration, err := ctrl.gitIntRepo.LoadByID(ctx, idGi, run.IdProject)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("git_integration %d not found", idGi)
	}
	host, err := githost.BuildFromIntegration(integration)
	if err != nil {
		return nil, err
	}
	title := body.PrTitle
	if title == "" {
		title = ctrl.fallbackPrTitle(ctx, run.IdIssue, body.BranchName)
	}
	prId, prUrl, err := openOrReusePr(ctx, host, run.BaseBranch, body.BranchName, title, body.PrBody)
	if err != nil {
		return nil, err
	}
	return &prComputed{prId: prId, prUrl: prUrl, hostType: integration.HostType, idGitIntegration: integration.IdGitIntegration}, nil
}

// openOrReusePr reuses an open PR/MR for branch if one exists, otherwise opens a
// new one against baseBranch, or the repo's default branch for a run dispatched
// before base branches were stamped on runs.
func openOrReusePr(ctx context.Context, host githost.GitHost, baseBranch *string, branch, title, body string) (string, string, error) {
	prId, prUrl, found, err := host.FindOpenPullRequest(ctx, branch)
	if err != nil {
		return "", "", fmt.Errorf("looking up existing PR: %w", err)
	}
	if found {
		return prId, prUrl, nil
	}
	base, err := prTargetBranch(ctx, host, baseBranch)
	if err != nil {
		return "", "", err
	}
	prId, prUrl, err = host.CreatePullRequest(ctx, branch, base, title, body)
	if err != nil {
		return "", "", fmt.Errorf("opening PR: %w", err)
	}
	return prId, prUrl, nil
}

// resolveIntegrationId returns the git_integration to use for PR creation: the
// one stamped on the run at dispatch, or the project's only integration for a run
// dispatched before that.
func (ctrl *AgentRunController) resolveIntegrationId(ctx context.Context, run *model.AgentRun) (int64, error) {
	if run.IdGitIntegration != nil {
		return *run.IdGitIntegration, nil
	}
	gitInts, err := ctrl.gitIntRepo.ListByProject(ctx, run.IdProject)
	if err != nil {
		return 0, err
	}
	if len(gitInts) == 1 {
		return gitInts[0].IdGitIntegration, nil
	}
	return 0, fmt.Errorf("no git_integration resolved for the pushed repo (run %d, project has %d integrations)", run.IdRun, len(gitInts))
}

func prTargetBranch(ctx context.Context, host githost.GitHost, baseBranch *string) (string, error) {
	if baseBranch != nil && *baseBranch != "" {
		return *baseBranch, nil
	}
	base, err := host.DefaultBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving base branch: %w", err)
	}
	return base, nil
}

// fallbackPrTitle is used only when the agent did not supply a pr_title.
func (ctrl *AgentRunController) fallbackPrTitle(ctx context.Context, idIssue int64, branchName string) string {
	issue, err := ctrl.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &idIssue})
	if err == nil && issue != nil && issue.Title != "" {
		return issue.Title
	}
	return "Agent changes (" + branchName + ")"
}

// writePrInfo persists the resolved PR onto the run (flipping its phase →
// pr_open) and mirrors it onto the issue. DB-only — safe inside the transaction.
// Uses run.Phase as the CAS from-phase: `in_progress` normally, `failed` when
// reconciling a crash-orphaned run, in which case SetPrInfoFrom clears
// the stale failure stamp too.
func (ctrl *AgentRunController) writePrInfo(ctx context.Context, run *model.AgentRun, pr *prComputed, branchName string) error {
	dto := model.SetRunPrReq{
		PrUrl:            pr.prUrl,
		PrId:             pr.prId,
		PrHostType:       pr.hostType,
		BranchName:       branchName,
		IdGitIntegration: pr.idGitIntegration,
	}
	if _, err := ctrl.agentRunRepo.SetPrInfoFrom(ctx, run.IdRun, dto, run.Phase); err != nil {
		return err
	}
	// Mirror onto issues.issue.{id_git_integration, mr_id} best-effort.
	_ = ctrl.issueRepo.LinkMr(ctx, run.IdIssue, dto.IdGitIntegration, dto.PrId)
	return nil
}

// derivePrHostAndId parses a PR URL into (host_type, pr_id). The path tail
// varies by host (/pull/N vs /merge_requests/N), so it checks host first,
// then path.
func derivePrHostAndId(prUrl string) (hostType, prId string, err error) {
	u, parseErr := url.Parse(prUrl)
	if parseErr != nil {
		return "", "", fmt.Errorf("invalid pr_url: %w", parseErr)
	}
	host := strings.ToLower(u.Host)

	switch {
	case strings.Contains(host, "github."):
		hostType = "github"
	case strings.Contains(host, "gitlab."):
		hostType = "gitlab"
	case strings.Contains(host, "gitea."):
		hostType = "gitea"
	default:
		hostType = ""
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, seg := range segments {
		if (seg == "pull" || seg == "merge_requests" || seg == "pulls") &&
			i+1 < len(segments) {
			prId = segments[i+1]
			if hostType == "" {
				if seg == "merge_requests" {
					hostType = "gitlab"
				} else {
					hostType = "github"
				}
			}
			break
		}
	}
	if prId == "" {
		return "", "", fmt.Errorf("could not extract pr_id from %s — expected /pull/<n> or /merge_requests/<n>", prUrl)
	}
	if !isAllDigits(prId) {
		return "", "", fmt.Errorf("pr_id %q is not numeric in %s — got a PR-creation/compare URL, not an actual PR", prId, prUrl)
	}
	if hostType == "" {
		return "", "", fmt.Errorf("could not infer pr_host_type from %s — host must contain github/gitlab/gitea", prUrl)
	}
	return hostType, prId, nil
}

// isAllDigits reports whether s is non-empty and consists only of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
	idAssignee, err := ctrl.issueRepo.LoadAssignee(ctx, run.IdIssue)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, agent.BuildRunSnapshot(run, events, tasks, idAssignee))
}
