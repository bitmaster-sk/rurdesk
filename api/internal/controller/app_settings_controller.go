package controller

import (
	"context"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppSettingsController struct {
	settings *service.AppSettingsService
	pool     *pgxpool.Pool
}

func NewAppSettingsController(settings *service.AppSettingsService, pool *pgxpool.Pool) *AppSettingsController {
	return &AppSettingsController{settings: settings, pool: pool}
}

// Get returns the resolved settings. Readable by any authenticated user.
func (sc *AppSettingsController) Get(c *gin.Context) {
	c.JSON(http.StatusOK, model.AppSettingsRes{
		TablePageSize:            sc.settings.TablePageSize(),
		KanbanPageSize:           sc.settings.KanbanPageSize(),
		GanttBacklogPageSize:     sc.settings.GanttBacklogPageSize(),
		SprintVelocityLimit:      sc.settings.SprintVelocityLimit(),
		UserApiKeyLimit:          sc.settings.UserApiKeyLimit(),
		IsAgentThinkingPersisted: sc.settings.IsAgentThinkingPersisted(),
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
	numericChanges := map[string]int{}
	if req.TablePageSize != nil {
		numericChanges[constants.SettingTablePageSize] = *req.TablePageSize
	}
	if req.KanbanPageSize != nil {
		numericChanges[constants.SettingKanbanPageSize] = *req.KanbanPageSize
	}
	if req.GanttBacklogPageSize != nil {
		numericChanges[constants.SettingGanttBacklogPageSize] = *req.GanttBacklogPageSize
	}
	if req.SprintVelocityLimit != nil {
		numericChanges[constants.SettingSprintVelocityLimit] = *req.SprintVelocityLimit
	}
	if req.UserApiKeyLimit != nil {
		numericChanges[constants.SettingUserApiKeyLimit] = *req.UserApiKeyLimit
	}
	boolChanges := map[string]bool{}
	if req.IsAgentThinkingPersisted != nil {
		boolChanges[constants.SettingIsAgentThinkingPersisted] = *req.IsAgentThinkingPersisted
	}
	ctx := c.Request.Context()
	if err := extctx.RunInTx(ctx, sc.pool, func(ctx context.Context) error {
		return sc.settings.Update(ctx, numericChanges, boolChanges)
	}); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	sc.Get(c)
}
