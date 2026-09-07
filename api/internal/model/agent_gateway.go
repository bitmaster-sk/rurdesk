package model

import "time"

type AgentGateway struct {
	IdGateway     int64     `json:"idGateway"  db:"id_gateway"`
	IdUserAgent   int64     `json:"idUserAgent"     db:"id_user_agent"`
	GatewayUrl    string    `json:"gatewayUrl"    db:"gateway_url"`
	MaxConcurrent int       `json:"maxConcurrent" db:"max_concurrent"`
	WebhookSecret []byte    `json:"-"             db:"webhook_secret"`
	ConfigJson    string    `json:"configJson"    db:"config_json"`
	CreatedAt     time.Time `json:"createdAt"     db:"created_at"`
}

type CreateAgentGatewayReq struct {
	GatewayUrl string `json:"gatewayUrl" binding:"required,url"`
}

// CreateAgentGatewayRes carries the tracker→gateway webhook signing secret,
// shown once and never retrievable again.
type CreateAgentGatewayRes struct {
	AgentGateway
	TrackerToGatewayToken string `json:"trackerToGatewayToken"`
}
