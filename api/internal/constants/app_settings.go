package constants

// App settings keys (key/value rows in projects.app_settings).
const (
	SettingTablePageSize        = "pagination.table_page_size"
	SettingKanbanPageSize       = "pagination.kanban_page_size"
	SettingGanttBacklogPageSize = "pagination.gantt_backlog_page_size"
	SettingSprintVelocityLimit  = "sprints.velocity_limit"
	SettingUserApiKeyLimit      = "user.api_key_limit"

	SettingIsAgentThinkingPersisted = "agent.is_thinking_persisted"
)

// AppNumericSettingsSpec describes a known numeric setting: its default and inclusive bounds.
type AppNumericSettingsSpec struct {
	Default int
	Min     int
	Max     int
}

// KnownAppNumericSettings is the source of truth for validation and defaults.
var KnownAppNumericSettings = map[string]AppNumericSettingsSpec{
	SettingTablePageSize:        {Default: 50, Min: 1, Max: 200},
	SettingKanbanPageSize:       {Default: 20, Min: 1, Max: 200},
	SettingGanttBacklogPageSize: {Default: 30, Min: 1, Max: 200},
	SettingSprintVelocityLimit:  {Default: 10, Min: 1, Max: 50},
	SettingUserApiKeyLimit:      {Default: 10, Min: 1, Max: 100},
}

// KnownAppBoolSettings maps each known boolean setting to its default.
var KnownAppBoolSettings = map[string]bool{
	SettingIsAgentThinkingPersisted: true,
}
