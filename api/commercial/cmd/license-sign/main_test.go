package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/licensekey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func encodePrivateKey(t *testing.T, privateKey any) string {
	t.Helper()

	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestRunIssuesAKeyTheEmbeddedPublicKeyAccepts(t *testing.T) {
	publicKey, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	tests := []struct {
		name              string
		expires           string
		expectedExpiresAt time.Time
	}{
		{"a date runs to the end of that day in UTC", "2027-12-31", time.Date(2027, 12, 31, 23, 59, 59, 0, time.UTC)},
		{"an RFC 3339 time is taken as is", "2027-06-30T12:00:00+02:00", time.Date(2027, 6, 30, 10, 0, 0, 0, time.UTC)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"--tenant", "acme", "--expires", test.expires}

			err := run(args, strings.NewReader(encodePrivateKey(t, signingKey)), &stdout, &stderr, now)
			require.NoError(t, err)

			provider, err := licensekey.New(publicKey, strings.TrimSpace(stdout.String()))
			require.NoError(t, err)
			status := provider.Status(context.Background())
			assert.Equal(t, "acme", status.Tenant)
			assert.True(t, test.expectedExpiresAt.Equal(*status.ExpiresAt), "expiresAt %s", status.ExpiresAt)

			// The operator compares this with the key embedded in the release.
			assert.Contains(t, stderr.String(), base64.StdEncoding.EncodeToString(publicKey))
		})
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	_, signingKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	ed25519PEM := encodePrivateKey(t, signingKey)

	tests := []struct {
		name          string
		args          []string
		privateKeyPEM string
	}{
		{"no tenant", []string{"--expires", "2027-12-31"}, ed25519PEM},
		{"no expiry", []string{"--tenant", "acme"}, ed25519PEM},
		{"unparsable expiry", []string{"--tenant", "acme", "--expires", "next year"}, ed25519PEM},
		{"expiry in the past", []string{"--tenant", "acme", "--expires", "2026-11-13"}, ed25519PEM},
		{"unknown flag", []string{"--tenant", "acme", "--expires", "2027-12-31", "--plan", "pro"}, ed25519PEM},
		{"no private key on stdin", []string{"--tenant", "acme", "--expires", "2027-12-31"}, ""},
		{"stdin is not PEM", []string{"--tenant", "acme", "--expires", "2027-12-31"}, "not a key"},
		{"private key is not Ed25519", []string{"--tenant", "acme", "--expires", "2027-12-31"}, encodePrivateKey(t, ecdsaKey)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			err := run(test.args, strings.NewReader(test.privateKeyPEM), &stdout, &stderr, now)

			assert.Error(t, err)
			assert.Empty(t, stdout.String())
		})
	}
}
