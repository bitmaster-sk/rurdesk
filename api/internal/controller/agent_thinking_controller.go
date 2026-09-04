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
	thinking     *agent.ThinkingService
	tasks        *agent.TaskService
	agentRunRepo *repository.AgentRunRepository
	acl          *service.AclService
}

func NewAgentThinkingController(
	thinking *agent.ThinkingService,
	tasks *agent.TaskService,
	agentRunRepo *repository.AgentRunRepository,
	acl *service.AclService,
) *AgentThinkingController {
	return &AgentThinkingController{
		thinking:     thinking,
		tasks:        tasks,
		agentRunRepo: agentRunRepo,
		acl:          acl,
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
	if err != nil {
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
