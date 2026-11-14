package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signLicense(t *testing.T, signingKey ed25519.PrivateKey) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"tenant":    "bitmaster-sk",
		"expiresAt": time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(ed25519.Sign(signingKey, payload))
}

func TestInstallLicense(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	_, otherSigningKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	tests := []struct {
		name               string
		licenseKey         string
		expectedIsLicensed bool
	}{
		{"a genuine key unlocks paid features", signLicense(t, signingKey), true},
		{"an invalid signature leaves paid features off", signLicense(t, otherSigningKey), false},
		{"no key leaves paid features off", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			injector.SetLicenseProvider(license.Free{})
			t.Setenv("LICENSE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
			t.Setenv("LICENSE_KEY", test.licenseKey)

			installLicense()

			provider := injector.GetLicenseProvider()
			assert.Equal(t, test.expectedIsLicensed, provider.IsAvailable(context.Background(), license.SSO))
			assert.Equal(t, test.expectedIsLicensed, provider.Status(context.Background()).IsLicensed)
		})
	}
}
