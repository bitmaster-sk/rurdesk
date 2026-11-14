package policy

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestPasswordLogin(t *testing.T) {
	enforced := config.Settings{IsEnabled: true, IsEnforced: true}
	regular := &model.User{IsActive: true}
	breakGlass := &model.User{IsActive: true, IsAdmin: true, IsBreakGlass: true}
	tests := []struct {
		name      string
		config    fakeConfig
		isLicense bool
		baseUrl   string
		user      *model.User
		expected  bool
	}{
		{name: "not enforced", config: fakeConfig{settings: config.Settings{IsEnabled: true}}, isLicense: true, baseUrl: "https://tracker.example.com", user: regular, expected: true},
		{name: "enforced blocks a regular account", config: fakeConfig{settings: enforced}, isLicense: true, baseUrl: "https://tracker.example.com", user: regular, expected: false},
		{name: "enforced lets break-glass in", config: fakeConfig{settings: enforced}, isLicense: true, baseUrl: "https://tracker.example.com", user: breakGlass, expected: true},
		{name: "break-glass without a password", config: fakeConfig{settings: enforced}, isLicense: true, baseUrl: "https://tracker.example.com", user: &model.User{IsActive: true, IsAdmin: true, IsBreakGlass: true, IsPasswordless: true}, expected: false},
		{name: "break-glass that lost admin", config: fakeConfig{settings: enforced}, isLicense: true, baseUrl: "https://tracker.example.com", user: &model.User{IsActive: true, IsBreakGlass: true}, expected: false},
		{name: "sso disabled keeps the stored flag inactive", config: fakeConfig{settings: config.Settings{IsEnforced: true}}, isLicense: true, baseUrl: "https://tracker.example.com", user: regular, expected: true},
		{name: "expired license", config: fakeConfig{settings: enforced}, baseUrl: "https://tracker.example.com", user: regular, expected: true},
		{name: "unusable public url", config: fakeConfig{settings: enforced}, isLicense: true, user: regular, expected: true},
		{name: "settings error", config: fakeConfig{err: errors.New("db down")}, isLicense: true, baseUrl: "https://tracker.example.com", user: regular, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(publicurl.EnvBaseUrl, tt.baseUrl)
			t.Cleanup(func() { viper.Set(publicurl.EnvBaseUrl, "") })
			policy := NewPasswordLogin(tt.config, func() license.Provider { return fakeLicense{isAvailable: tt.isLicense} })

			assert.Equal(t, tt.expected, policy.Allow(context.Background(), tt.user))
		})
	}
}
