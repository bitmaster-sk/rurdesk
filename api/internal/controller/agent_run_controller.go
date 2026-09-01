package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	agentRunRepo  *repository.AgentRunRepository
	agentTaskRepo *repository.AgentTaskRepository
	botGwRepo     *repository.BotGatewayRepository
	issueRepo     *repository.IssueRepository
	projectRepo   *repository.ProjectRepository
	messageRepo   *repository.MessageRepository
	gitIntRepo    *repository.GitIntegrationRepository
	acl           *service.AclService
	stagePlan     *service.StagePlanService
	dispatcher    *agent.Dispatcher
	notifier      *notify.Notifier
	pool          *pgxpool.Pool
}

func NewAgentRunController(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	botGwRepo *repository.BotGatewayRepository,
	issueRepo *repository.IssueRepository,
	projectRepo *repository.ProjectRepository,
	messageRepo *repository.MessageRepository,
	gitIntRepo *repository.GitIntegrationRepository,
	acl *service.AclService,
	stagePlan *service.StagePlanService,
	dispatcher *agent.Dispatcher,
	notifier *notify.Notifier,
	pool *pgxpool.Pool,
) *AgentRunController {
	return &AgentRunController{
		agentRunRepo:  agentRunRepo,
		agentTaskRepo: agentTaskRepo,
		botGwRepo:     botGwRepo,
		issueRepo:     issueRepo,
		projectRepo:   projectRepo,
		messageRepo:   messageRepo,
		gitIntRepo:    gitIntRepo,
		acl:           acl,
		stagePlan:     stagePlan,
		dispatcher:    dispatcher,
		notifier:      notifier,
		pool:          pool,
	}
}

