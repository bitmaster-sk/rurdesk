package constants

// App settings keys (key/value rows in projects.app_settings).
const (
	SettingTablePageSize        = "pagination.table_page_size"
	SettingKanbanPageSize       = "pagination.kanban_page_size"
	SettingGanttBacklogPageSize = "pagination.gantt_backlog_page_size"
	SettingSprintVelocityLimit  = "sprints.velocity_limit"
)

// AppSettingSpec describes a known setting: its default and inclusive bounds.
type AppSettingSpec struct {
	Default int
	Min     int
	Max     int
}

// KnownAppSettings is the source of truth for validation and defaults.
var KnownAppSettings = map[string]AppSettingSpec{
	SettingTablePageSize:        {Default: 50, Min: 1, Max: 200},
	SettingKanbanPageSize:       {Default: 20, Min: 1, Max: 200},
	SettingGanttBacklogPageSize: {Default: 30, Min: 1, Max: 200},
	SettingSprintVelocityLimit:  {Default: 10, Min: 1, Max: 50},
}
