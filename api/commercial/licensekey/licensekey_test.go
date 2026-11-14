package licensekey

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/spf13/viper"
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

// stampEmbeddedPublicKey stands in for the -ldflags -X stamp of a release build.
func stampEmbeddedPublicKey(t *testing.T, encodedPublicKey string) {
	t.Helper()

	original := embeddedPublicKey
	t.Cleanup(func() { embeddedPublicKey = original })
	embeddedPublicKey = encodedPublicKey
}

func encodePublicKey(publicKey ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(publicKey)
}

func TestFromEnvPrefersTheEmbeddedPublicKey(t *testing.T) {
	viper.AutomaticEnv()

	ourPublicKey, ourSigningKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	envPublicKey, envSigningKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	signedByUs := issueKey(t, ourSigningKey, validClaims(time.Hour))
	signedByEnvPair := issueKey(t, envSigningKey, validClaims(time.Hour))

	const (
		accepted      = "accepted"
		rejected      = "rejected"
		notConfigured = "not configured"
	)

	tests := []struct {
		name            string
		embeddedKey     string
		envPublicKey    string
		licenseKey      string
		expectedOutcome string
	}{
		{"embedded: a key signed by the env pair is rejected", encodePublicKey(ourPublicKey), encodePublicKey(envPublicKey), signedByEnvPair, rejected},
		{"embedded: a key signed by us is accepted", encodePublicKey(ourPublicKey), encodePublicKey(envPublicKey), signedByUs, accepted},
		{"embedded: no env public key is needed", encodePublicKey(ourPublicKey), "", signedByUs, accepted},
		{"embedded: an undecodable embedded key never falls back to env", "not-base64!!!", encodePublicKey(envPublicKey), signedByEnvPair, rejected},
		{"embedded: no license key", encodePublicKey(ourPublicKey), encodePublicKey(envPublicKey), "", notConfigured},
		{"not embedded: the env pair works", "", encodePublicKey(envPublicKey), signedByEnvPair, accepted},
		{"not embedded: a key signed by another pair is rejected", "", encodePublicKey(envPublicKey), signedByUs, rejected},
		{"not embedded: no env public key", "", "", signedByEnvPair, notConfigured},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stampEmbeddedPublicKey(t, test.embeddedKey)
			t.Setenv("LICENSE_PUBLIC_KEY", test.envPublicKey)
			t.Setenv("LICENSE_KEY", test.licenseKey)

			provider, err := FromEnv()

			switch test.expectedOutcome {
			case accepted:
				require.NoError(t, err)
				assert.True(t, provider.IsAvailable(context.Background(), license.SSO))
			case rejected:
				require.Error(t, err)
				assert.NotErrorIs(t, err, ErrNotConfigured)
			case notConfigured:
				assert.ErrorIs(t, err, ErrNotConfigured)
			}
		})
	}
}

func TestIsEnvPublicKeyIgnored(t *testing.T) {
	viper.AutomaticEnv()

	publicKey, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	tests := []struct {
		name            string
		embeddedKey     string
		envPublicKey    string
		expectedIgnored bool
	}{
		{"embedded key and env key", encodePublicKey(publicKey), "env-key", true},
		{"embedded key only", encodePublicKey(publicKey), "", false},
		{"env key only", "", "env-key", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stampEmbeddedPublicKey(t, test.embeddedKey)
			t.Setenv("LICENSE_PUBLIC_KEY", test.envPublicKey)

			assert.Equal(t, test.expectedIgnored, IsEnvPublicKeyIgnored())
		})
	}
}
