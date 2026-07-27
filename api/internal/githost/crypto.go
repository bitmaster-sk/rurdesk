package githost

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	encryptionKey     []byte
	encryptionKeyOnce sync.Once
	encryptionKeyErr  error
)

// LoadEncryptionKey reads GIT_INTEGRATION_ENCRYPTION_KEY from env once.
// Key must be 32 bytes base64-encoded.
func LoadEncryptionKey() ([]byte, error) {
	encryptionKeyOnce.Do(func() {
		raw := os.Getenv("GIT_INTEGRATION_ENCRYPTION_KEY")
		if raw == "" {
			encryptionKeyErr = fmt.Errorf("GIT_INTEGRATION_ENCRYPTION_KEY is not set")
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			encryptionKeyErr = fmt.Errorf("decoding encryption key: %w", err)
			return
		}
		if len(decoded) != 32 {
			encryptionKeyErr = fmt.Errorf("encryption key must be 32 bytes, got %d", len(decoded))
			return
		}
		encryptionKey = decoded
	})
	return encryptionKey, encryptionKeyErr
}

// ResetEncryptionKey clears the cached key — for testing only.
func ResetEncryptionKey() {
	encryptionKeyOnce = sync.Once{}
	encryptionKey = nil
	encryptionKeyErr = nil
}

// Encrypt encrypts plaintext with AES-256-GCM. Returns (ciphertext, nonce, error).
func Encrypt(key []byte, plaintext []byte) (ciphertext []byte, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("creating GCM: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts AES-256-GCM ciphertext with the given key and nonce.
func Decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	return plaintext, nil
}
