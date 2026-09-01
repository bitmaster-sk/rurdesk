package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/bitmaster-sk/rurdesk/api/internal/urlutil"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type IssueController struct {
	issueRepo           *repository.IssueRepository
	projectRepo         *repository.ProjectRepository
	userRepo            *repository.UserRepository
	stateRepo           *repository.StateRepository
	severityRepo        *repository.SeverityRepository
	issueTypeRepo       *repository.IssueTypeRepository
	gitIntRepo          *repository.GitIntegrationRepository
	agentRunRepo        *repository.AgentRunRepository
	agentTaskRepo       *repository.AgentTaskRepository
	botGwRepo           *repository.BotGatewayRepository
	projectSkillService *service.ProjectSkillService
	participantRepo     *repository.IssueParticipantRepository
	dispatcher          *agent.Dispatcher
	notifier            *notify.Notifier
	acl                 *service.AclService
	stagePlan           *service.StagePlanService
	notifSvc            *service.NotificationService
	pool                *pgxpool.Pool
}

func NewIssueController(
	ir *repository.IssueRepository,
	pr *repository.ProjectRepository,
	ur *repository.UserRepository,
	stateRepo *repository.StateRepository,
	severityRepo *repository.SeverityRepository,
	issueTypeRepo *repository.IssueTypeRepository,
	participantRepo *repository.IssueParticipantRepository,
	acl *service.AclService,
	notifSvc *service.NotificationService,
	pool *pgxpool.Pool,
) *IssueController {
	return &IssueController{
		issueRepo:       ir,
		projectRepo:     pr,
		userRepo:        ur,
		stateRepo:       stateRepo,
		severityRepo:    severityRepo,
		issueTypeRepo:   issueTypeRepo,
		participantRepo: participantRepo,
		acl:             acl,
		notifSvc:        notifSvc,
		pool:            pool,
	}
}

func (ic *IssueController) WithGitIntRepo(repo *repository.GitIntegrationRepository) *IssueController {
	ic.gitIntRepo = repo
	return ic
}

func (ic *IssueController) WithAgentRun(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	botGwRepo *repository.BotGatewayRepository,
	projectSkillService *service.ProjectSkillService,
	stagePlan *service.StagePlanService,
	dispatcher *agent.Dispatcher,
	notifier *notify.Notifier,
) *IssueController {
	ic.agentRunRepo = agentRunRepo
	ic.agentTaskRepo = agentTaskRepo
	ic.botGwRepo = botGwRepo
	ic.projectSkillService = projectSkillService
	ic.stagePlan = stagePlan
	ic.dispatcher = dispatcher
	ic.notifier = notifier
	return ic
}

func (ic *IssueController) GetIssues(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ic.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	filter, err := buildIssueFilter(c, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	if groupBy := c.Query("groupBy"); groupBy != "" {
		ic.getIssuesGrouped(c, filter, groupBy)
		return
	}

	// No limit param → unlimited (back-compat).
	if filter.Limit == nil {
		issues, err := ic.issueRepo.LoadIssues(ctx, filter)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, model.IssuesPageRes{Items: issues, NextCursor: nil, Total: len(issues)})
		return
	}

	limit := int(*filter.Limit)
	items, next, err := ic.issueRepo.LoadIssuesPage(ctx, filter, limit)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	total, err := ic.issueRepo.CountIssues(ctx, filter)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, model.IssuesPageRes{Items: items, NextCursor: next, Total: total})
}

func (ic *IssueController) getIssuesGrouped(c *gin.Context, filter *model.LoadIssuesFilter, groupBy string) {
	keys := strings.Split(groupBy, ",")
	perGroup := 20
	if filter.Limit != nil {
		perGroup = int(*filter.Limit)
	}
	groups, err := ic.issueRepo.LoadIssuesGrouped(c.Request.Context(), filter, keys, perGroup)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

func (ic *IssueController) GetIssue(c *gin.Context) {
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

	if !ic.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	issue, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: &idIssuePublic})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, issue)
}

