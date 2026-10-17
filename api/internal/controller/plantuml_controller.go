package controller

import (
	"errors"
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
		switch {
		case errors.Is(err, errs.ErrPlantUmlDisabled):
			ctx.Status(http.StatusNotFound)
		case errors.Is(err, errs.ErrPlantUmlSyntax):
			var appErr *errs.Error
			if errs.As(err, &appErr) {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": appErr.Message})
			} else {
				ctx.Status(http.StatusUnprocessableEntity)
			}
		default:
			ctx.Status(http.StatusBadGateway)
		}
		return
	}
	ctx.JSON(http.StatusOK, model.PlantUmlRenderRes{Svg: svg})
}
