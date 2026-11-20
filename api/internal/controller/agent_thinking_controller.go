package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type AgentThinkingController struct {
	thinking      *agent.ThinkingService
	tasks         *agent.TaskService
	agentRunRepo  *repository.AgentRunRepository
	agentTaskRepo *repository.AgentTaskRepository
	conversations *service.AgentConversationService
	acl           *service.AclService
}

func NewAgentThinkingController(
	thinking *agent.ThinkingService,
	tasks *agent.TaskService,
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	conversations *service.AgentConversationService,
	acl *service.AclService,
) *AgentThinkingController {
	return &AgentThinkingController{
		thinking:      thinking,
		tasks:         tasks,
		agentRunRepo:  agentRunRepo,
		agentTaskRepo: agentTaskRepo,
		conversations: conversations,
		acl:           acl,
	}
}

func (ctrl *AgentThinkingController) Create(c *gin.Context) {
	ctx := c.Request.Context()
	idTask, err := strconv.ParseInt(c.Param("idTask"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	var body model.AgentThinkingReq
	if err := c.ShouldBindJSON(&body); err != nil {
		_ = c.Error(errs.ErrValidation)
		c.Status(http.StatusBadRequest)
		return
	}
	user, _ := extctx.GetUser(ctx)
	if !ctrl.requireAssignedAgent(c, idTask, user.IdUser) {
		return
	}

	if err := ctrl.thinking.Create(ctx, idTask, body.Seq, body.Events); err != nil {
		if errors.Is(err, repository.ErrTaskStatusMismatch) {
			_ = c.Error(errs.ErrTaskNotRunningByAgent)
			c.Status(errs.ErrTaskNotRunningByAgent.HttpStatus())
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (ctrl *AgentThinkingController) Get(c *gin.Context) {
	ctx := c.Request.Context()
	idRun, err := strconv.ParseInt(c.Param("idRun"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	stage := c.Query("stage")
	if stage == "" {
		_ = c.Error(errs.ErrValidation)
		c.Status(http.StatusBadRequest)
		return
	}

	run, err := ctrl.agentRunRepo.LoadById(ctx, idRun)
	// A private run is read per turn by its owner (GetChatTurn) and does not exist here.
	if err != nil || run.IsPrivate() {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	user, _ := extctx.GetUser(ctx)
	if !ctrl.acl.CanReadProject(ctx, user.IdUser, run.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	res, err := ctrl.thinking.LoadForStage(ctx, idRun, stage)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetChatTurn returns the thinking of one turn of the caller's conversation; another user's
// conversation or a task outside its run is not found.
func (ctrl *AgentThinkingController) GetChatTurn(c *gin.Context) {
	ctx := c.Request.Context()
	idConversation, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	idTask, err := strconv.ParseInt(c.Query("idTask"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrValidation)
		return
	}

	user, _ := extctx.GetUser(ctx)
	conversation, err := ctrl.conversations.LoadConversation(ctx, &user, idConversation)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	task, err := ctrl.agentTaskRepo.LoadById(ctx, idTask)
	if errors.Is(err, repository.ErrTaskNotFound) {
		ResponseErr(c, errs.ErrNotFound)
		return
	}
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if conversation.IdRun == nil || task.IdRun != *conversation.IdRun {
		ResponseErr(c, errs.ErrNotFound)
		return
	}

	res, err := ctrl.thinking.LoadForTask(ctx, idTask)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// requireAssignedAgent reports whether the caller is the agent executing the task,
// and writes the refusal itself when it is not: 403 for another user, 404 for a
// task that does not exist, so a typo in a task id does not read as revoked
// access.
func (ctrl *AgentThinkingController) requireAssignedAgent(c *gin.Context, idTask, idUser int64) bool {
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
