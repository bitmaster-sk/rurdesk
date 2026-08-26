package constants

import "time"

const (
	SessionLifetime         = 24 * time.Hour
	SessionLifetimeExtended = 30 * 24 * time.Hour
)

const SessionIndexPrefix = "sessions:user:"