func (ic *IssueController) validateIssueRefsInProject(ctx context.Context, idProject int64, idState, idSeverity, idIssueType *int64) error {
	if idState != nil {
		if _, err := ic.stateRepo.LoadState(ctx, idProject, *idState); err != nil {
			return errStateNotInProject
		}
	}
	if idSeverity != nil {
		if _, err := ic.severityRepo.LoadSeverity(ctx, idProject, *idSeverity); err != nil {
			return errSeverityNotInProject
		}
	}
	if idIssueType != nil {
		if _, err := ic.issueTypeRepo.LoadIssueType(ctx, idProject, *idIssueType); err != nil {
			return errIssueTypeNotInProject
		}
	}
	return nil
}

func (ic *IssueController) CreateIssue(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.CreateIssueReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	dto.IdProject = idProject

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey != "" {
		if len(idempotencyKey) > 80 {
			_ = c.Error(fmt.Errorf("idempotency key too long"))
			c.Status(http.StatusBadRequest)
			return
		}
		existing, lookupErr := ic.issueRepo.FindByIdempotencyKey(ctx, user.IdUser, idempotencyKey)
		if lookupErr != nil {
			_ = c.Error(lookupErr)
			c.Status(http.StatusInternalServerError)
			return
		}
		if existing != nil {
			c.JSON(http.StatusOK, existing)
			return
		}
		dto.IdempotencyKey = &idempotencyKey
	}

	var result *model.Issue
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		if !ic.acl.CanCreateIssue(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		if dto.AssignedTo != nil {
			if !ic.acl.CanReadProject(ctx, *dto.AssignedTo, dto.IdProject) {
				return errForbidden
			}
		}
		if err := ic.validateIssueRefsInProject(ctx, dto.IdProject, dto.IdState, dto.IdSeverity, dto.IdIssueType); err != nil {
			return err
		}
		var insertErr error
		result, insertErr = ic.issueRepo.InsertIssue(ctx, &model.Issue{
			IdProject:      dto.IdProject,
			IdState:        dto.IdState,
			IdSeverity:     dto.IdSeverity,
			IdIssueType:    dto.IdIssueType,
			Title:          dto.Title,
			Description:    dto.Description,
			CreateBy:       user.IdUser,
			UpdateBy:       user.IdUser,
			AssignedTo:     dto.AssignedTo,
			Estimated:      dto.Estimated,
			Points:         dto.Points,
			ScheduledAt:    dto.ScheduledAt,
			IdempotencyKey: dto.IdempotencyKey,
		})
		if insertErr != nil {
			return insertErr
		}
		idCreator := user.IdUser
		if err := ic.participantRepo.Add(ctx, result.IdIssue, idCreator, "creator", &idCreator); err != nil {
			return fmt.Errorf("adding creator participant: %w", err)
		}
		if dto.AssignedTo != nil && *dto.AssignedTo != idCreator {
			if err := ic.participantRepo.Add(ctx, result.IdIssue, *dto.AssignedTo, "assignee", &idCreator); err != nil {
				return fmt.Errorf("adding assignee participant: %w", err)
			}
		}
		return nil
	})
	if err == errForbidden {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err == errStateNotInProject || err == errSeverityNotInProject || err == errIssueTypeNotInProject {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		// Race: a concurrent request with the same idempotency key won —
		// Postgres unique_violation (23505); re-select and return the winner.
		var pgErr *pgconn.PgError
		if idempotencyKey != "" && errors.As(err, &pgErr) && pgErr.Code == "23505" {
			existing, lookupErr := ic.issueRepo.FindByIdempotencyKey(ctx, user.IdUser, idempotencyKey)
			if lookupErr == nil && existing != nil {
				c.JSON(http.StatusOK, existing)
				return
			}
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// No prior assignee, so handleBotAssignment takes its first-assignment path
	// when the new issue is bot-assigned.
	if ic.dispatcher != nil && result != nil {
		ic.handleBotAssignment(ctx, &model.Issue{}, result)
	}

	// Broadcast so other project members' open views pick it up live.
	if result != nil {
		agent.BroadcastIssueNotice(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, result.IdIssue, notify.ActionCreate)
	}

	c.JSON(http.StatusOK, result)
}

func (ic *IssueController) EditIssue(c *gin.Context) {
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

	var dto model.EditIssueReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err := dto.Validate(); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	dto.IdProject = idProject
	dto.IdIssuePublic = idIssuePublic

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var result *model.Issue
	var oldIssue *model.Issue
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		if dto.AssignedTo.Value != nil {
			if !ic.acl.CanReadProject(ctx, *dto.AssignedTo.Value, dto.IdProject) {
				return errForbidden
			}
		}
		if err := ic.validateIssueRefsInProject(ctx, dto.IdProject, dto.IdState.Value, dto.IdSeverity.Value, dto.IdIssueType.Value); err != nil {
			return err
		}
		issue, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &dto.IdProject, IdIssuePublic: &dto.IdIssuePublic})
		if err != nil {
			return err
		}
		snapshot := *issue
		oldIssue = &snapshot

		idGitIntegration := dto.IdGitIntegration.PtrOrElse(issue.IdGitIntegration)
		mrId := dto.MrId.PtrOrElse(issue.MrId)

		// Both-or-neither: MR link fields must move together.
		if (idGitIntegration == nil) != (mrId == nil) {
			return errInvalidMrLink
		}
		if dto.IdGitIntegration.IsDefined && dto.IdGitIntegration.Value != nil && ic.gitIntRepo != nil {
			gi, loadErr := ic.gitIntRepo.LoadByID(ctx, *dto.IdGitIntegration.Value, dto.IdProject)
			if loadErr != nil {
				return loadErr
			}
			if gi == nil {
				return errGitIntegrationNotFound
			}
		}

		// A bot with no configured gateway can't be assigned.
		if ic.botGwRepo != nil && dto.AssignedTo.Value != nil && !int64PtrEq(issue.AssignedTo, dto.AssignedTo.Value) {
			assignee, loadErr := ic.userRepo.LoadUser(ctx, *dto.AssignedTo.Value)
			if loadErr == nil && assignee.IsBot {
				gw, gwErr := ic.botGwRepo.LoadByBotUser(ctx, assignee.IdUser)
				if gwErr != nil || gw == nil {
					return errBotNoGateway
				}
			}
		}

		issue.UpdateBy = user.IdUser
		issue.IdState = dto.IdState.PtrOrElse(issue.IdState)
		issue.IdSeverity = dto.IdSeverity.PtrOrElse(issue.IdSeverity)
		issue.IdIssueType = dto.IdIssueType.PtrOrElse(issue.IdIssueType)
		issue.Title = dto.Title.OrElse(issue.Title)
		issue.Description = dto.Description.OrElse(issue.Description)
		issue.AssignedTo = dto.AssignedTo.PtrOrElse(issue.AssignedTo)
		issue.Estimated = dto.Estimated.OrElse(issue.Estimated)
		issue.Points = dto.Points.PtrOrElse(issue.Points)
		issue.ScheduledAt = dto.ScheduledAt.PtrOrElse(issue.ScheduledAt)
		issue.IdGitIntegration = idGitIntegration
		issue.MrId = mrId

		result, err = ic.issueRepo.UpdateIssue(ctx, issue)
		if err != nil {
			return err
		}
		if dto.AssignedTo.Value != nil {
			if addErr := ic.participantRepo.Add(ctx, issue.IdIssue, *dto.AssignedTo.Value, "assignee", &user.IdUser); addErr != nil {
				return fmt.Errorf("auto-adding assignee as participant (edit): %w", addErr)
			}
		}
		return nil
	})
	if err == errForbidden {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err == errInvalidMrLink {
		_ = c.Error(errInvalidMrLink)
		c.Status(http.StatusBadRequest)
		return
	}
	if err == errStateNotInProject || err == errSeverityNotInProject || err == errIssueTypeNotInProject {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err == errGitIntegrationNotFound {
		_ = c.Error(errGitIntegrationNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err == errBotNoGateway {
		_ = c.Error(errBotNoGateway)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Notifications, agent dispatch, and broadcasts run outside the transaction.
	if oldIssue != nil {
		project, projErr := ic.projectRepo.LoadProject(ctx, dto.IdProject)
		if projErr == nil {
			ic.sendIssueNotifications(ctx, user, oldIssue, result, project)
		}
	}

	if ic.dispatcher != nil && oldIssue != nil {
		ic.handleBotAssignment(ctx, oldIssue, result)
	}

	// An assignee change may have added a participant inside the tx above;
	// broadcast the updated list so open clients refresh without a reload.
	if dto.AssignedTo.Value != nil {
		broadcastParticipants(ctx, ic.notifier, ic.projectRepo, ic.participantRepo, dto.IdProject, result.IdIssue)
	}

	agent.BroadcastIssueUpdate(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, result.IdIssue)

	c.JSON(http.StatusOK, result)
}

// Creates the run directly rather than through EditIssue, whose assignee hook
// would create a second run carrying the project defaults.
func (ic *IssueController) AssignAgent(c *gin.Context) {
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

	var dto model.CreateAgentRunReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(errs.ErrValidation.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}
	for stage := range dto.IdsSkillByStage {
		if !constants.IsSkillStage(stage) {
			_ = c.Error(errs.ErrBadRequest.WithMessage("unknown stage: " + stage))
			c.Status(http.StatusBadRequest)
			return
		}
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	issue, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: &idIssuePublic})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if issue == nil {
		_ = c.Error(errNotFound)
		c.Status(http.StatusNotFound)
		return
	}

	bot, err := ic.userRepo.LoadUser(ctx, dto.IdUserBot)
	if err != nil || bot == nil {
		_ = c.Error(errNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if !bot.IsBot {
		_ = c.Error(errNotABot)
		c.Status(http.StatusBadRequest)
		return
	}
	// Same gate as the EditIssue assignee path: without it this endpoint could
	// hand a project's issue to an agent that cannot even read the project.
	if !ic.acl.CanReadProject(ctx, bot.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	gateway, gwErr := ic.botGwRepo.LoadByBotUser(ctx, bot.IdUser)
	if gwErr != nil || gateway == nil {
		_ = c.Error(errBotNoGateway)
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	existing, err := ic.agentRunRepo.LoadActiveByIssue(ctx, issue.IdIssue)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if existing != nil {
		_ = c.Error(errIssueHasActiveRun)
		c.Status(http.StatusConflict)
		return
	}

	oldIssue := *issue

	// The assignee write and the run insert must commit together: on the 23505 race
	// the endpoint answers 409 and the issue must not stay re-pointed.
	var updated *model.Issue
	var run *model.AgentRun
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		issue.AssignedTo = &bot.IdUser
		issue.UpdateBy = user.IdUser

		var txErr error
		updated, txErr = ic.issueRepo.UpdateIssue(ctx, issue)
		if txErr != nil {
			return txErr
		}
		if txErr = ic.participantRepo.Add(ctx, issue.IdIssue, bot.IdUser, "assignee", &user.IdUser); txErr != nil {
			return fmt.Errorf("auto-adding assignee as participant (assign-agent): %w", txErr)
		}
		stagePlan, txErr := ic.stagePlan.Build(dto.IdsSkillByStage)
		if txErr != nil {
			return txErr
		}
		run, txErr = ic.agentRunRepo.Insert(ctx, issue.IdIssue, bot.IdUser, idProject, stagePlan)
		return txErr
	})
	if err != nil {
		// The partial unique index backstops a race with the EditIssue path.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = c.Error(errIssueHasActiveRun.WithMessage("issue already has an active run"))
			c.Status(http.StatusConflict)
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if project, projErr := ic.projectRepo.LoadProject(ctx, idProject); projErr == nil {
		ic.sendIssueNotifications(ctx, user, &oldIssue, updated, project)
	}

	agent.BroadcastRunUpdate(ctx, ic.notifier, ic.projectRepo, ic.agentRunRepo, ic.agentTaskRepo, run)
	broadcastParticipants(ctx, ic.notifier, ic.projectRepo, ic.participantRepo, idProject, updated.IdIssue)
	agent.BroadcastIssueUpdate(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, updated.IdIssue)

	c.JSON(http.StatusOK, run)
}

func (ic *IssueController) handleBotAssignment(ctx context.Context, oldIssue, newIssue *model.Issue) {
	if int64PtrEq(oldIssue.AssignedTo, newIssue.AssignedTo) {
		return
	}

	// An assignee change never cancels the run — completed stages are kept for
	// resume/hand-off. The scheduler gates on issue.assigned_to ==
	// run.id_user_bot, so a run parks when the issue leaves its bot and resumes
	// when it returns (or is re-pointed below). Only Cancel truly cancels a run.
	existing, _ := ic.agentRunRepo.LoadActiveByIssue(ctx, newIssue.IdIssue)

	// Moving away from a bot aborts its in-flight stage: the gateway subprocess
	// stops and the half-finished stage is discarded (redone by whoever picks
	// the run up next). Completed stages and the run phase are untouched.
	if oldIssue.AssignedTo != nil && existing != nil {
		oldUser, err := ic.userRepo.LoadUser(ctx, *oldIssue.AssignedTo)
		if err == nil && oldUser.IsBot {
			_ = ic.agentTaskRepo.CancelNonTerminalForRun(ctx, existing.IdRun)
			runForAbort := existing // still points at the old bot
			go func() {
				_ = ic.dispatcher.DispatchCancelled(context.Background(), runForAbort)
			}()
		}
	}

	// New assignee isn't a bot — nothing to route; the gate parks the run.
	if newIssue.AssignedTo == nil {
		if existing != nil {
			agent.BroadcastRunUpdate(ctx, ic.notifier, ic.projectRepo, ic.agentRunRepo, ic.agentTaskRepo, existing)
		}
		return
	}
	newUser, err := ic.userRepo.LoadUser(ctx, *newIssue.AssignedTo)
	if err != nil || !newUser.IsBot {
		if existing != nil {
			agent.BroadcastRunUpdate(ctx, ic.notifier, ic.projectRepo, ic.agentRunRepo, ic.agentTaskRepo, existing)
		}
		return
	}
	gateway, err := ic.botGwRepo.LoadByBotUser(ctx, newUser.IdUser)
	if err != nil || gateway == nil {
		// Bot has no gateway — can't route; broadcast so the client sees the
		// run parked instead of silently dropping the update.
		if existing != nil {
			agent.BroadcastRunUpdate(ctx, ic.notifier, ic.projectRepo, ic.agentRunRepo, ic.agentTaskRepo, existing)
		}
		return
	}

	// Resume/hand-off: re-point the existing non-terminal run to the new bot,
	// keeping completed stages; the scheduler dispatches the next stage.
	if existing != nil {
		updated := existing
		if existing.IdUserBot != newUser.IdUser {
			if reassigned, rerr := ic.agentRunRepo.ReassignBot(ctx, existing.IdRun, newUser.IdUser); rerr == nil && reassigned != nil {
				updated = reassigned
			}
		}
		agent.BroadcastRunUpdate(ctx, ic.notifier, ic.projectRepo, ic.agentRunRepo, ic.agentTaskRepo, updated)
		return
	}

	// A failing matrix read must not block the run: skills are an enhancement.
	idsSkillByStage, skillErr := ic.projectSkillService.LoadDefaultIdsSkillByStage(ctx, newIssue.IdProject)
	if skillErr != nil {
		log.Warn().Err(skillErr).Int64("idProject", newIssue.IdProject).
			Msg("loading project skill defaults — creating run without skills")
		idsSkillByStage = nil
	}
	stagePlan, err := ic.stagePlan.Build(idsSkillByStage)
	if err != nil {
		log.Error().Err(err).Int64("idIssue", newIssue.IdIssue).Msg("building stage plan — run not created")
		return
	}
	run, err := ic.agentRunRepo.Insert(ctx, newIssue.IdIssue, newUser.IdUser, newIssue.IdProject, stagePlan)
	if err != nil {
		return
	}
	agent.BroadcastRunUpdate(ctx, ic.notifier, ic.projectRepo, ic.agentRunRepo, ic.agentTaskRepo, run)
}

func (ic *IssueController) DeleteIssue(c *gin.Context) {
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

	// Snapshot before it disappears — the delete notice carries it.
	var deletedIssue *model.Issue
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		if !ic.acl.CanDeleteIssue(ctx, user.IdUser, idProject) {
			return errForbidden
		}
		issue, loadErr := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: &idIssuePublic})
		if loadErr == nil {
			deletedIssue = issue
		}
		return ic.issueRepo.DeleteIssue(ctx, idProject, idIssuePublic)
	})
	if err == errForbidden {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("delete issue: error")
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	agent.BroadcastIssueSnapshot(ctx, ic.notifier, ic.projectRepo, deletedIssue, notify.ActionDelete)

	c.Status(http.StatusOK)
}

func (ic *IssueController) BulkEditIssues(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.BulkEditIssuesReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var results []*model.Issue
	// Issues whose participant list changed (assignee added) — broadcast after commit.
	var participantChangedIssueIds []int64
	// Old assignee per issue, captured before the update so post-commit bot
	// routing sees a truthful "old" side (re-specifying the same bot must not
	// spawn a new run).
	oldAssignedByPublic := make(map[int64]*int64)
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		participantChangedIssueIds = nil // reset if the tx body re-runs
		for k := range oldAssignedByPublic {
			delete(oldAssignedByPublic, k)
		}
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errForbidden
		}

		for _, entry := range dto.Issues {
			if err := ic.validateIssueRefsInProject(ctx, idProject, entry.IdState, entry.IdSeverity, entry.IdIssueType); err != nil {
				return err
			}
			if entry.IdUserAssigned != nil {
				if !ic.acl.CanReadProject(ctx, *entry.IdUserAssigned, idProject) {
					return fmt.Errorf("user %d is not a member of project %d", *entry.IdUserAssigned, idProject)
				}
			}
		}

		// Capture current assignees for issues about to be reassigned.
		assignPublicIds := make([]int64, 0, len(dto.Issues))
		for _, entry := range dto.Issues {
			if entry.IdUserAssigned != nil {
				assignPublicIds = append(assignPublicIds, entry.IdIssuePublic)
			}
		}
		if len(assignPublicIds) > 0 {
			current, loadErr := ic.issueRepo.LoadIssues(ctx, &model.LoadIssuesFilter{
				IdProject:      idProject,
				IdsIssuePublic: assignPublicIds,
			})
			if loadErr != nil {
				return fmt.Errorf("loading issues for bot-assignment routing: %w", loadErr)
			}
			for _, issue := range current {
				oldAssignedByPublic[issue.IdIssuePublic] = issue.AssignedTo
			}
		}

		var txErr error
		results, txErr = ic.issueRepo.BulkUpdateIssues(ctx, idProject, dto.Issues, user.IdUser)
		if txErr != nil {
			return txErr
		}
		// Auto-add each newly-assigned user as a participant.
		issueIdByPublic := make(map[int64]int64, len(results))
		for _, updatedIssue := range results {
			issueIdByPublic[updatedIssue.IdIssuePublic] = updatedIssue.IdIssue
		}
		for _, entry := range dto.Issues {
			if entry.IdUserAssigned == nil {
				continue
			}
			idIssue, ok := issueIdByPublic[entry.IdIssuePublic]
			if !ok {
				continue
			}
			if addErr := ic.participantRepo.Add(ctx, idIssue, *entry.IdUserAssigned, "assignee", &user.IdUser); addErr != nil {
				return fmt.Errorf("auto-adding assignee as participant (bulk edit issue=%d): %w", idIssue, addErr)
			}
			participantChangedIssueIds = append(participantChangedIssueIds, idIssue)
		}
		return nil
	})

	if err == errForbidden {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err == errStateNotInProject || err == errSeverityNotInProject || err == errIssueTypeNotInProject {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	for _, idIssue := range participantChangedIssueIds {
		broadcastParticipants(ctx, ic.notifier, ic.projectRepo, ic.participantRepo, idProject, idIssue)
	}

	// Broadcast every updated issue — the gantt drag/resize cascade depends on it.
	for _, updatedIssue := range results {
		agent.BroadcastIssueSnapshot(ctx, ic.notifier, ic.projectRepo, updatedIssue, notify.ActionUpdate)
	}

	// Reuses single-issue routing with the captured old assignee so an
	// unchanged bot assignment is a no-op rather than a duplicate run.
	if ic.dispatcher != nil {
		for _, newIssue := range results {
			oldAssigned, changed := oldAssignedByPublic[newIssue.IdIssuePublic]
			if !changed {
				continue
			}
			oldIssue := &model.Issue{
				IdIssue:    newIssue.IdIssue,
				IdProject:  newIssue.IdProject,
				AssignedTo: oldAssigned,
			}
			ic.handleBotAssignment(ctx, oldIssue, newIssue)
		}
	}

	c.JSON(http.StatusOK, results)
}

// buildIssueFilter constructs a LoadIssuesFilter from query params. Errors only on
// a malformed *Within — a silently dropped window would widen the result set.
func buildIssueFilter(c *gin.Context, idProject int64) (*model.LoadIssuesFilter, error) {
	f := &model.LoadIssuesFilter{IdProject: idProject}
	f.Title = c.Query("title")
	f.IdsSeverity = urlutil.ParseInt64Array(c, "idsSeverity")
	f.SeverityUnset = c.Query("severityUnset") == "true"
	f.IdsIssueType = urlutil.ParseInt64Array(c, "idsIssueType")
	f.IssueTypeUnset = c.Query("issueTypeUnset") == "true"
	f.IdsState = urlutil.ParseInt64Array(c, "idsState")
	f.StateUnset = c.Query("stateUnset") == "true"
	f.IdsAssignedTo = urlutil.ParseInt64Array(c, "idsAssignedTo")
	f.AssignedToUnset = c.Query("assignedToUnset") == "true"
	f.ScheduledAtUnset = c.Query("scheduledAtUnset") == "true"
	f.AssignedToNull = c.Query("assignedToNull") == "true"
	f.IdsIssuePublic = urlutil.ParseInt64Array(c, "idsIssuePublic")
	if v := c.Query("idSprint"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.IdSprint = &id
		}
	}
	f.SprintUnset = c.Query("sprintUnset") == "true"
	orderCol := c.Query("orderColumn")
	orderDir := c.Query("orderDirection")
	if orderCol != "" {
		f.Order = &model.Order{Column: orderCol, Direction: orderDir}
	}
	if search := c.Query("search"); search != "" {
		f.Search = &search
	}
	if c.Query("excludeFinalStates") == "true" {
		f.ExcludeFinalStates = true
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && limit > 0 {
			if limit > 200 {
				limit = 200
			}
			f.Limit = &limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil && offset >= 0 {
			f.Offset = &offset
		}
	}
	if cursor := c.Query("cursor"); cursor != "" {
		f.Cursor = &cursor
	}
	// Date-range filters (calendar/gantt windowing).
	parseTime := func(q string) time.Time {
		if v := c.Query(q); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
		return time.Time{}
	}
	f.ScheduledAtFrom = parseTime("scheduledAtFrom")
	f.ScheduledAtTo = parseTime("scheduledAtTo")
	f.CreateAtFrom = parseTime("createAtFrom")
	f.CreateAtTo = parseTime("createAtTo")
	f.UpdateAtFrom = parseTime("updateAtFrom")
	f.UpdateAtTo = parseTime("updateAtTo")

	var err error
	if f.CreateAtWithin, err = parseWithinParam(c, "createAtWithin"); err != nil {
		return nil, err
	}
	if f.UpdateAtWithin, err = parseWithinParam(c, "updateAtWithin"); err != nil {
		return nil, err
	}
	return f, nil
}

func parseWithinParam(c *gin.Context, key string) (time.Duration, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	within, err := urlutil.ParsePositiveDuration(raw)
	if err != nil {
		return 0, errInvalidWithin
	}
	return within, nil
}

func (ic *IssueController) sendIssueNotifications(
	ctx context.Context,
	actor model.User,
	oldIssue *model.Issue,
	newIssue *model.Issue,
	project *model.Project,
) {
	idProject := project.IdProject
	idIssuePublic := newIssue.IdIssuePublic

	source := ""
	if isBot, err := ic.userRepo.IsBotUser(ctx, actor.IdUser); err == nil && isBot {
		source = "bot"
	}

	base := &model.CreateNotificationReq{
		IdProject:     &idProject,
		ProjectName:   project.Name,
		ProjectColor:  project.Color,
		ActorName:     actor.Name,
		ActorAvatarBg: actor.ColorAvatarBg,
		RefType:       constants.NotificationRefTypeIssue,
		RefId:         strconv.FormatInt(newIssue.IdIssue, 10),
		RefTitle:      newIssue.Title,
		RefPublicId:   &idIssuePublic,
		Source:        source,
	}

	// Assignee changed
	if !int64PtrEq(oldIssue.AssignedTo, newIssue.AssignedTo) && newIssue.AssignedTo != nil {
		if *newIssue.AssignedTo != actor.IdUser {
			dto := *base
			dto.IdUser = *newIssue.AssignedTo
			dto.Type = constants.NotificationTypeAssigned
			if notifErr := ic.notifSvc.Notify(ctx, &dto); notifErr != nil {
				log.Warn().Err(notifErr).
					Int64("idUser", dto.IdUser).
					Int64("idIssue", newIssue.IdIssue).
					Msg("sendIssueNotifications: failed to create notification")
			}
		}
	}

	// State changed — notify creator and assignee (except actor)
	if !int64PtrEq(oldIssue.IdState, newIssue.IdState) {
		var stateBody *model.NotificationBodyState
		if newIssue.IdState != nil {
			if state, err := ic.stateRepo.LoadState(ctx, newIssue.IdProject, *newIssue.IdState); err == nil {
				stateBody = &model.NotificationBodyState{StateName: state.Name}
			}
		}
		for _, recipientId := range uniqueRecipients(newIssue.CreateBy, newIssue.AssignedTo, actor.IdUser) {
			dto := *base
			dto.IdUser = recipientId
			dto.Type = constants.NotificationTypeStateChanged
			dto.Body = stateBody
			if notifErr := ic.notifSvc.Notify(ctx, &dto); notifErr != nil {
				log.Warn().Err(notifErr).
					Int64("idUser", recipientId).
					Int64("idIssue", newIssue.IdIssue).
					Msg("sendIssueNotifications: failed to create notification")
			}
		}
	}

	// Severity changed — notify creator and assignee (except actor)
	if !int64PtrEq(oldIssue.IdSeverity, newIssue.IdSeverity) {
		var severityBody *model.NotificationBodySeverity
		notificationType := constants.NotificationTypeSeverityEscalated
		if newIssue.IdSeverity != nil {
			if sev, err := ic.severityRepo.LoadSeverity(ctx, newIssue.IdProject, *newIssue.IdSeverity); err == nil {
				severityBody = &model.NotificationBodySeverity{SeverityName: sev.Title, SeverityColor: sev.Color}
				if oldIssue.IdSeverity != nil {
					if oldSev, err := ic.severityRepo.LoadSeverity(ctx, oldIssue.IdProject, *oldIssue.IdSeverity); err == nil {
						if sev.OrderRank < oldSev.OrderRank {
							notificationType = constants.NotificationTypeSeverityDeescalated
						}
					}
				}
			}
		} else {
			notificationType = constants.NotificationTypeSeverityDeescalated
		}

		for _, recipientId := range uniqueRecipients(newIssue.CreateBy, newIssue.AssignedTo, actor.IdUser) {
			dto := *base
			dto.IdUser = recipientId
			dto.Type = notificationType
			dto.Body = severityBody
			if notifErr := ic.notifSvc.Notify(ctx, &dto); notifErr != nil {
				log.Warn().Err(notifErr).
					Int64("idUser", recipientId).
					Int64("idIssue", newIssue.IdIssue).
					Msg("sendIssueNotifications: failed to create notification")
			}
		}
	}
}

func int64PtrEq(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func uniqueRecipients(creatorId int64, assignedTo *int64, excludeId int64) []int64 {
	seen := map[int64]bool{excludeId: true}
	var result []int64
	if !seen[creatorId] {
		seen[creatorId] = true
		result = append(result, creatorId)
	}
	if assignedTo != nil && !seen[*assignedTo] {
		seen[*assignedTo] = true
		result = append(result, *assignedTo)
	}
	return result
}
