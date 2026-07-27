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

type PhaseStateMapController struct {
	phaseStateMapRepo *repository.PhaseStateMapRepository
	stateRepo         *repository.StateRepository
	acl               *service.AclService
}

func NewPhaseStateMapController(
	phaseStateMapRepo *repository.PhaseStateMapRepository,
	stateRepo *repository.StateRepository,
	acl *service.AclService,
) *PhaseStateMapController {
	return &PhaseStateMapController{
		phaseStateMapRepo: phaseStateMapRepo,
		stateRepo:         stateRepo,
		acl:               acl,
	}
}

func (ctrl *PhaseStateMapController) GetMappings(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ctrl.acl.CanManageAgentPhaseStateMap(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	mappings, err := ctrl.phaseStateMapRepo.LoadMappings(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, mappings)
}

func (ctrl *PhaseStateMapController) ReplaceMappings(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ctrl.acl.CanManageAgentPhaseStateMap(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var dto model.ReplacePhaseStateMappingsReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := dto.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, entry := range dto.Mappings {
		if entry.IdState == nil {
			continue
		}
		state, stateErr := ctrl.stateRepo.LoadState(ctx, idProject, *entry.IdState)
		if stateErr != nil || state == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "state not found or does not belong to this project"})
			return
		}
	}

	result, err := ctrl.phaseStateMapRepo.ReplaceMappings(ctx, idProject, dto.Mappings)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, result)
}
