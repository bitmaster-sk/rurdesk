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

type ApiKeyController struct {
	apiKeySvc *service.ApiKeyService
	settings  *service.AppSettingsService
}

func NewApiKeyController(
	apiKeySvc *service.ApiKeyService,
	settings *service.AppSettingsService,
) *ApiKeyController {
	return &ApiKeyController{apiKeySvc: apiKeySvc, settings: settings}
}

func (kc *ApiKeyController) List(c *gin.Context) {
	// User authenticated via token cannot manage its own tokens.
	if extctx.IsApiKeyAuth(c.Request.Context()) {
		_ = c.Error(errApiKeySelfManage)
		c.Status(http.StatusForbidden)
		return
	}

	user, ok := extctx.GetUser(c.Request.Context())
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	keys, err := kc.apiKeySvc.ListUserApiKeys(c.Request.Context(), user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (kc *ApiKeyController) Create(c *gin.Context) {
	// User authenticated via token cannot manage its own tokens.
	if extctx.IsApiKeyAuth(c.Request.Context()) {
		_ = c.Error(errApiKeySelfManage)
		c.Status(http.StatusForbidden)
		return
	}

	user, ok := extctx.GetUser(c.Request.Context())
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	var req model.CreateUserApiKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	res, err := kc.apiKeySvc.CreateUserApiKey(
		c.Request.Context(),
		user.IdUser,
		kc.settings.UserApiKeyLimit(),
		&req,
	)
	if errors.Is(err, service.ErrApiKeyLimitReached) {
		_ = c.Error(errApiKeyLimitReached)
		c.Status(http.StatusConflict)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (kc *ApiKeyController) Regenerate(c *gin.Context) {
	// User authenticated via token cannot manage its own tokens.
	if extctx.IsApiKeyAuth(c.Request.Context()) {
		_ = c.Error(errApiKeySelfManage)
		c.Status(http.StatusForbidden)
		return
	}

	user, ok := extctx.GetUser(c.Request.Context())
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	idApiKey, err := strconv.ParseInt(c.Param("idApiKey"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	res, err := kc.apiKeySvc.RegenerateUserApiKey(c.Request.Context(), user.IdUser, idApiKey)
	if errors.Is(err, repository.ErrApiKeyNotFound) {
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (kc *ApiKeyController) Revoke(c *gin.Context) {
	// User authenticated via token cannot manage its own tokens.
	if extctx.IsApiKeyAuth(c.Request.Context()) {
		_ = c.Error(errApiKeySelfManage)
		c.Status(http.StatusForbidden)
		return
	}

	user, ok := extctx.GetUser(c.Request.Context())
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	idApiKey, err := strconv.ParseInt(c.Param("idApiKey"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	err = kc.apiKeySvc.RevokeUserApiKey(c.Request.Context(), user.IdUser, idApiKey)
	if errors.Is(err, repository.ErrApiKeyNotFound) {
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
