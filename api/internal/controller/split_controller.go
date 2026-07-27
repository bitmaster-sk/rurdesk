package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5"
)

const splitRateLimitTTL = 10 * time.Second

// SplitController handles AI-driven issue split preview and acceptance.
type SplitController struct {
	splitSvc  *service.SplitService
	aclSvc    *service.AclService
	cache     *redis.Client
	issueRepo *repository.IssueRepository
}

// NewSplitController creates a new SplitController.
func NewSplitController(
	splitSvc *service.SplitService,
	aclSvc *service.AclService,
	cache *redis.Client,
	issueRepo *repository.IssueRepository,
) *SplitController {
	return &SplitController{
		splitSvc:  splitSvc,
		aclSvc:    aclSvc,
		cache:     cache,
		issueRepo: issueRepo,
	}
}

// Preview calls the AI to propose child issues for the given issue. Nothing is saved.
func (c *SplitController) Preview(ctx *gin.Context) {
	idProject, err := strconv.ParseInt(ctx.Param("idProject"), 10, 64)
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	idIssuePublic, err := strconv.ParseInt(ctx.Param("idIssuePublic"), 10, 64)
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	var req model.SplitPreviewReq
	// Hint is optional — ignore bind error and use zero-value struct.
	_ = ctx.ShouldBindJSON(&req)

	reqCtx := ctx.Request.Context()
	user, _ := extctx.GetUser(reqCtx)

	if !c.aclSvc.CanUpdateIssue(reqCtx, user.IdUser, idProject) {
		_ = ctx.Error(errForbidden)
		ctx.Status(http.StatusForbidden)
		return
	}

	// Rate limit: 1 call per user per 10 seconds.
	rateLimitKey := fmt.Sprintf("split:rate:%d", user.IdUser)
	set, err := c.cache.SetNX(reqCtx, rateLimitKey, "1", splitRateLimitTTL).Result()
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	if !set {
		ttl, _ := c.cache.TTL(reqCtx, rateLimitKey).Result()
		retryAfter := int(ttl.Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		ctx.Header("Retry-After", strconv.Itoa(retryAfter))
		_ = ctx.Error(errs.ErrRateLimited)
		ctx.JSON(errs.ErrRateLimited.HttpStatus(), errs.ErrRateLimited)
		return
	}

	issue, err := c.issueRepo.LoadIssue(reqCtx, &repository.LoadIssueFilter{
		IdProject:     &idProject,
		IdIssuePublic: &idIssuePublic,
	})
	if err != nil {
		_ = ctx.Error(err)
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.Status(http.StatusNotFound)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("split preview: issue load error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	children, err := c.splitSvc.Preview(reqCtx, issue.IdIssue, req.Hint)
	if err != nil {
		_ = ctx.Error(err)
		var appErr *errs.Error
		if errs.As(err, &appErr) {
			ctx.JSON(appErr.HttpStatus(), appErr)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("split preview: service error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, model.SplitPreviewRes{Children: children})
}

// Accept persists the proposed child issues returned by Preview.
func (c *SplitController) Accept(ctx *gin.Context) {
	idProject, err := strconv.ParseInt(ctx.Param("idProject"), 10, 64)
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	idIssuePublic, err := strconv.ParseInt(ctx.Param("idIssuePublic"), 10, 64)
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	var req model.SplitAcceptReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	if len(req.Children) > 20 {
		ctx.Status(http.StatusBadRequest)
		return
	}

	reqCtx := ctx.Request.Context()
	user, _ := extctx.GetUser(reqCtx)

	if !c.aclSvc.CanCreateIssue(reqCtx, user.IdUser, idProject) {
		_ = ctx.Error(errForbidden)
		ctx.Status(http.StatusForbidden)
		return
	}

	issue, err := c.issueRepo.LoadIssue(reqCtx, &repository.LoadIssueFilter{
		IdProject:     &idProject,
		IdIssuePublic: &idIssuePublic,
	})
	if err != nil {
		_ = ctx.Error(err)
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.Status(http.StatusNotFound)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("split accept: issue load error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	created, err := c.splitSvc.Accept(reqCtx, idProject, issue.IdIssue, req.Children, user.IdUser)
	if err != nil {
		_ = ctx.Error(err)
		var appErr *errs.Error
		if errs.As(err, &appErr) {
			ctx.JSON(appErr.HttpStatus(), appErr)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("split accept: service error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, model.SplitAcceptRes{Children: created})
}
