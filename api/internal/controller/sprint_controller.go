package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type SprintController struct {
	sprintRepo *repository.SprintRepository
	stateRepo  *repository.StateRepository
	sprintSvc  *service.SprintService
	acl        *service.AclService
	settings   *service.AppSettingsService
}

func NewSprintController(sprintRepo *repository.SprintRepository, stateRepo *repository.StateRepository, sprintSvc *service.SprintService, acl *service.AclService, settings *service.AppSettingsService) *SprintController {
	return &SprintController{sprintRepo: sprintRepo, stateRepo: stateRepo, sprintSvc: sprintSvc, acl: acl, settings: settings}
}

func (sc *SprintController) List(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	sprints, err := sc.sprintRepo.LoadByProject(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, sprints)
}

func (sc *SprintController) Create(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanManageSprint(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	var req model.CreateSprintReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint, err := sc.sprintSvc.Create(ctx, idProject, req, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		switch {
		case errors.Is(err, errs.ErrSprintWindow):
			c.Status(errs.ErrSprintWindow.HttpStatus())
		default:
			c.Status(http.StatusInternalServerError)
		}
		return
	}
	c.JSON(http.StatusCreated, sprint)
}

func (sc *SprintController) Close(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint, err := sc.sprintRepo.LoadOne(ctx, idSprint)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if !sc.acl.CanManageSprint(ctx, user.IdUser, sprint.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	moved, err := sc.sprintSvc.Close(ctx, idSprint, user.IdUser)
	if errors.Is(err, errs.ErrSprintClosed) {
		c.Status(errs.ErrSprintClosed.HttpStatus())
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"moved": moved})
}

func (sc *SprintController) SprintStats(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint, err := sc.sprintRepo.LoadOne(ctx, idSprint)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if !sc.acl.CanReadProject(ctx, user.IdUser, sprint.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	idsFinal, idsStart, err := sc.stateRepo.FinalAndStartStateIds(ctx, sprint.IdProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	stats, err := sc.sprintRepo.SprintStats(ctx, idSprint, idsFinal, idsStart)
	sc.respondStats(c, stats, err)
}

func (sc *SprintController) Burndown(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint, err := sc.sprintRepo.LoadOne(ctx, idSprint)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if !sc.acl.CanReadProject(ctx, user.IdUser, sprint.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	burndown, err := sc.sprintSvc.Burndown(ctx, sprint)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, burndown)
}

func (sc *SprintController) BacklogStats(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	idsFinal, idsStart, err := sc.stateRepo.FinalAndStartStateIds(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	stats, err := sc.sprintRepo.BacklogStats(ctx, idProject, idsFinal, idsStart)
	sc.respondStats(c, stats, err)
}

func (sc *SprintController) respondStats(c *gin.Context, stats *model.SprintStats, err error) {
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (sc *SprintController) Velocity(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	spec := constants.KnownAppSettings[constants.SettingSprintVelocityLimit]
	limit := 2 * sc.settings.SprintVelocityLimit()
	if raw := c.Query("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < spec.Min || limit > spec.Max {
			_ = c.Error(errs.ErrBadRequest)
			c.Status(http.StatusBadRequest)
			return
		}
	}
	idsFinal, err := sc.stateRepo.FinalStateIds(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	velocity, err := sc.sprintRepo.VelocityByProject(ctx, idProject, idsFinal, limit)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if velocity == nil {
		velocity = []*model.SprintVelocity{}
	}
	c.JSON(http.StatusOK, velocity)
}

func (sc *SprintController) Edit(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint, err := sc.sprintRepo.LoadOne(ctx, idSprint)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if !sc.acl.CanManageSprint(ctx, user.IdUser, sprint.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	var req model.EditSprintReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	updated, err := sc.sprintSvc.Edit(ctx, sprint, req, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		switch {
		case errors.Is(err, errs.ErrSprintClosed):
			c.Status(errs.ErrSprintClosed.HttpStatus())
		case errors.Is(err, errs.ErrSprintWindow):
			c.Status(errs.ErrSprintWindow.HttpStatus())
		default:
			c.Status(http.StatusInternalServerError)
		}
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (sc *SprintController) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint, err := sc.sprintRepo.LoadOne(ctx, idSprint)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if !sc.acl.CanManageSprint(ctx, user.IdUser, sprint.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err := sc.sprintRepo.Delete(ctx, idSprint); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

// AssignIssue sets/clears an issue's sprint, project-scoped by public id like other
// issue routes. A 0-row update (400) covers both "unknown issue" and "sprint from
// another project" — no 404 pre-check query.
func (sc *SprintController) AssignIssue(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, errP := strconv.ParseInt(c.Param("idProject"), 10, 64)
	idIssuePublic, errI := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
	if errP != nil || errI != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	var req model.AssignSprintReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ok, err := sc.sprintRepo.AssignIssue(ctx, idProject, idIssuePublic, req.IdSprint, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !ok {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusNoContent)
}
