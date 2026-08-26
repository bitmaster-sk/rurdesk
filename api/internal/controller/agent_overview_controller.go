package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type AgentOverviewController struct {
	overviewRepo *repository.AgentOverviewRepository
	acl          *service.AclService
}

func NewAgentOverviewController(
	overviewRepo *repository.AgentOverviewRepository,
	acl *service.AclService,
) *AgentOverviewController {
	return &AgentOverviewController{overviewRepo: overviewRepo, acl: acl}
}

func (ctrl *AgentOverviewController) Get(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !ctrl.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	overview, err := ctrl.overviewRepo.Load(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if overview == nil {
		overview = []*model.AgentOverview{}
	}
	c.JSON(http.StatusOK, overview)
}
