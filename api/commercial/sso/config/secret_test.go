package config

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadKey(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		isValid bool
	}{
		{name: "32 bytes", value: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), isValid: true},
		{name: "surrounding spaces", value: " " + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)) + " ", isValid: true},
		{name: "empty", value: ""},
		{name: "not base64", value: "%%%"},
		{name: "16 bytes", value: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(EnvEncryptionKey, tt.value)
			t.Cleanup(func() { viper.Set(EnvEncryptionKey, "") })

			key, err := LoadKey()

			if !tt.isValid {
				assert.ErrorIs(t, err, ErrKeyMissing)
				return
			}
			require.NoError(t, err)
			assert.Len(t, key, 32)
		})
	}
}

func TestSecret_RoundTripsAndRejectsAnotherKey(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)

	ciphertext, nonce, err := sealSecret(key, "s3cret")
	require.NoError(t, err)

	plaintext, err := openSecret(key, ciphertext, nonce)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", plaintext)

	_, err = openSecret(bytes.Repeat([]byte{2}, 32), ciphertext, nonce)
	assert.ErrorIs(t, err, ErrSecretUndecryptable)
}
