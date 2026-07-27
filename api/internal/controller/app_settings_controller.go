package controller

import (
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type AppSettingsController struct {
	settings *service.AppSettingsService
}

func NewAppSettingsController(settings *service.AppSettingsService) *AppSettingsController {
	return &AppSettingsController{settings: settings}
}

// Get returns the resolved settings. Readable by any authenticated user.
func (sc *AppSettingsController) Get(c *gin.Context) {
	c.JSON(http.StatusOK, model.AppSettingsRes{
		TablePageSize:        sc.settings.TablePageSize(),
		KanbanPageSize:       sc.settings.KanbanPageSize(),
		GanttBacklogPageSize: sc.settings.GanttBacklogPageSize(),
	})
}

// Update applies a partial change. Admin-only (route is under the admin group).
func (sc *AppSettingsController) Update(c *gin.Context) {
	var req model.UpdateAppSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	changes := map[string]int{}
	if req.TablePageSize != nil {
		changes[constants.SettingTablePageSize] = *req.TablePageSize
	}
	if req.KanbanPageSize != nil {
		changes[constants.SettingKanbanPageSize] = *req.KanbanPageSize
	}
	if req.GanttBacklogPageSize != nil {
		changes[constants.SettingGanttBacklogPageSize] = *req.GanttBacklogPageSize
	}
	if err := sc.settings.Update(c.Request.Context(), changes); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	sc.Get(c)
}
