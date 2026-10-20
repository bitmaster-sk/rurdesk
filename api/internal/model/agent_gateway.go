package model

import (
	"slices"
	"time"
)

type AgentGateway struct {
	IdGateway         int64      `json:"idGateway"         db:"id_gateway"`
	IdUserAgent       int64      `json:"idUserAgent"       db:"id_user_agent"`
	GatewayUrl        string     `json:"gatewayUrl"        db:"gateway_url"`
	WebhookSecret     []byte     `json:"-"                 db:"webhook_secret"`
	Models            []string   `json:"models"            db:"models"`
	ModelDefault      *string    `json:"modelDefault"      db:"model_default"`
	ModelsReportedAt  *time.Time `json:"modelsReportedAt"  db:"models_reported_at"`
	MaxTurnsLimit     *int       `json:"maxTurnsLimit"     db:"max_turns_limit"`
	MaxTurnsPlan      *int       `json:"maxTurnsPlan"      db:"max_turns_plan"`
	MaxTurnsImplement *int       `json:"maxTurnsImplement" db:"max_turns_implement"`
	CreatedAt         time.Time  `json:"createdAt"         db:"created_at"`
}

func (gw *AgentGateway) IsSupportedModel(model string) bool {
	return slices.Contains(gw.Models, model)
}

// The max=10000 tags must stay equal to constants.MaxAgentTurns.
type CreateAgentGatewayReq struct {
	GatewayUrl        string `json:"gatewayUrl"        binding:"required,url"`
	MaxTurnsPlan      *int   `json:"maxTurnsPlan"      binding:"omitempty,min=1,max=10000"`
	MaxTurnsImplement *int   `json:"maxTurnsImplement" binding:"omitempty,min=1,max=10000"`
}

type GatewayRecoveredReq struct {
	Models        []string `json:"models"`
	ModelDefault  string   `json:"modelDefault"`
	MaxTurnsLimit int      `json:"maxTurnsLimit"`
}

type CreateAgentGatewayRes struct {
	AgentGateway
	TrackerToGatewayToken string `json:"trackerToGatewayToken"`
}
