package model

// AppSettingsRes is the resolved view of all known settings (returned by GET /settings).
type AppSettingsRes struct {
	TablePageSize            int    `json:"tablePageSize"`
	KanbanPageSize           int    `json:"kanbanPageSize"`
	GanttBacklogPageSize     int    `json:"ganttBacklogPageSize"`
	SprintVelocityLimit      int    `json:"sprintVelocityLimit"`
	UserApiKeyLimit          int    `json:"userApiKeyLimit"`
	IsAgentThinkingPersisted bool   `json:"isAgentThinkingPersisted"`
	AgentThinkingMaxKb       int    `json:"agentThinkingMaxKb"`
	AttachmentMaxSizeMb      int    `json:"attachmentMaxSizeMb"`
	AgentMaxTurnsPlan        int    `json:"agentMaxTurnsPlan"`
	AgentMaxTurnsImplement   int    `json:"agentMaxTurnsImplement"`
	PlantUmlServerUrl        string `json:"plantUmlServerUrl"`
}

// UpdateAppSettingsReq is a partial update — nil fields are left unchanged.
type UpdateAppSettingsReq struct {
	TablePageSize            *int    `json:"tablePageSize"`
	KanbanPageSize           *int    `json:"kanbanPageSize"`
	GanttBacklogPageSize     *int    `json:"ganttBacklogPageSize"`
	SprintVelocityLimit      *int    `json:"sprintVelocityLimit"`
	UserApiKeyLimit          *int    `json:"userApiKeyLimit"`
	IsAgentThinkingPersisted *bool   `json:"isAgentThinkingPersisted"`
	AgentThinkingMaxKb       *int    `json:"agentThinkingMaxKb"`
	AttachmentMaxSizeMb      *int    `json:"attachmentMaxSizeMb"`
	AgentMaxTurnsPlan        *int    `json:"agentMaxTurnsPlan"`
	AgentMaxTurnsImplement   *int    `json:"agentMaxTurnsImplement"`
	PlantUmlServerUrl        *string `json:"plantUmlServerUrl"`
}
