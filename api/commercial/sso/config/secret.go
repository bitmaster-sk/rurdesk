package config

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/spf13/viper"
)

const (
	EnvEncryptionKey = "SSO_ENCRYPTION_KEY"
	keyLength        = 32
)

func LoadKey() ([]byte, error) {
	raw := strings.TrimSpace(viper.GetString(EnvEncryptionKey))
	key, err := base64.StdEncoding.DecodeString(raw)
	if raw == "" || err != nil || len(key) != keyLength {
		return nil, ErrKeyMissing
	}
	return key, nil
}

func sealSecret(key []byte, plaintext string) (ciphertext, nonce []byte, err error) {
	ciphertext, nonce, err = githost.Encrypt(key, []byte(plaintext))
	if err != nil {
		return nil, nil, fmt.Errorf("encrypting client secret: %w", err)
	}
	return ciphertext, nonce, nil
}

func openSecret(key, ciphertext, nonce []byte) (string, error) {
	plaintext, err := githost.Decrypt(key, nonce, ciphertext)
	if err != nil {
		return "", ErrSecretUndecryptable
	}
	return string(plaintext), nil
}
