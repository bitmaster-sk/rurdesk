package licensekey

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func issueKey(t *testing.T, signingKey ed25519.PrivateKey, issued claims) string {
	t.Helper()

	payload, err := json.Marshal(issued)
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(ed25519.Sign(signingKey, payload))
}

func validClaims(expiresIn time.Duration) claims {
	return claims{
		Tenant:    "bitmaster-sk",
		ExpiresAt: time.Now().Add(expiresIn),
	}
}

func TestNewAcceptsAGenuineKey(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	provider, err := New(publicKey, issueKey(t, signingKey, validClaims(30*24*time.Hour)))
	require.NoError(t, err)

	status := provider.Status(context.Background())
	assert.True(t, status.IsLicensed)
	assert.Equal(t, "bitmaster-sk", status.Tenant)
	assert.True(t, provider.IsAvailable(context.Background(), license.SSO))
}

func TestNewRejectsBadKeys(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	_, otherSigningKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	genuine := issueKey(t, signingKey, validClaims(time.Hour))

	noExpiry := validClaims(time.Hour)
	noExpiry.ExpiresAt = time.Time{}

	noTenant := validClaims(time.Hour)
	noTenant.Tenant = ""

	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"no separator", "not-a-key"},
		{"payload is not base64", "!!!.abc"},
		{"signature is not base64", base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + ".!!!"},
		{"signed by someone else", issueKey(t, otherSigningKey, validClaims(time.Hour))},
		{"payload tampered after signing", "x" + genuine},
		{"payload is not json", base64.RawURLEncoding.EncodeToString([]byte("nope")) + "." +
			base64.RawURLEncoding.EncodeToString(ed25519.Sign(signingKey, []byte("nope")))},
		{"no expiry in payload", issueKey(t, signingKey, noExpiry)},
		{"no tenant in payload", issueKey(t, signingKey, noTenant)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(publicKey, test.key)
			assert.Error(t, err)
		})
	}
}

func TestNewRejectsAnUnusablePublicKey(t *testing.T) {
	_, err := New(ed25519.PublicKey("too short"), "a.b")
	assert.Error(t, err)
}

func TestExpiredKeyEntitlesNothingButStillReports(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	provider, err := New(publicKey, issueKey(t, signingKey, validClaims(-time.Hour)))
	require.NoError(t, err)

	assert.False(t, provider.IsAvailable(context.Background(), license.SSO))

	status := provider.Status(context.Background())
	assert.True(t, status.IsLicensed)
	assert.Equal(t, license.SeverityExpired, status.ExpirySeverity(time.Now()))
}

func TestAnUnexpiredKeyEntitlesEveryPaidFeature(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	provider, err := New(publicKey, issueKey(t, signingKey, validClaims(time.Hour)))
	require.NoError(t, err)

	assert.True(t, provider.IsAvailable(context.Background(), license.SSO))
	assert.True(t, provider.IsAvailable(context.Background(), license.Feature("not-invented-yet")))
}

func TestUnknownPayloadFieldsAreIgnored(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	payload, err := json.Marshal(map[string]any{
		"tenant":    "bitmaster-sk",
		"expiresAt": time.Now().Add(time.Hour).Format(time.RFC3339),
		"plan":      "enterprise",
		"features":  []string{"sso"},
	})
	require.NoError(t, err)

	key := base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(ed25519.Sign(signingKey, payload))

	provider, err := New(publicKey, key)
	require.NoError(t, err)
	assert.Equal(t, "bitmaster-sk", provider.Status(context.Background()).Tenant)
}
