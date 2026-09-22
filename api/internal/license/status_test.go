package license

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func licensedUntil(now time.Time, days float64) Status {
	expiresAt := now.Add(time.Duration(days * float64(24*time.Hour)))
	return Status{IsLicensed: true, ExpiresAt: &expiresAt}
}

func TestExpirySeverity(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		status   Status
		expected Severity
	}{
		{"free instance is never warned", Status{}, SeverityNone},
		{"licensed without expiry is never warned", Status{IsLicensed: true}, SeverityNone},
		{"far from expiry", licensedUntil(now, 90), SeverityNone},
		{"exactly at the info threshold", licensedUntil(now, 30), SeverityNone},
		{"just inside the info threshold", licensedUntil(now, 29), SeverityInfo},
		{"exactly at the warning threshold", licensedUntil(now, 20), SeverityInfo},
		{"just inside the warning threshold", licensedUntil(now, 19), SeverityWarning},
		{"exactly at the error threshold", licensedUntil(now, 10), SeverityWarning},
		{"just inside the error threshold", licensedUntil(now, 9), SeverityError},
		{"expiring later today", licensedUntil(now, 0.5), SeverityError},
		{"expired moments ago", licensedUntil(now, -0.001), SeverityExpired},
		{"long expired", licensedUntil(now, -400), SeverityExpired},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.status.ExpirySeverity(now))
		})
	}
}

func TestDaysRemainingRoundsDown(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		status   Status
		expected int
	}{
		{"no expiry", Status{IsLicensed: true}, 0},
		{"whole days", licensedUntil(now, 14), 14},
		{"partial day rounds down", licensedUntil(now, 9.9), 9},
		{"expires today", licensedUntil(now, 0.4), 0},
		{"already expired", licensedUntil(now, -3), 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.status.DaysRemaining(now))
		})
	}
}

func TestIsDismissible(t *testing.T) {
	assert.True(t, SeverityInfo.IsDismissible())
	assert.True(t, SeverityWarning.IsDismissible())
	assert.False(t, SeverityError.IsDismissible())
	assert.False(t, SeverityExpired.IsDismissible())
	assert.False(t, SeverityNone.IsDismissible())
}

func TestFreeProviderEntitlesNothing(t *testing.T) {
	provider := Free{}

	assert.False(t, provider.IsAvailable(context.Background(), SSO))
	assert.Equal(t, Status{}, provider.Status(context.Background()))
	assert.Equal(t, SeverityNone, provider.Status(context.Background()).ExpirySeverity(time.Now()))
}
