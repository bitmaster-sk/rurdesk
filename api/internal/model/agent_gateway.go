package model

import "time"

type AgentGateway struct {
	IdGateway     int64     `json:"idGateway"     db:"id_gateway"`
	IdUserAgent   int64     `json:"idUserAgent"   db:"id_user_agent"`
	GatewayUrl    string    `json:"gatewayUrl"    db:"gateway_url"`
	WebhookSecret []byte    `json:"-"             db:"webhook_secret"`
	CreatedAt     time.Time `json:"createdAt"     db:"created_at"`
}

type CreateAgentGatewayReq struct {
	GatewayUrl string `json:"gatewayUrl" binding:"required,url"`
}

type CreateAgentGatewayRes struct {
	AgentGateway
	TrackerToGatewayToken string `json:"trackerToGatewayToken"`
}
