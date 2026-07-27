package main

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadKey_FromEnv covers the env-var path of readKey: a valid 32-byte base64
// value decodes, and wrong-length / non-base64 values are rejected with a clear
// error (the masked-prompt path needs a TTY and is exercised manually).
func TestReadKey_FromEnv(t *testing.T) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	valid := base64.StdEncoding.EncodeToString(raw)

	t.Run("valid 32-byte key", func(t *testing.T) {
		t.Setenv("TEST_KEY", valid)
		key, err := readKey("TEST_KEY", "")
		require.NoError(t, err)
		assert.Equal(t, raw, key)
	})

	t.Run("wrong length rejected", func(t *testing.T) {
		t.Setenv("TEST_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))
		_, err := readKey("TEST_KEY", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "32 bytes")
	})

	t.Run("invalid base64 rejected", func(t *testing.T) {
		t.Setenv("TEST_KEY", "not valid base64 !!!")
		_, err := readKey("TEST_KEY", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decoding")
	})
}
