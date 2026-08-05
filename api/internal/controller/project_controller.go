package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectController struct {
	projectRepo  *repository.ProjectRepository
	teamRepo     *repository.TeamRepository
	severityRepo *repository.SeverityRepository
	stateRepo    *repository.StateRepository
	acl          *service.AclService
	pool         *pgxpool.Pool
}

func NewProjectController(
	pr *repository.ProjectRepository,
	tr *repository.TeamRepository,
	sr *repository.SeverityRepository,
	str *repository.StateRepository,
	acl *service.AclService,
	pool *pgxpool.Pool,
) *ProjectController {
	return &ProjectController{
		projectRepo:  pr,
		teamRepo:     tr,
		severityRepo: sr,
		stateRepo:    str,
		acl:          acl,
		pool:         pool,
	}
}

func (pc *ProjectController) CreateProject(c *gin.Context) {
	var dto model.CreateProjectReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	project := &model.Project{Name: dto.Name, Color: dto.Color}

	err := extctx.RunInTx(ctx, pc.pool, func(ctx context.Context) error {
		var err error
		project, err = pc.projectRepo.InsertProject(ctx, project)
		if err != nil {
			return err
		}
		if err = pc.severityRepo.InsertDefaultSeverities(ctx, project.IdProject); err != nil {
			return err
		}
		if err = pc.stateRepo.InsertDefaultStates(ctx, project.IdProject); err != nil {
			return err
		}
		// Creator is always added as owner, even when a team is also added.
		if err = pc.projectRepo.InsertProjectUser(ctx, project.IdProject, user.IdUser, model.RoleOwner); err != nil {
			return err
		}
		if dto.IdTeam != nil {
			if !pc.acl.CanReadTeam(ctx, user.IdUser, *dto.IdTeam) {
				return errForbidden
			}
			return pc.projectRepo.InsertProjectTeam(ctx, project.IdProject, *dto.IdTeam, model.RoleMember)
		}
		return nil
	})
	if err == errForbidden {
		_ = c.Error(err)
		c.Status(http.StatusForbidden)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (pc *ProjectController) UpdateProject(c *gin.Context) {
	var dto model.EditProjectReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanUpdateProject(ctx, user.IdUser, dto.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	project, err := pc.projectRepo.LoadProject(ctx, dto.IdProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	project.Name = dto.Name
	project.Color = dto.Color
	project.IdStateDefault = dto.IdStateDefault
	project.IdSeverityDefault = dto.IdSeverityDefault

	if err := pc.projectRepo.UpdateProject(ctx, project); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (pc *ProjectController) DeleteProject(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanDeleteProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if err := pc.projectRepo.DeleteProject(ctx, idProject); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (pc *ProjectController) GetProjects(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	projects, err := pc.acl.LoadVisibleProjects(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, projects)
}

func (pc *ProjectController) GetProject(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	project, err := pc.projectRepo.LoadProject(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, project)
}

func (pc *ProjectController) GetUserRole(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	role, hasAccess := pc.acl.GetProjectRole(ctx, user.IdUser, idProject)
	if !hasAccess {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	c.JSON(http.StatusOK, model.UserRoleRes{Role: role})
}

func (pc *ProjectController) GetProjectMembers(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	members, err := pc.projectRepo.LoadProjectsMembers(ctx, []int64{idProject})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, members)
}
