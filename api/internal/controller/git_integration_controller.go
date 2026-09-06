package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type GitIntegrationController struct {
	gitIntRepo *repository.GitIntegrationRepository
	acl        *service.AclService
	diffCache  *githost.DiffCache
}

func NewGitIntegrationController(
	gitIntRepo *repository.GitIntegrationRepository,
	acl *service.AclService,
	diffCache *githost.DiffCache,
) *GitIntegrationController {
	return &GitIntegrationController{
		gitIntRepo: gitIntRepo,
		acl:        acl,
		diffCache:  diffCache,
	}
}

func (gc *GitIntegrationController) List(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanReadGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	integrations, err := gc.gitIntRepo.ListByProject(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	result := make([]*model.GitIntegrationRes, 0, len(integrations))
	for _, gi := range integrations {
		result = append(result, gi.ToRes())
	}
	c.JSON(http.StatusOK, result)
}

func (gc *GitIntegrationController) Get(c *gin.Context) {
	idProject, idGitIntegration, ok := gc.parseProjectAndIntegration(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanReadGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	integration, err := gc.gitIntRepo.LoadByID(ctx, idGitIntegration, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if integration == nil {
		_ = c.Error(errs.ErrGitIntegrationNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, integration.ToRes())
}

func (gc *GitIntegrationController) Create(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var req model.CreateGitIntegrationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanManageGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	key, err := githost.LoadEncryptionKey()
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	tokenEnc, nonce, err := githost.Encrypt(key, []byte(req.AccessToken))
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	integration, err := gc.gitIntRepo.Create(ctx, idProject, req.Name, req.HostType, req.BaseUrl, req.RepoPath, tokenEnc, nonce)
	if errors.Is(err, repository.ErrGitIntegrationDuplicate) {
		_ = c.Error(errs.ErrGitIntegrationDuplicate)
		c.Status(http.StatusConflict)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, integration.ToRes())
}

func (gc *GitIntegrationController) Update(c *gin.Context) {
	idProject, idGitIntegration, ok := gc.parseProjectAndIntegration(c)
	if !ok {
		return
	}

	var req model.UpdateGitIntegrationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanManageGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var tokenEnc, nonce []byte
	if req.AccessToken != "" {
		key, err := githost.LoadEncryptionKey()
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		var encErr error
		tokenEnc, nonce, encErr = githost.Encrypt(key, []byte(req.AccessToken))
		if encErr != nil {
			_ = c.Error(encErr)
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	integration, err := gc.gitIntRepo.Update(ctx, idGitIntegration, idProject, req.Name, req.HostType, req.BaseUrl, req.RepoPath, tokenEnc, nonce)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if integration == nil {
		_ = c.Error(errs.ErrGitIntegrationNotFound)
		c.Status(http.StatusNotFound)
		return
	}

	gc.diffCache.PurgeIntegration(idGitIntegration)
	c.JSON(http.StatusOK, integration.ToRes())
}

func (gc *GitIntegrationController) Delete(c *gin.Context) {
	idProject, idGitIntegration, ok := gc.parseProjectAndIntegration(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanManageGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	_, err := gc.gitIntRepo.Delete(ctx, idGitIntegration, idProject)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = c.Error(errs.ErrGitIntegrationNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	gc.diffCache.PurgeIntegration(idGitIntegration)
	c.Status(http.StatusNoContent)
}

func (gc *GitIntegrationController) GetDiff(c *gin.Context) {
	idProject, idGitIntegration, ok := gc.parseProjectAndIntegration(c)
	if !ok {
		return
	}
	mrId := c.Param("mrId")

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanReadGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	host, err := gc.loadHostForIntegration(ctx, idGitIntegration, idProject)
	if errors.Is(err, errs.ErrGitIntegrationNotFound) {
		_ = c.Error(errs.ErrGitIntegrationNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	diff, err := gc.fetchDiff(ctx, host, idGitIntegration, mrId)
	if err != nil {
		_ = c.Error(errs.ErrGitHostUnavailable)
		c.Status(http.StatusBadGateway)
		return
	}
	c.JSON(http.StatusOK, diff)
}

func (gc *GitIntegrationController) GetStatus(c *gin.Context) {
	idProject, idGitIntegration, ok := gc.parseProjectAndIntegration(c)
	if !ok {
		return
	}
	mrId := c.Param("mrId")

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !gc.acl.CanReadGitIntegration(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	host, err := gc.loadHostForIntegration(ctx, idGitIntegration, idProject)
	if errors.Is(err, errs.ErrGitIntegrationNotFound) {
		_ = c.Error(errs.ErrGitIntegrationNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	status, err := gc.fetchStatus(ctx, host, idGitIntegration, mrId)
	if err != nil {
		_ = c.Error(errs.ErrGitHostUnavailable)
		c.Status(http.StatusBadGateway)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (gc *GitIntegrationController) loadHostForIntegration(ctx context.Context, idGitIntegration, idProject int64) (githost.GitHost, error) {
	integration, err := gc.gitIntRepo.LoadByID(ctx, idGitIntegration, idProject)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, errs.ErrGitIntegrationNotFound
	}
	return githost.BuildFromIntegration(integration)
}

func (gc *GitIntegrationController) fetchStatus(ctx context.Context, host githost.GitHost, idGitIntegration int64, mrId string) (*githost.Status, error) {
	if cached, ok := gc.diffCache.GetStatus(idGitIntegration, mrId); ok {
		return cached, nil
	}
	status, err := host.GetMergeRequestStatus(ctx, mrId)
	if err != nil {
		return nil, err
	}
	gc.diffCache.SetStatus(idGitIntegration, mrId, status)
	return status, nil
}

func (gc *GitIntegrationController) fetchDiff(ctx context.Context, host githost.GitHost, idGitIntegration int64, mrId string) (*githost.Diff, error) {
	// Resolve current head SHA via status (30s cache) — the diff cache is keyed by
	// head SHA, so we need it before we can read the diff cache.
	status, err := gc.fetchStatus(ctx, host, idGitIntegration, mrId)
	if err != nil {
		return nil, err
	}

	// Only serve the cached diff for the current head SHA: a new commit yields a new
	// SHA within the status TTL and misses, so we never serve stale data.
	if status.HeadSHA != "" {
		if cached, ok := gc.diffCache.GetDiff(idGitIntegration, mrId, status.HeadSHA); ok {
			return cached, nil
		}
	}

	diff, err := host.GetMergeRequestChanges(ctx, mrId)
	if err != nil {
		return nil, err
	}
	gc.diffCache.SetDiff(idGitIntegration, mrId, diff.HeadSHA, diff)
	return diff, nil
}

func (gc *GitIntegrationController) parseProjectAndIntegration(c *gin.Context) (idProject, idGitIntegration int64, ok bool) {
	var err error
	idProject, err = strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, 0, false
	}
	idGitIntegration, err = strconv.ParseInt(c.Param("idGitIntegration"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, 0, false
	}
	return idProject, idGitIntegration, true
}
