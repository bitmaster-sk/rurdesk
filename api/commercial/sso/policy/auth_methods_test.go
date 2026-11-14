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

type fakeConfig struct {
	config.IService
	settings config.Settings
	err      error
}

func (f fakeConfig) Settings(context.Context) (config.Settings, error) {
	return f.settings, f.err
}

type fakeLicense struct {
	isAvailable bool
}

var _ license.Provider = fakeLicense{}

func (f fakeLicense) IsAvailable(context.Context, license.Feature) bool { return f.isAvailable }

func (fakeLicense) Status(context.Context) license.Status { return license.Status{} }

type fakeProvisioning struct {
	isProvisioning bool
	err            error
}

func (f fakeProvisioning) IsProvisioning(context.Context) (bool, error) {
	return f.isProvisioning, f.err
}

func TestAuthMethods(t *testing.T) {
	enabled := config.Settings{IsEnabled: true}
	oidc := &model.SsoMethod{Protocol: config.ProtocolOidc}
	tests := []struct {
		name      string
		config    fakeConfig
		isLicense bool
		baseUrl   string
		scim      fakeProvisioning
		expected  model.AuthMethodsRes
	}{
		{name: "disabled", config: fakeConfig{}, isLicense: true, baseUrl: "https://tracker.example.com", expected: model.AuthMethodsRes{Password: true}},
		{name: "ready", config: fakeConfig{settings: enabled}, isLicense: true, baseUrl: "https://tracker.example.com", expected: model.AuthMethodsRes{Password: true, Sso: oidc}},
		{name: "without license", config: fakeConfig{settings: enabled}, baseUrl: "https://tracker.example.com", expected: model.AuthMethodsRes{Password: true, SsoUnavailable: true}},
		{name: "without public url", config: fakeConfig{settings: enabled}, isLicense: true, expected: model.AuthMethodsRes{Password: true, SsoUnavailable: true}},
		{name: "enforced", config: fakeConfig{settings: config.Settings{IsEnabled: true, IsEnforced: true}}, isLicense: true, baseUrl: "https://tracker.example.com", expected: model.AuthMethodsRes{Password: true, Sso: &model.SsoMethod{Protocol: config.ProtocolOidc, IsEnforced: true}}},
		{name: "saml", config: fakeConfig{settings: config.Settings{IsEnabled: true, ActiveProtocol: config.ProtocolSaml}}, isLicense: true, baseUrl: "https://tracker.example.com", expected: model.AuthMethodsRes{Password: true, Sso: &model.SsoMethod{Protocol: config.ProtocolSaml}}},
		{name: "scim allows a long session", config: fakeConfig{settings: enabled}, isLicense: true, baseUrl: "https://tracker.example.com", scim: fakeProvisioning{isProvisioning: true}, expected: model.AuthMethodsRes{Password: true, Sso: &model.SsoMethod{Protocol: config.ProtocolOidc, CanStaySignedIn: true}}},
		{name: "scim state error", config: fakeConfig{settings: enabled}, isLicense: true, baseUrl: "https://tracker.example.com", scim: fakeProvisioning{isProvisioning: true, err: errors.New("db down")}, expected: model.AuthMethodsRes{Password: true, Sso: oidc}},
		{name: "settings error", config: fakeConfig{err: errors.New("db down")}, isLicense: true, baseUrl: "https://tracker.example.com", expected: model.AuthMethodsRes{Password: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(publicurl.EnvBaseUrl, tt.baseUrl)
			t.Cleanup(func() { viper.Set(publicurl.EnvBaseUrl, "") })
			provider := NewAuthMethods(tt.config, tt.scim, func() license.Provider { return fakeLicense{isAvailable: tt.isLicense} })

			assert.Equal(t, tt.expected, provider.Methods(context.Background()))
		})
	}
}
