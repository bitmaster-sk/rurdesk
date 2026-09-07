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

const qualityRateLimitTTL = 5 * time.Second

// QualityController handles AI-driven issue quality checks.
type QualityController struct {
	qualitySvc *service.QualityService
	aclSvc     *service.AclService
	issueRepo  *repository.IssueRepository
	cache      *redis.Client
}

// NewQualityController creates a new QualityController.
func NewQualityController(
	qualitySvc *service.QualityService,
	aclSvc *service.AclService,
	issueRepo *repository.IssueRepository,
	cache *redis.Client,
) *QualityController {
	return &QualityController{
		qualitySvc: qualitySvc,
		aclSvc:     aclSvc,
		issueRepo:  issueRepo,
		cache:      cache,
	}
}

// isRateLimited enforces the per-user rate limit, writing the response if limited.
func (c *QualityController) isRateLimited(ctx *gin.Context, userID int64) bool {
	reqCtx := ctx.Request.Context()
	rateLimitKey := fmt.Sprintf("quality:rate:%d", userID)

	set, err := c.cache.SetNX(reqCtx, rateLimitKey, "1", qualityRateLimitTTL).Result()
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusInternalServerError)
		return true
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
		return true
	}
	return false
}

// Preview runs an AI quality check without persisting — for pre-save use.
// POST /api/private/project/:idProject/quality
func (c *QualityController) Preview(ctx *gin.Context) {
	idProject, err := strconv.ParseInt(ctx.Param("idProject"), 10, 64)
	if err != nil {
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}

	var req model.QualityCheckReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(err)
		ctx.JSON(errs.ErrBadRequest.HttpStatus(), errs.ErrBadRequest)
		return
	}

	reqCtx := ctx.Request.Context()
	user, _ := extctx.GetUser(reqCtx)

	if !c.aclSvc.CanUpdateIssue(reqCtx, user.IdUser, idProject) {
		_ = ctx.Error(errs.ErrForbidden)
		ctx.Status(http.StatusForbidden)
		return
	}

	if c.isRateLimited(ctx, user.IdUser) {
		return
	}

	report, err := c.qualitySvc.Preview(reqCtx, req.Title, req.Description)
	if err != nil {
		_ = ctx.Error(err)
		var appErr *errs.Error
		if errs.As(err, &appErr) {
			ctx.JSON(appErr.HttpStatus(), appErr)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("quality preview: service error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, report)
}

// Check runs an AI quality check on an existing issue and persists the result.
// POST /api/private/project/:idProject/issue/:idIssuePublic/quality
func (c *QualityController) Check(ctx *gin.Context) {
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

	var req model.QualityCheckReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		_ = ctx.Error(err)
		ctx.JSON(errs.ErrBadRequest.HttpStatus(), errs.ErrBadRequest)
		return
	}

	reqCtx := ctx.Request.Context()
	user, _ := extctx.GetUser(reqCtx)

	if !c.aclSvc.CanUpdateIssue(reqCtx, user.IdUser, idProject) {
		_ = ctx.Error(errs.ErrForbidden)
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
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("quality check: issue load error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if c.isRateLimited(ctx, user.IdUser) {
		return
	}

	report, err := c.qualitySvc.Check(reqCtx, issue.IdIssue, req.Title, req.Description, user.IdUser)
	if err != nil {
		_ = ctx.Error(err)
		var appErr *errs.Error
		if errs.As(err, &appErr) {
			ctx.JSON(appErr.HttpStatus(), appErr)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("quality check: service error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, report)
}

// GetQuality returns the persisted quality report for an issue (no AI call).
// GET /api/private/project/:idProject/issue/:idIssuePublic/quality
func (c *QualityController) GetQuality(ctx *gin.Context) {
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

	reqCtx := ctx.Request.Context()
	user, _ := extctx.GetUser(reqCtx)

	if !c.aclSvc.CanReadProject(reqCtx, user.IdUser, idProject) {
		_ = ctx.Error(errs.ErrForbidden)
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
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("quality get: issue load error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	report, err := c.qualitySvc.GetForIssue(reqCtx, issue.IdIssue)
	if err != nil {
		_ = ctx.Error(err)
		var appErr *errs.Error
		if errs.As(err, &appErr) {
			ctx.JSON(appErr.HttpStatus(), appErr)
			return
		}
		extctx.GetLogger(reqCtx).Error().Err(err).Msg("quality get: service error")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, report)
}
