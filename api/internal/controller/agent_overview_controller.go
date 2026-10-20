package controller

import (
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type AgentOverviewController struct {
	overviewRepo    *repository.AgentOverviewRepository
	agentTaskConfig *service.AgentTaskConfigService
	acl             *service.AclService
}

func NewAgentOverviewController(
	overviewRepo *repository.AgentOverviewRepository,
	agentTaskConfig *service.AgentTaskConfigService,
	acl *service.AclService,
) *AgentOverviewController {
	return &AgentOverviewController{overviewRepo: overviewRepo, agentTaskConfig: agentTaskConfig, acl: acl}
}

func (ctrl *AgentOverviewController) Get(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !ctrl.acl.CanReadProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	overview, err := ctrl.overviewRepo.Load(ctx, idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if overview == nil {
		overview = []*model.AgentOverview{}
	}
	if err := ctrl.agentTaskConfig.ApplyToOverviews(ctx, overview); err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, overview)
}
