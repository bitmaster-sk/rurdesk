package model

import (
	"encoding/json"
	"time"
)

type ApiKey struct {
	IdApiKey          int64      `json:"idApiKey"          db:"id_api_key"`
	IdUser            int64      `json:"idUser"            db:"id_user"`
	Name              string     `json:"name"              db:"name"`
	RateLimitOverride *int       `json:"rateLimitOverride" db:"rate_limit_override"`
	CreatedAt         time.Time  `json:"createdAt"         db:"created_at"`
	ExpiresAt         *time.Time `json:"expiresAt"         db:"expires_at"`
	LastUsedAt        *time.Time `json:"lastUsedAt"        db:"last_used_at"`
}

type CreateApiKeyReq struct {
	Name string `json:"name" binding:"required,max=100"`
	// ExpiresAt nil = never expires.
	ExpiresAt *time.Time `json:"expiresAt"`
	// RateLimitOverride nil = default per-user limit; if set, must be >= 1 req/min.
	RateLimitOverride *int `json:"rateLimitOverride" binding:"omitempty,min=1"`
}

// CreateApiKeyRes is returned once on creation only. RawKey is never stored.
type CreateApiKeyRes struct {
	ApiKey
	RawKey string `json:"rawKey"`
}

// ApiKeySession is cached in Redis under auth:apikey:<hash>, storing the full
// User to avoid a lookup per authenticated request. IdUser == 0 is the
// negative-cache sentinel (key not found/expired). ExpiresAt (nil = never)
// caps the cache TTL and is re-checked on every hit.
type ApiKeySession struct {
	User            User       `json:"user"`
	IdApiKey        int64      `json:"idApiKey"`
	RateLimitPerMin int        `json:"rateLimitPerMin"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
}

func (s ApiKeySession) MarshalBinary() ([]byte, error)     { return json.Marshal(s) }
func (s *ApiKeySession) UnmarshalBinary(data []byte) error { return json.Unmarshal(data, s) }
