package controller

import (
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type IssueLinkController struct {
	issueLinkSvc *service.IssueLinkService
}

func NewIssueLinkController(issueLinkSvc *service.IssueLinkService) *IssueLinkController {
	return &IssueLinkController{issueLinkSvc: issueLinkSvc}
}

func (ilc *IssueLinkController) Search(c *gin.Context) {
	var idProjectFirst int64
	if raw := c.Query("idProject"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
			return
		}
		idProjectFirst = parsed
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	options, err := ilc.issueLinkSvc.Search(ctx, user.IdUser, idProjectFirst, c.Query("q"))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, options)
}

func (ilc *IssueLinkController) Info(c *gin.Context) {
	var dto model.IssueLinkInfoReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	info, err := ilc.issueLinkSvc.LoadInfo(ctx, user.IdUser, dto.Refs)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, info)
}
