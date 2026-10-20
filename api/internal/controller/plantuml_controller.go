package controller

import (
	"net/http"
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
		_ = ctx.Error(err)
		ctx.Status(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		_ = ctx.Error(errs.ErrValidation)
		ctx.Status(http.StatusBadRequest)
		return
	}
	if len(req.Source) > plantUmlMaxSourceBytes {
		_ = ctx.Error(errs.ErrValidation)
		ctx.Status(http.StatusRequestEntityTooLarge)
		return
	}
	svg, err := c.plantUmlSvc.Render(ctx.Request.Context(), req.Source)
	if err != nil {
		_ = ctx.Error(err)
		// WithMessage copies the sentinel, so match by code — errors.Is cannot
		// see through the copy.
		var appErr *errs.Error
		switch {
		case errs.As(err, &appErr) && appErr.Code == errs.ErrPlantUmlSyntax.Code:
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": appErr.Message})
		case errs.As(err, &appErr) && appErr.Code == errs.ErrPlantUmlDisabled.Code:
			ctx.Status(http.StatusNotFound)
		default:
			ctx.Status(http.StatusBadGateway)
		}
		return
	}
	ctx.JSON(http.StatusOK, model.PlantUmlRenderRes{Svg: svg})
}
