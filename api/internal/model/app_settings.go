package model

// AppSettingsRes is the resolved view of all known settings (returned by GET /settings).
type AppSettingsRes struct {
	TablePageSize        int `json:"tablePageSize"`
	KanbanPageSize       int `json:"kanbanPageSize"`
	GanttBacklogPageSize int `json:"ganttBacklogPageSize"`
}

// UpdateAppSettingsReq is a partial update — nil fields are left unchanged.
type UpdateAppSettingsReq struct {
	TablePageSize        *int `json:"tablePageSize"`
	KanbanPageSize       *int `json:"kanbanPageSize"`
	GanttBacklogPageSize *int `json:"ganttBacklogPageSize"`
}
