package controller

import (
	"errors"
	"net/http"
	"strconv"

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
}

func NewSprintController(sprintRepo *repository.SprintRepository, stateRepo *repository.StateRepository, sprintSvc *service.SprintService, acl *service.AclService) *SprintController {
	return &SprintController{sprintRepo: sprintRepo, stateRepo: stateRepo, sprintSvc: sprintSvc, acl: acl}
}

func (sc *SprintController) List(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanReadProject(ctx, user.IdUser, idProject) {
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
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanManageSprint(ctx, user.IdUser, idProject) {
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
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, sprint)
}

func (sc *SprintController) Close(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
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
		c.Status(http.StatusForbidden)
		return
	}
	moved, err := sc.sprintSvc.Close(ctx, idSprint, user.IdUser)
	if errors.Is(err, service.ErrSprintClosed) {
		c.Status(http.StatusConflict)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"moved": moved})
}

func (sc *SprintController) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
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
		c.Status(http.StatusForbidden)
		return
	}
	finalIds, err := sc.stateRepo.FinalStateIds(ctx, sprint.IdProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	stats, err := sc.sprintRepo.Stats(ctx, idSprint, finalIds)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (sc *SprintController) Edit(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
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
		c.Status(http.StatusForbidden)
		return
	}
	var req model.EditSprintReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	sprint.Name = req.Name
	sprint.StartAt = req.StartAt
	sprint.EndAt = req.EndAt
	updated, err := sc.sprintRepo.Update(ctx, sprint, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (sc *SprintController) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSprint, err := strconv.ParseInt(c.Param("idSprint"), 10, 64)
	if err != nil {
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
		c.Status(http.StatusBadRequest)
		return
	}
	if !sc.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
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
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusNoContent)
}