func (ctrl *AgentRunController) GetRun(c *gin.Context) {
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

	if !ctrl.acl.CanReadProject(ctx, user.IdUser, run.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	events, err := ctrl.agentRunRepo.LoadEvents(ctx, idRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, idRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, agent.BuildRunSnapshot(run, events, tasks))
}

func (ctrl *AgentRunController) GetRunByIssue(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idIssuePublic, err := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
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

	issue, err := ctrl.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: &idIssuePublic})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	run, err := ctrl.agentRunRepo.LoadActiveByIssue(ctx, issue.IdIssue)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if run == nil {
		run, err = ctrl.agentRunRepo.LoadLatestByIssue(ctx, issue.IdIssue)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	if run == nil {
		c.Status(http.StatusNoContent)
		return
	}

	events, err := ctrl.agentRunRepo.LoadEvents(ctx, run.IdRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, agent.BuildRunSnapshot(run, events, tasks))
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
	ctrl.respondRunSnapshot(c, ctx, updated)
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
	if err := ctrl.agentTaskRepo.CancelNonTerminalForRun(ctx, idRun); err != nil {
		log.Warn().Err(err).Int64("idRun", idRun).Msg("cancel: failed to cancel non-terminal tasks")
	}

	go func() {
		_ = ctrl.dispatcher.DispatchCancelled(context.Background(), run)
	}()

	ctrl.notifyRunUpdate(updated)
	ctrl.respondRunSnapshot(c, ctx, updated)
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
// run for the same issue/bot/project. Message log is preserved.
func (ctrl *AgentRunController) Restart(c *gin.Context) {
	ctx := c.Request.Context()
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	user, _ := extctx.GetUser(ctx)

	oldRun, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
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
	if !ctrl.acl.CanUpdateIssue(ctx, user.IdUser, oldRun.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// refuse to restart a run that already has a PR — it would push a new
	// branch and open a duplicate PR while orphaning the existing one. Guard on
	// pr_id, not phase: `phase == pr_open` alone misses edge phases.
	if oldRun.PrId != nil && *oldRun.PrId != "" {
		_ = c.Error(errs.ErrRunHasPr)
		c.Status(http.StatusConflict)
		return
	}

	idUser := user.IdUser
	if !constants.TerminalPhases[oldRun.Phase] {
		_, _ = ctrl.agentRunRepo.TransitionPhase(ctx, idRun, oldRun.Phase, constants.PhaseCancelled, constants.ActorTypeUser, &idUser, "restart")
	}
	if err := ctrl.agentTaskRepo.CancelNonTerminalForRun(ctx, idRun); err != nil {
		log.Warn().Err(err).Int64("idRun", idRun).Msg("restart: failed to cancel non-terminal tasks")
	}

	stagePlan, err := ctrl.stagePlan.Build(ctrl.stagePlan.IdsSkillByStage(oldRun.StagePlan))
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	newRun, err := ctrl.agentRunRepo.Insert(ctx, oldRun.IdIssue, oldRun.IdUserBot, oldRun.IdProject, stagePlan)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	ctrl.notifyRunUpdate(newRun)
	c.JSON(http.StatusOK, gin.H{"oldIdRun": idRun, "newIdRun": newRun.IdRun})
}

func (ctrl *AgentRunController) GetSkills(c *gin.Context) {
	run, ok := ctrl.loadRunForSkills(c, false)
	if !ok {
		return
	}
	ctrl.respondRunSkills(c, run)
}

func (ctrl *AgentRunController) PatchSkills(c *gin.Context) {
	ctx := c.Request.Context()
	run, ok := ctrl.loadRunForSkills(c, true)
	if !ok {
		return
	}

	var dto model.UpdateAgentRunStageSkillsReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(errs.ErrValidation.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	dispatched, err := ctrl.dispatchedStages(ctx, run.IdRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	// The stage's prompt was already built and sent, so its skills are settled.
	if dispatched[dto.Stage] {
		_ = c.Error(errs.ErrSkillStageDispatched)
		c.Status(http.StatusConflict)
		return
	}

	updated, err := ctrl.stagePlan.SetStageSkills(ctx, run.IdRun, dto.Stage, dto.IdsSkill)
	if errors.Is(err, errs.ErrStageNotInPlan) {
		_ = c.Error(errs.ErrStageNotInPlan)
		c.Status(errs.ErrStageNotInPlan.HttpStatus())
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	ctrl.respondRunSkills(c, updated)
}

func (ctrl *AgentRunController) respondRunSkills(c *gin.Context, run *model.AgentRun) {
	dispatched, err := ctrl.dispatchedStages(c.Request.Context(), run.IdRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	plan, err := ctrl.stagePlan.Parse(run.StagePlan)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	stages := make([]model.AgentRunStageSkills, 0, len(plan.Stages))
	for _, entry := range plan.Stages {
		if !constants.IsSkillStage(entry.Name) {
			continue
		}
		idsSkill := entry.IdsSkill
		if idsSkill == nil {
			idsSkill = []int64{}
		}
		stages = append(stages, model.AgentRunStageSkills{
			Name:       entry.Name,
			IdsSkill:   idsSkill,
			Dispatched: dispatched[entry.Name],
		})
	}
	c.JSON(http.StatusOK, stages)
}

func (ctrl *AgentRunController) loadRunForSkills(c *gin.Context, isWrite bool) (*model.AgentRun, bool) {
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return nil, false
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
	if err == repository.ErrRunNotFound {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return nil, false
	}

	hasAccess := ctrl.acl.CanReadProject(ctx, user.IdUser, run.IdProject)
	if isWrite {
		hasAccess = ctrl.acl.CanUpdateIssue(ctx, user.IdUser, run.IdProject)
	}
	if !hasAccess {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return nil, false
	}
	return run, true
}

func (ctrl *AgentRunController) dispatchedStages(ctx context.Context, idRun int64) (map[string]bool, error) {
	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, idRun)
	if err != nil {
		return nil, fmt.Errorf("loading run tasks: %w", err)
	}
	dispatched := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		dispatched[task.Stage] = true
	}
	return dispatched, nil
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

// requireRunBot asserts the caller is the bot executing this run, and writes the
// response when it is not.
//
// The gateway callbacks sit in the ordinary authenticated group, which accepts
// any user JWT — the API key is not a distinct principal — so this match against
// run.IdUserBot is the whole authorization boundary. A human can never match it,
// since IdUserBot always points at a bot account.
func (ctrl *AgentRunController) requireRunBot(c *gin.Context, idUserBot int64) bool {
	caller, ok := extctx.GetUser(c.Request.Context())
	if !ok {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusUnauthorized)
		return false
	}
	if caller.IdUser != idUserBot {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return false
	}
	return true
}

// requireTaskBot is requireRunBot for callbacks addressed by task id, resolving
// the owning bot in one indexed lookup rather than loading task and run.
func (ctrl *AgentRunController) requireTaskBot(c *gin.Context, idTask int64) bool {
	idUserBot, err := ctrl.agentTaskRepo.BotForTask(c.Request.Context(), idTask)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			_ = c.Error(errs.ErrTaskNotFound)
			c.Status(http.StatusNotFound)
			return false
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return false
	}
	return ctrl.requireRunBot(c, idUserBot)
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
	if !ctrl.requireRunBot(c, run.IdUserBot) {
		return
	}

	if err := validateOutcomeForStage(body.Outcome, task.Stage); err != nil {
		_ = c.Error(errs.ErrBadRequest.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	botUser, _ := extctx.GetUser(ctx)

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

	// Resolve the PR OUTSIDE the DB transaction — it makes git-host HTTP calls
	// and must not hold a transaction open. A host failure converts to an
	// `errored` outcome so the normal failure path records it.
	pr := ctrl.resolvePrForCompletion(ctx, run, task, &body)

	var idMessageOut *int64
	var notifierMsg *model.Message
	var targetStatus, nextPhase string
	txErr := extctx.RunInTx(ctx, ctrl.pool, func(ctx context.Context) error {
		return ctrl.applyCompleteStage(ctx, run, task, &body, pr, reconcile, &botUser, &idMessageOut, &notifierMsg, &targetStatus, &nextPhase)
	})
	if txErr != nil {
		if errors.Is(txErr, errReconcileSuperseded) {
			// A concurrent Restart/Continue moved the run out of `failed` mid-reconcile;
			// the tx rolled back — respond 200 no-op.
			c.JSON(http.StatusOK, model.CompleteStageRes{IdTask: idTask, Status: task.Status, NextPhase: run.Phase})
			return
		}
		_ = c.Error(errs.ErrInternal.WithMessage(txErr.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}

	members, _ := ctrl.projectRepo.LoadProjectsMembers(ctx, []int64{run.IdProject})
	var idsUser []int64
	if len(members) > 0 {
		idsUser = make([]int64, len(members))
		for i, m := range members {
			idsUser[i] = m.IdUser
		}
	}

	if notifierMsg != nil {
		ctrl.notifier.Send <- &notify.Notice{
			IdsUser: idsUser,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionCreate,
			Payload: notifierMsg,
			Source:  "bot",
		}
	}
	ctrl.notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectAgentTask,
		Action:  notify.ActionUpdate,
		Payload: map[string]any{"idTask": idTask, "idRun": run.IdRun, "status": targetStatus},
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
		Status:    targetStatus,
		NextPhase: nextPhase,
	})
}

func (ctrl *AgentRunController) applyCompleteStage(
	ctx context.Context,
	run *model.AgentRun,
	task *model.AgentTask,
	body *model.CompleteStageReq,
	pr *prComputed,
	reconcile bool,
	botUser *model.User,
	idMessageOut **int64,
	notifierMsgOut **model.Message,
	targetStatusOut *string,
	nextPhaseOut *string,
) error {
	if body.Message != "" {
		msg, err := ctrl.messageRepo.InsertIssueAgentMessage(
			ctx, body.Message, botUser, run.IdIssue, constants.MessageKind(body.MessageKind),
		)
		if err != nil {
			return fmt.Errorf("writing message: %w", err)
		}
		id := msg.IdMessage
		*idMessageOut = &id
		*notifierMsgOut = msg
	}

	prWritten := false
	if pr != nil {
		if err := ctrl.writePrInfo(ctx, run, pr, body.BranchName); err != nil {
			// The run may have been moved out of `failed` by a concurrent
			// Restart/Continue between the pre-tx guard and this CAS — treat that
			// as a superseded no-op (roll back, respond 200), not a 500.
			if reconcile && errors.Is(err, repository.ErrPhaseMismatch) {
				return errReconcileSuperseded
			}
			return fmt.Errorf("applying PR info: %w", err)
		}
		prWritten = true
	}

	targetStatus := constants.TaskStatusCompleted
	if body.Outcome == constants.StageOutcomeErrored {
		targetStatus = constants.TaskStatusFailed
	}
	if err := ctrl.agentTaskRepo.SetOutputAndStats(ctx, task.IdTask, *idMessageOut, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
		return err
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
			return errReconcileSuperseded
		}
		return fmt.Errorf("transitioning task status: %w", err)
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
			return err
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
					return errReconcileSuperseded
				}
				return fmt.Errorf("reconcile phase transition: %w", err)
			}
		} else if _, err := ctrl.agentRunRepo.TransitionPhase(ctx, run.IdRun, run.Phase, nextPhase, constants.ActorTypeAgent, nil, "complete_stage:"+body.Outcome); err != nil {
			log.Debug().Err(err).Int64("idRun", run.IdRun).Str("toPhase", nextPhase).Msg("phase transition tolerated")
		}
	}

	*targetStatusOut = targetStatus
	*nextPhaseOut = nextPhase
	return nil
}

// GatewayRecovered is called when a gateway (re)starts. Its in-flight
// subprocesses died with it, so every active task for this bot is orphaned —
// fail them and their runs so the user gets Continue/Restart instead of a
// silently-stuck run.
func (ctrl *AgentRunController) GatewayRecovered(c *gin.Context) {
	ctx := c.Request.Context()
	bot, ok := extctx.GetUser(ctx)
	if !ok {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusUnauthorized)
		return
	}
	runIds, err := ctrl.agentTaskRepo.FailActiveForBot(ctx, bot.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	for _, idRun := range runIds {
		agent.FailRun(ctx, ctrl.notifier, ctrl.projectRepo, ctrl.agentRunRepo, ctrl.agentTaskRepo, idRun, "gateway_restart")
	}
	c.JSON(http.StatusOK, gin.H{"failedRuns": len(runIds)})
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
	if !ctrl.requireTaskBot(c, idTask) {
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

// ReportRunRepo records the repo a run pushes to (gateway-supplied, non-LLM) and
// resolves the matching git_integration onto the run, so PR creation later knows
// which credentials/repo to use. A zero or ambiguous match is left null — the
// complete path then fails clearly rather than guessing.
func (ctrl *AgentRunController) ReportRunRepo(c *gin.Context) {
	ctx := c.Request.Context()
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	var body model.ReportRunRepoReq
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(errs.ErrValidation.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}
	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
	if err != nil {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if !ctrl.requireRunBot(c, run.IdUserBot) {
		return
	}

	gitInts, err := ctrl.gitIntRepo.ListByProject(ctx, run.IdProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	match := uniqueIntegrationForRepo(gitInts, body.RepoPath)
	if match == nil {
		log.Warn().Int64("idRun", idRun).Str("repoPath", body.RepoPath).Int("candidates", len(gitInts)).
			Msg("no unique git_integration matched the pushed repo; leaving run.id_git_integration null")
		c.JSON(http.StatusOK, gin.H{"matched": false})
		return
	}
	if err := ctrl.agentRunRepo.SetGitIntegration(ctx, idRun, match.IdGitIntegration); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"matched": true})
}

// uniqueIntegrationForRepo returns the single integration whose repo_path matches
// repoPath (case-insensitive, slash-trimmed), or nil if zero or more than one
// match.
func uniqueIntegrationForRepo(gitInts []*model.GitIntegration, repoPath string) *model.GitIntegration {
	want := normalizeRepoPath(repoPath)
	var found *model.GitIntegration
	for _, gi := range gitInts {
		if normalizeRepoPath(gi.RepoPath) == want {
			if found != nil {
				return nil // ambiguous
			}
			found = gi
		}
	}
	return found
}

// normalizeRepoPath canonicalizes a repo slug for comparison — lower-cased,
// slash-trimmed, trailing ".git" removed — so "org/repo.git" and "org/repo"
// compare equal.
func normalizeRepoPath(p string) string {
	p = strings.TrimSuffix(strings.Trim(strings.ToLower(strings.TrimSpace(p)), "/"), ".git")
	return strings.Trim(p, "/")
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
	if !ctrl.requireTaskBot(c, idTask) {
		return
	}
	if err := ctrl.agentTaskRepo.RecordHeartbeat(ctx, idTask); err != nil {
		if errors.Is(err, repository.ErrTaskStatusMismatch) {
			_ = c.Error(errs.ErrTaskNotActive)
			c.Status(http.StatusConflict)
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
	prId, prUrl, err := openOrReusePr(ctx, host, body.BranchName, title, body.PrBody)
	if err != nil {
		return nil, err
	}
	return &prComputed{prId: prId, prUrl: prUrl, hostType: integration.HostType, idGitIntegration: integration.IdGitIntegration}, nil
}

// openOrReusePr reuses an open PR/MR for branch if one exists, otherwise opens a
// new one against the repo's default branch.
func openOrReusePr(ctx context.Context, host githost.GitHost, branch, title, body string) (string, string, error) {
	prId, prUrl, found, err := host.FindOpenPullRequest(ctx, branch)
	if err != nil {
		return "", "", fmt.Errorf("looking up existing PR: %w", err)
	}
	if found {
		return prId, prUrl, nil
	}
	base, err := host.DefaultBranch(ctx)
	if err != nil {
		return "", "", fmt.Errorf("resolving base branch: %w", err)
	}
	prId, prUrl, err = host.CreatePullRequest(ctx, branch, base, title, body)
	if err != nil {
		return "", "", fmt.Errorf("opening PR: %w", err)
	}
	return prId, prUrl, nil
}

// resolveIntegrationId returns the git_integration to use for PR creation: the
// one stamped on the run by ReportRunRepo, or, as a fallback for single-repo
// projects, the project's only integration.
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
	agent.BroadcastRunUpdate(context.Background(), ctrl.notifier, ctrl.projectRepo, ctrl.agentRunRepo, ctrl.agentTaskRepo, run)
}

// respondRunSnapshot returns the same snapshot shape as the GET endpoints and
// the websocket broadcast. Mutating endpoints must use this — the bare run
// drops `stages`, blanking the client's timeline until the next notice.
func (ctrl *AgentRunController) respondRunSnapshot(c *gin.Context, ctx context.Context, run *model.AgentRun) {
	events, err := ctrl.agentRunRepo.LoadEvents(ctx, run.IdRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	tasks, err := ctrl.agentTaskRepo.LoadByRun(ctx, run.IdRun)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, agent.BuildRunSnapshot(run, events, tasks))
}
