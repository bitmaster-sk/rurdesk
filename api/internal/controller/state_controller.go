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

type StateController struct {
	projectRepo *repository.ProjectRepository
	stateRepo   *repository.StateRepository
	stateSvc    *service.StateService
	acl         *service.AclService
	pool        *pgxpool.Pool
}

func NewStateController(sr *repository.StateRepository, svc *service.StateService, acl *service.AclService, pr *repository.ProjectRepository, pool *pgxpool.Pool) *StateController {
	return &StateController{
		projectRepo: pr,
		stateRepo:   sr,
		stateSvc:    svc,
		acl:         acl,
		pool:        pool,
	}
}

func (sc *StateController) GetStates(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	projects, err := sc.projectRepo.LoadProjects(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if len(projects) == 0 {
		c.JSON(http.StatusOK, []*model.State{})
		return
	}
	idsProject := make([]int64, len(projects))
	for i, p := range projects {
		idsProject[i] = p.IdProject
	}

	states, err := sc.stateRepo.LoadStates(ctx, idsProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, states)
}

func (sc *StateController) CreateState(c *gin.Context) {
	var dto model.CreateStateReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var state *model.State
	err := extctx.RunInTx(ctx, sc.pool, func(ctx context.Context) error {
		if !sc.acl.CanCreateState(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		s := &model.State{
			IdProject: dto.IdProject,
			Name:      dto.Name,
			Start:     dto.Start,
			Final:     dto.Final,
			Protected: false,
		}
		var err error
		state, err = sc.stateRepo.InsertState(ctx, s)
		if err != nil {
			return err
		}
		return sc.stateRepo.InsertProjectState(ctx, state)
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
	c.JSON(http.StatusOK, state)
}

func (sc *StateController) EditState(c *gin.Context) {
	idState, err := strconv.ParseInt(c.Param("idState"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.EditStateReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	dto.IdState = idState

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var state *model.State
	err = extctx.RunInTx(ctx, sc.pool, func(ctx context.Context) error {
		if !sc.acl.CanUpdateState(ctx, user.IdUser, dto.IdProject) {
			return errForbidden
		}
		var err error
		state, err = sc.stateRepo.LoadState(ctx, dto.IdProject, dto.IdState)
		if err != nil {
			return err
		}
		state.Name = dto.Name
		state.Start = dto.Start
		state.Final = dto.Final
		state.OrderRank = dto.OrderRank

		if state.Protected {
			oldIdState := state.IdState
			if err := sc.stateRepo.DeleteProjectState(ctx, state); err != nil {
				return err
			}
			state, err = sc.stateRepo.InsertState(ctx, state)
			if err != nil {
				return err
			}
			if err := sc.stateRepo.InsertProjectState(ctx, state); err != nil {
				return err
			}
			// InsertProjectState appends at the end; honor the requested order so
			// reordering a protected default keeps its position after the fork.
			if err := sc.stateRepo.UpdateProjectState(ctx, state); err != nil {
				return err
			}
			// Migrate the project's issues onto the forked copy so they are not
			// orphaned on the now-unmapped shared default.
			return sc.stateRepo.ReassignIssuesState(ctx, state.IdProject, oldIdState, &state.IdState)
		}
		state, err = sc.stateRepo.UpdateState(ctx, state)
		if err != nil {
			return err
		}
		return sc.stateRepo.UpdateProjectState(ctx, state)
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
	c.JSON(http.StatusOK, state)
}

// GetStateUsage reports what still points at the state, for the delete dialog.
func (sc *StateController) GetStateUsage(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idState, err := strconv.ParseInt(c.Param("idState"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !sc.acl.CanDeleteState(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	usage, err := sc.stateRepo.LoadStateUsage(ctx, idProject, idState)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func (sc *StateController) DeleteState(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idState, err := strconv.ParseInt(c.Param("idState"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	// was inside RunInTx; read-only Redis-cached check, so fail fast instead
	if !sc.acl.CanDeleteState(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// single intent param: migrateTo=<id> repoints, the literal migrateTo=null
	// unassigns, absent → 409 when in use, anything else (incl. empty) → 400
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

	err = sc.stateSvc.DeleteWithMigration(ctx, idProject, idState, migrateTo, unassign)
	var appErr *errs.Error
	switch {
	case err == nil:
		c.Status(http.StatusOK)
	case errors.Is(err, pgx.ErrNoRows):
		// state not mapped to this project → 404, not 500
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case errs.As(err, &appErr):
		// errs sentinels carry their own status (409 in-use, 422 bad target)
		_ = c.Error(appErr)
		c.Status(appErr.HttpStatus())
	default:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	}
}
