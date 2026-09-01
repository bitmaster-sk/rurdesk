package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// The server never inspects `config`, so both bounds are abuse bounds, not rules: a
// realistic config is a few hundred bytes, and no real user names 100 views.
const (
	savedViewConfigMaxBytes = 8192
	savedViewMaxPerAuthor   = 100
)

type SavedViewController struct {
	repo *repository.SavedViewRepository
	acl  *service.AclService
}

func NewSavedViewController(repo *repository.SavedViewRepository, acl *service.AclService) *SavedViewController {
	return &SavedViewController{repo: repo, acl: acl}
}

func (sv *SavedViewController) List(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sv.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	views, err := sv.repo.LoadByProject(ctx, idProject, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if views == nil {
		views = []*model.SavedView{}
	}
	c.JSON(http.StatusOK, views)
}

func (sv *SavedViewController) Create(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if !sv.acl.CanCreateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	var req model.CreateSavedViewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if vErr := sv.validateSavedViewConfig(req.Config); vErr != nil {
		_ = c.Error(vErr)
		c.Status(vErr.HttpStatus())
		return
	}
	owned, err := sv.repo.CountByCreator(ctx, idProject, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if owned >= savedViewMaxPerAuthor {
		_ = c.Error(errSavedViewLimit)
		c.Status(errSavedViewLimit.HttpStatus())
		return
	}
	view, err := sv.repo.Insert(ctx, &model.SavedView{
		IdProject: idProject,
		Name:      req.Name,
		ViewType:  req.ViewType,
		Config:    req.Config,
		IsShared:  req.IsShared,
	}, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (sv *SavedViewController) Edit(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	view, ok := sv.requireManageableView(c)
	if !ok {
		return
	}
	var req model.EditSavedViewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if vErr := sv.validateSavedViewConfig(req.Config); vErr != nil {
		_ = c.Error(vErr)
		c.Status(vErr.HttpStatus())
		return
	}
	view.Name = req.Name
	view.ViewType = req.ViewType
	view.Config = req.Config
	view.IsShared = req.IsShared
	updated, err := sv.repo.Update(ctx, view, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (sv *SavedViewController) Delete(c *gin.Context) {
	view, ok := sv.requireManageableView(c)
	if !ok {
		return
	}
	if err := sv.repo.Delete(c.Request.Context(), view.IdSavedView); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

// requireManageableView loads the view named by :idSavedView and decides whether this
// caller may change or delete it: its creator, or a project owner, who is the way in once
// the creator has left. On false it has already written the response (400, 404 or 403),
// so the handler must just return.
func (sv *SavedViewController) requireManageableView(c *gin.Context) (*model.SavedView, bool) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	idSavedView, err := strconv.ParseInt(c.Param("idSavedView"), 10, 64)
	if err != nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return nil, false
	}
	view, err := sv.repo.LoadOne(ctx, idSavedView)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = c.Error(errNotFound)
		c.Status(http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		// Reporting a dead pool as "not found" makes the row vanish from the UI and
		// leaves nothing in the log pointing at the database.
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return nil, false
	}
	allowed := view.CreateBy == user.IdUser && sv.acl.CanReadProject(ctx, user.IdUser, view.IdProject)
	if !allowed && !sv.acl.CanUpdateProject(ctx, user.IdUser, view.IdProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return nil, false
	}
	return view, true
}

// validateSavedViewConfig checks the three things the server cares about in a config it
// otherwise never looks inside:
//   - size, because nothing else limits how much a user can store here;
//   - that it is a JSON object, because `null` or a list is still valid JSON but breaks
//     every client that later reads the row back;
//   - that orderColumn is a column we can really sort by, because an unknown one is
//     ignored and the view would quietly sort by "last update" instead.
func (sv *SavedViewController) validateSavedViewConfig(raw json.RawMessage) *errs.Error {
	if len(raw) > savedViewConfigMaxBytes {
		return errSavedViewConfigTooLarge
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return errSavedViewConfigInvalid
	}
	var probe struct {
		OrderColumn string `json:"orderColumn"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return errSavedViewConfigInvalid
	}
	if probe.OrderColumn != "" && !repository.IsSortColumn(probe.OrderColumn) {
		return errSavedViewBadSort
	}
	return nil
}
