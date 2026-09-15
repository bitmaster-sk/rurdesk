package model

import "time"

// LicenseRes is the read view of this instance's licence. Severity is "none"
// when there is nothing to show, and the other fields are then empty.
type LicenseRes struct {
	Severity      string     `json:"severity"`
	DaysRemaining int        `json:"daysRemaining"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	IsDismissible bool       `json:"isDismissible"`
}
