package licensekey

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/spf13/viper"
)

// ErrNotConfigured means no key was supplied. It is not a failure: the instance
// runs with paid features off.
var ErrNotConfigured = errors.New("no license key configured")

// claims carries no feature list: a valid, unexpired key entitles every paid
// feature.
type claims struct {
	Tenant    string    `json:"tenant"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Provider answers entitlement questions from a signed key. Verification runs
// once at construction and never calls out.
type Provider struct {
	claims claims
}

// New verifies encodedKey against publicKey and returns a Provider carrying its
// claims. The key format is "<base64url(payload)>.<base64url(signature)>".
func New(publicKey ed25519.PublicKey, encodedKey string) (Provider, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return Provider{}, fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	payload, signature, found := strings.Cut(strings.TrimSpace(encodedKey), ".")
	if !found {
		return Provider{}, errors.New("malformed license key: expected payload.signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Provider{}, fmt.Errorf("decoding license payload: %w", err)
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return Provider{}, fmt.Errorf("decoding license signature: %w", err)
	}

	if !ed25519.Verify(publicKey, payloadBytes, signatureBytes) {
		return Provider{}, errors.New("license key signature is not valid")
	}

	var parsed claims
	if err := json.Unmarshal(payloadBytes, &parsed); err != nil {
		return Provider{}, fmt.Errorf("parsing license payload: %w", err)
	}
	if parsed.ExpiresAt.IsZero() {
		return Provider{}, errors.New("license payload has no expiry")
	}
	if parsed.Tenant == "" {
		return Provider{}, errors.New("license payload has no tenant")
	}

	return Provider{claims: parsed}, nil
}

// embeddedPublicKey is our release public key (base64 of the raw 32-byte
// Ed25519 key), stamped at link time by script/build.sh. Dev and e2e builds
// leave it empty and read LICENSE_PUBLIC_KEY from env instead.
var embeddedPublicKey string

// FromEnv builds a Provider from LICENSE_KEY. A release build verifies it
// against its embedded public key and ignores LICENSE_PUBLIC_KEY, so a
// self-made key pair cannot unlock it. Only an unstamped build reads
// LICENSE_PUBLIC_KEY (base64 of the raw 32-byte Ed25519 key).
func FromEnv() (Provider, error) {
	encodedKey := viper.GetString("LICENSE_KEY")
	encodedPublicKey, origin := publicKeyInUse()
	if encodedKey == "" || encodedPublicKey == "" {
		return Provider{}, ErrNotConfigured
	}

	publicKey, err := base64.StdEncoding.DecodeString(encodedPublicKey)
	if err != nil {
		return Provider{}, fmt.Errorf("decoding %s: %w", origin, err)
	}
	return New(publicKey, encodedKey)
}

// IsEnvPublicKeyIgnored reports a LICENSE_PUBLIC_KEY that this build overrides
// with its embedded key.
func IsEnvPublicKeyIgnored() bool {
	return embeddedPublicKey != "" && viper.GetString("LICENSE_PUBLIC_KEY") != ""
}

// publicKeyInUse never falls back to env when a key is embedded, not even when
// the embedded one is broken.
func publicKeyInUse() (encodedPublicKey string, origin string) {
	if embeddedPublicKey != "" {
		return embeddedPublicKey, "embedded public key"
	}
	return viper.GetString("LICENSE_PUBLIC_KEY"), "LICENSE_PUBLIC_KEY"
}

// IsAvailable entitles every paid feature while the key is unexpired.
func (p Provider) IsAvailable(_ context.Context, _ license.Feature) bool {
	return !p.isExpired(time.Now())
}

// Status reports the licence even when it has expired, so the expired banner
// keeps showing.
func (p Provider) Status(_ context.Context) license.Status {
	expiresAt := p.claims.ExpiresAt
	return license.Status{
		IsLicensed: true,
		Tenant:     p.claims.Tenant,
		ExpiresAt:  &expiresAt,
	}
}

func (p Provider) isExpired(now time.Time) bool {
	return !now.Before(p.claims.ExpiresAt)
}
