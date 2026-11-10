package model

import (
	"slices"
	"time"
)

type AgentGateway struct {
	IdGateway             int64                        `json:"idGateway"         db:"id_gateway"`
	IdUserAgent           int64                        `json:"idUserAgent"       db:"id_user_agent"`
	GatewayUrl            string                       `json:"gatewayUrl"        db:"gateway_url"`
	WebhookSecret         []byte                       `json:"-"                 db:"webhook_secret"`
	Models                []string                     `json:"models"            db:"models"`
	ModelDefault          *string                      `json:"modelDefault"      db:"model_default"`
	ModelsReportedAt      *time.Time                   `json:"modelsReportedAt"  db:"models_reported_at"`
	MaxTurnsLimit         *int                         `json:"maxTurnsLimit"     db:"max_turns_limit"`
	StageDefaults         map[string]AgentStageOptions `json:"stageDefaults"     db:"stage_defaults"`
	ModelsWithoutThinking []string                     `json:"modelsWithoutThinking" db:"models_without_thinking"`
	CreatedAt             time.Time                    `json:"createdAt"         db:"created_at"`
}

func (gw *AgentGateway) IsSupportedModel(model string) bool {
	return slices.Contains(gw.Models, model)
}

// The stage list must stay equal to constants.SkillStages.
type CreateAgentGatewayReq struct {
	GatewayUrl    string                       `json:"gatewayUrl"    binding:"required,url"`
	StageDefaults map[string]AgentStageOptions `json:"stageDefaults" binding:"omitempty,dive,keys,oneof=brainstorming design implementation_plan implementation release_notes,endkeys"`
}

func (req CreateAgentGatewayReq) StoredStageDefaults() map[string]AgentStageOptions {
	defaults := map[string]AgentStageOptions{}
	for stage, stageDefault := range req.StageDefaults {
		if stageDefault.MaxTurns != nil || stageDefault.Effort != nil {
			defaults[stage] = stageDefault
		}
	}
	return defaults
}

type GatewayRecoveredReq struct {
	Models        []string `json:"models"`
	ModelDefault  string   `json:"modelDefault"`
	MaxTurnsLimit int      `json:"maxTurnsLimit"`
}

type GatewayModelThinkingReq struct {
	IsThinkingByModel map[string]bool `json:"isThinkingByModel" binding:"required"`
}

type CreateAgentGatewayRes struct {
	AgentGateway
	TrackerToGatewayToken string `json:"trackerToGatewayToken"`
}
