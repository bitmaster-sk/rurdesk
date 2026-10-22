package agent

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func decryptGitTokenForTest(t *testing.T, webhookSecret []byte, encrypted string) (string, error) {
	t.Helper()
	gcm, err := gitTokenCipher(webhookSecret)
	require.NoError(t, err)
	sealed, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	return string(plain), err
}

func TestEncryptGitToken(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")

	t.Run("only the same webhook secret decrypts it", func(t *testing.T) {
		encrypted, err := EncryptGitToken(secret, "glpat-secret-token")
		require.NoError(t, err)
		require.NotContains(t, encrypted, "glpat-secret-token")

		plain, err := decryptGitTokenForTest(t, secret, encrypted)
		require.NoError(t, err)
		require.Equal(t, "glpat-secret-token", plain)

		_, err = decryptGitTokenForTest(t, []byte("another-secret-another-secret-xx"), encrypted)
		require.Error(t, err)
	})

	t.Run("the same token encrypts differently every time", func(t *testing.T) {
		first, err := EncryptGitToken(secret, "glpat-secret-token")
		require.NoError(t, err)
		second, err := EncryptGitToken(secret, "glpat-secret-token")
		require.NoError(t, err)
		require.NotEqual(t, first, second)
	})

	t.Run("a gateway without a secret is refused", func(t *testing.T) {
		_, err := EncryptGitToken(nil, "glpat-secret-token")
		require.Error(t, err)
	})
}
