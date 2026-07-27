package githost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("ghp_test_token_12345")
	ciphertext, nonce, err := Encrypt(key, plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)
	assert.Len(t, nonce, 12)

	decrypted, err := Decrypt(key, nonce, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecrypt_WrongKey_Fails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 0xFF

	ciphertext, nonce, err := Encrypt(key1, []byte("secret"))
	require.NoError(t, err)

	_, err = Decrypt(key2, nonce, ciphertext)
	assert.Error(t, err)
}

func TestDecrypt_TamperedCiphertext_Fails(t *testing.T) {
	key := make([]byte, 32)
	ciphertext, nonce, err := Encrypt(key, []byte("secret"))
	require.NoError(t, err)

	ciphertext[0] ^= 0xFF
	_, err = Decrypt(key, nonce, ciphertext)
	assert.Error(t, err)
}

func TestEncrypt_UniqueNonces(t *testing.T) {
	key := make([]byte, 32)
	_, nonce1, _ := Encrypt(key, []byte("a"))
	_, nonce2, _ := Encrypt(key, []byte("a"))
	assert.NotEqual(t, nonce1, nonce2, "each encryption must use a unique nonce")
}
