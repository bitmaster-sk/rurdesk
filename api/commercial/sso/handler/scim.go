package handler

import (
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) SetScimEnabled(c *gin.Context) {
	var req SetScimEnabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	ctx := c.Request.Context()
	if *req.IsEnabled && !h.isLicensed(ctx) {
		controller.ResponseErr(c, errs.ErrLicenseRequired)
		return
	}
	if err := h.deps.Scim.SetEnabled(ctx, *req.IsEnabled); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) GenerateScimToken(c *gin.Context) {
	ctx := c.Request.Context()
	acting, _ := extctx.GetUser(ctx)
	token, err := h.deps.Scim.GenerateToken(ctx, acting.IdUser)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	controller.ResponseOk(c, ScimTokenRes{Token: token})
}

func (h *Handler) DeleteScimToken(c *gin.Context) {
	if err := h.deps.Scim.DeleteToken(c.Request.Context()); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}
