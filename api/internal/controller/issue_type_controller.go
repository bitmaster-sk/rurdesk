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

type IssueTypeController struct {
	issueTypeRepo *repository.IssueTypeRepository
	issueTypeSvc  *service.IssueTypeService
	acl           *service.AclService
	pool          *pgxpool.Pool
}

func NewIssueTypeController(itr *repository.IssueTypeRepository, svc *service.IssueTypeService, acl *service.AclService, pool *pgxpool.Pool) *IssueTypeController {
	return &IssueTypeController{
		issueTypeRepo: itr,
		issueTypeSvc:  svc,
		acl:           acl,
		pool:          pool,
	}
}

func (itc *IssueTypeController) GetIssueTypes(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	projects, err := itc.acl.LoadVisibleProjects(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if len(projects) == 0 {
		c.JSON(http.StatusOK, []*model.IssueType{})
		return
	}
	idsProject := make([]int64, len(projects))
	for i, p := range projects {
		idsProject[i] = p.IdProject
	}

	issueTypes, err := itc.issueTypeRepo.LoadIssueTypes(ctx, idsProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, issueTypes)
}

func (itc *IssueTypeController) CreateIssueType(c *gin.Context) {
	var dto model.CreateIssueTypeReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var issueType *model.IssueType
	err := extctx.RunInTx(ctx, itc.pool, func(ctx context.Context) error {
		if !itc.acl.CanCreateIssueType(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		var err error
		issueType, err = itc.issueTypeRepo.InsertIssueType(ctx, &model.IssueType{
			IdProject: dto.IdProject,
			Name:      dto.Name,
		})
		return err
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
	c.JSON(http.StatusOK, issueType)
}

func (itc *IssueTypeController) EditIssueType(c *gin.Context) {
	idIssueType, err := strconv.ParseInt(c.Param("idIssueType"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.EditIssueTypeReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	dto.IdIssueType = idIssueType

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var issueType *model.IssueType
	err = extctx.RunInTx(ctx, itc.pool, func(ctx context.Context) error {
		if !itc.acl.CanUpdateIssueType(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		var err error
		issueType, err = itc.issueTypeRepo.LoadIssueType(ctx, dto.IdProject, dto.IdIssueType)
		if err != nil {
			return err
		}
		issueType.Name = dto.Name
		issueType.OrderRank = dto.OrderRank

		issueType, err = itc.issueTypeRepo.UpdateIssueType(ctx, issueType)
		if err != nil {
			return err
		}
		return itc.issueTypeRepo.MoveIssueType(ctx, issueType)
	})
	if err == errForbidden {
		_ = c.Error(err)
		c.Status(http.StatusForbidden)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, issueType)
}

func (itc *IssueTypeController) GetIssueTypeUsage(c *gin.Context) {
	idProject, idIssueType, ok := itc.parseParams(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !itc.acl.CanDeleteIssueType(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	usage, err := itc.issueTypeRepo.LoadIssueTypeUsage(ctx, idProject, idIssueType)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func (itc *IssueTypeController) DeleteIssueType(c *gin.Context) {
	idProject, idIssueType, ok := itc.parseParams(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !itc.acl.CanDeleteIssueType(ctx, user.IdUser, idProject) {
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

	err := itc.issueTypeSvc.DeleteWithMigration(ctx, idProject, idIssueType, migrateTo, unassign)
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

func (itc *IssueTypeController) parseParams(c *gin.Context) (idProject, idIssueType int64, ok bool) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, 0, false
	}
	idIssueType, err = strconv.ParseInt(c.Param("idIssueType"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, 0, false
	}
	return idProject, idIssueType, true
}
