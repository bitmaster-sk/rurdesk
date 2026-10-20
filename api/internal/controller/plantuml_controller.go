package controller

import (
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

const plantUmlMaxSourceBytes = 64 << 10

type PlantUmlController struct {
	plantUmlSvc *service.PlantUmlService
}

func NewPlantUmlController(plantUmlSvc *service.PlantUmlService) *PlantUmlController {
	return &PlantUmlController{plantUmlSvc: plantUmlSvc}
}

func (c *PlantUmlController) Render(ctx *gin.Context) {
	var req model.PlantUmlRenderReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ResponseErr(ctx, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		ResponseErr(ctx, errs.ErrBadRequest.WithMessage("source must not be empty"))
		return
	}
	if len(req.Source) > plantUmlMaxSourceBytes {
		ResponseErr(ctx, errs.ErrPlantUmlSourceTooLarge)
		return
	}
	svg, err := c.plantUmlSvc.Render(ctx.Request.Context(), req.Source)
	if err != nil {
		ResponseErr(ctx, err)
		return
	}
	ResponseOk(ctx, model.PlantUmlRenderRes{Svg: svg})
}
