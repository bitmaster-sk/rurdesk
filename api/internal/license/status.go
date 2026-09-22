package license

import "time"

type Severity string

// SeverityNone is not the empty string on purpose, so a missing field never
// reads as "nothing to show". A zero-valued Severity is not None; get one from
// ExpirySeverity.
const (
	SeverityNone    Severity = "none"
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityExpired Severity = "expired"
)

const (
	infoThresholdDays    = 30
	warningThresholdDays = 20
	errorThresholdDays   = 10
)

// Status describes the licence itself, not what it entitles.
type Status struct {
	IsLicensed bool
	Tenant     string
	ExpiresAt  *time.Time
}

// DaysRemaining counts whole days left, rounded down. Zero means it expires
// today.
func (s Status) DaysRemaining(now time.Time) int {
	if s.ExpiresAt == nil {
		return 0
	}
	remaining := s.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

// ExpirySeverity classifies how urgently the licence needs renewing. A free
// instance always returns SeverityNone and must never be warned.
func (s Status) ExpirySeverity(now time.Time) Severity {
	if !s.IsLicensed || s.ExpiresAt == nil {
		return SeverityNone
	}
	if !now.Before(*s.ExpiresAt) {
		return SeverityExpired
	}

	switch days := s.DaysRemaining(now); {
	case days < errorThresholdDays:
		return SeverityError
	case days < warningThresholdDays:
		return SeverityWarning
	case days < infoThresholdDays:
		return SeverityInfo
	default:
		return SeverityNone
	}
}

// IsDismissible reports whether the banner can be hidden: info and warning can,
// error and expired cannot.
func (severity Severity) IsDismissible() bool {
	return severity == SeverityInfo || severity == SeverityWarning
}
