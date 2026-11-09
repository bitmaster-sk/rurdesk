package constants

import "slices"

// App settings keys (key/value rows in projects.app_settings).
const (
	SettingTablePageSize        = "pagination.table_page_size"
	SettingKanbanPageSize       = "pagination.kanban_page_size"
	SettingGanttBacklogPageSize = "pagination.gantt_backlog_page_size"
	SettingSprintVelocityLimit  = "sprints.velocity_limit"
	SettingUserApiKeyLimit      = "user.api_key_limit"
	SettingAttachmentMaxSizeMb  = "attachment.max_size_mb"

	SettingIsAgentThinkingPersisted = "agent.is_thinking_persisted"
	SettingAgentThinkingMaxKb       = "agent.thinking_max_kb"

	SettingWikiAlwaysTokenLimit = "wiki.always_token_limit"

	SettingPlantUmlServerUrl = "plantuml.server_url"
)

// AppNumericSettingsSpec describes a known numeric setting: its default and inclusive bounds.
type AppNumericSettingsSpec struct {
	Default int
	Min     int
	Max     int
}

// KnownAppNumericSettings is the source of truth for validation and defaults.
var KnownAppNumericSettings = withAgentMaxTurnsSettings(map[string]AppNumericSettingsSpec{
	SettingTablePageSize:        {Default: 50, Min: 1, Max: 200},
	SettingKanbanPageSize:       {Default: 20, Min: 1, Max: 200},
	SettingGanttBacklogPageSize: {Default: 30, Min: 1, Max: 200},
	SettingSprintVelocityLimit:  {Default: 10, Min: 1, Max: 50},
	SettingUserApiKeyLimit:      {Default: 10, Min: 1, Max: 100},
	SettingAgentThinkingMaxKb:   {Default: 1024, Min: 64, Max: 10240},
	SettingAttachmentMaxSizeMb:  {Default: 25, Min: 1, Max: 100},
	// Max 60000 mirrors wikiPromptMaxTokens (service/wiki_prompt.go): a larger
	// limit could never reach an agent prompt anyway.
	SettingWikiAlwaysTokenLimit: {Default: 12000, Min: 0, Max: 60000},
})

// KnownAppBoolSettings maps each known boolean setting to its default.
var KnownAppBoolSettings = map[string]bool{
	SettingIsAgentThinkingPersisted: true,
}

// KnownAppStringSettings maps each known string setting to its default.
var KnownAppStringSettings = withAgentEffortSettings(map[string]string{
	SettingPlantUmlServerUrl: "",
})

func SettingAgentMaxTurns(stage string) string {
	return "agent.max_turns." + stage
}

func SettingAgentEffort(stage string) string {
	return "agent.effort." + stage
}

func IsAgentEffortSetting(key string) bool {
	return slices.ContainsFunc(SkillStages, func(stage string) bool { return SettingAgentEffort(stage) == key })
}

func withAgentMaxTurnsSettings(specs map[string]AppNumericSettingsSpec) map[string]AppNumericSettingsSpec {
	for _, stage := range SkillStages {
		spec := AppNumericSettingsSpec{Default: 250, Min: 1, Max: MaxAgentTurns}
		if stage == StageImplementation {
			spec.Default = 500
		}
		specs[SettingAgentMaxTurns(stage)] = spec
	}
	return specs
}

func withAgentEffortSettings(defaults map[string]string) map[string]string {
	for _, stage := range SkillStages {
		defaults[SettingAgentEffort(stage)] = ""
	}
	return defaults
}
