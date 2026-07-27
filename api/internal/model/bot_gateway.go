package model

import "time"

type BotGateway struct {
	IdBotGateway  int64     `json:"idBotGateway"  db:"id_bot_gateway"`
	IdUserBot     int64     `json:"idUserBot"     db:"id_user_bot"`
	GatewayUrl    string    `json:"gatewayUrl"    db:"gateway_url"`
	MaxConcurrent int       `json:"maxConcurrent" db:"max_concurrent"`
	WebhookSecret []byte    `json:"-"             db:"webhook_secret"`
	ConfigJson    string    `json:"configJson"    db:"config_json"`
	CreatedAt     time.Time `json:"createdAt"     db:"created_at"`
}

type CreateBotGatewayReq struct {
	GatewayUrl string `json:"gatewayUrl" binding:"required,url"`
}

// CreateBotGatewayRes carries the tracker→gateway webhook signing secret,
// shown once and never retrievable again.
type CreateBotGatewayRes struct {
	BotGateway
	TrackerToGatewayToken string `json:"trackerToGatewayToken"`
}
