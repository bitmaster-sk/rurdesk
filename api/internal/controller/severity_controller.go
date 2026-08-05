package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SeverityController struct {
	projectRepo  *repository.ProjectRepository
	severityRepo *repository.SeverityRepository
	severitySvc  *service.SeverityService
	acl          *service.AclService
	pool         *pgxpool.Pool
}

func NewSeverityController(sr *repository.SeverityRepository, svc *service.SeverityService, acl *service.AclService, pr *repository.ProjectRepository, pool *pgxpool.Pool) *SeverityController {
	return &SeverityController{
		projectRepo:  pr,
		severityRepo: sr,
		severitySvc:  svc,
		acl:          acl,
		pool:         pool,
	}
}

func (sc *SeverityController) GetSeverities(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	projects, err := sc.acl.LoadVisibleProjects(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if len(projects) == 0 {
		c.JSON(http.StatusOK, []*model.Severity{})
		return
	}
	idsProject := make([]int64, len(projects))
	for i, p := range projects {
		idsProject[i] = p.IdProject
	}

	severities, err := sc.severityRepo.LoadSeverities(ctx, idsProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, severities)
}

func (sc *SeverityController) CreateSeverity(c *gin.Context) {
	var dto model.CreateSeverityReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var severity *model.Severity
	err := extctx.RunInTx(ctx, sc.pool, func(ctx context.Context) error {
		if !sc.acl.CanCreateSeverity(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		sev := &model.Severity{
			IdProject: dto.IdProject,
			Title:     dto.Title,
			Color:     dto.Color,
			Protected: false,
		}
		var err error
		severity, err = sc.severityRepo.InsertSeverity(ctx, sev)
		if err != nil {
			return err
		}
		return sc.severityRepo.InsertProjectSeverity(ctx, severity)
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
	c.JSON(http.StatusOK, severity)
}

func (sc *SeverityController) EditSeverity(c *gin.Context) {
	idSeverity, err := strconv.ParseInt(c.Param("idSeverity"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.EditSeverityReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	dto.IdSeverity = idSeverity

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var severity *model.Severity
	err = extctx.RunInTx(ctx, sc.pool, func(ctx context.Context) error {
		if !sc.acl.CanUpdateSeverity(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		var err error
		severity, err = sc.severityRepo.LoadSeverity(ctx, dto.IdProject, dto.IdSeverity)
		if err != nil {
			return err
		}
		severity.Title = dto.Title
		severity.Color = dto.Color
		severity.OrderRank = dto.OrderRank

		if severity.Protected {
			oldIdSeverity := severity.IdSeverity
			if err := sc.severityRepo.DeleteProjectSeverity(ctx, severity); err != nil {
				return err
			}
			severity, err = sc.severityRepo.InsertSeverity(ctx, severity)
			if err != nil {
				return err
			}
			if err := sc.severityRepo.InsertProjectSeverity(ctx, severity); err != nil {
				return err
			}
			// InsertProjectSeverity appends at the end; honor the requested order
			// so reordering a protected default keeps its position after the fork.
			if err := sc.severityRepo.UpdateProjectSeverity(ctx, severity); err != nil {
				return err
			}
			// Migrate the project's issues onto the forked copy so they are not
			// orphaned on the now-unmapped shared default.
			return sc.severityRepo.ReassignIssuesSeverity(ctx, severity.IdProject, oldIdSeverity, &severity.IdSeverity)
		}
		severity, err = sc.severityRepo.UpdateSeverity(ctx, severity)
		if err != nil {
			return err
		}
		return sc.severityRepo.UpdateProjectSeverity(ctx, severity)
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
	c.JSON(http.StatusOK, severity)
}

// GetSeverityUsage reports what still points at the severity, for the delete dialog.
func (sc *SeverityController) GetSeverityUsage(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idSeverity, err := strconv.ParseInt(c.Param("idSeverity"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !sc.acl.CanDeleteSeverity(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	usage, err := sc.severityRepo.LoadSeverityUsage(ctx, idProject, idSeverity)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func (sc *SeverityController) DeleteSeverity(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idSeverity, err := strconv.ParseInt(c.Param("idSeverity"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !sc.acl.CanDeleteSeverity(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var migrateTo *int64
	unassign := false
	if raw, hasIntent := c.GetQuery("migrateTo"); hasIntent {
		if raw == "null" {
			unassign = true
		} else {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				_ = c.Error(errs.ErrBadRequest)
				c.Status(http.StatusBadRequest)
				return
			}
			migrateTo = &id
		}
	}

	err = sc.severitySvc.DeleteWithMigration(ctx, idProject, idSeverity, migrateTo, unassign)
	var appErr *errs.Error
	switch {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, pgx.ErrNoRows):
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case errs.As(err, &appErr):
		_ = c.Error(appErr)
		c.Status(appErr.HttpStatus())
	default:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	}
}
