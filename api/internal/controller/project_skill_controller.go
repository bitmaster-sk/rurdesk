package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type ProjectSkillController struct {
	projectSkillRepo    *repository.ProjectSkillRepository
	projectSkillService *service.ProjectSkillService
	acl                 *service.AclService
}

func NewProjectSkillController(
	projectSkillRepo *repository.ProjectSkillRepository,
	projectSkillService *service.ProjectSkillService,
	acl *service.AclService,
) *ProjectSkillController {
	return &ProjectSkillController{
		projectSkillRepo:    projectSkillRepo,
		projectSkillService: projectSkillService,
		acl:                 acl,
	}
}

// Member-readable on purpose: the assignee dock prefills its chips from this.
func (ctrl *ProjectSkillController) Get(c *gin.Context) {
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

	entries, err := ctrl.projectSkillRepo.Load(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (ctrl *ProjectSkillController) Replace(c *gin.Context) {
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

	var entries []model.UpdateProjectSkillReq
	if err := c.ShouldBindJSON(&entries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := ctrl.projectSkillService.Replace(ctx, idProject, entries)
	var appErr *errs.Error
	if errs.As(err, &appErr) {
		_ = c.Error(appErr)
		c.Status(appErr.HttpStatus())
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, result)
}
