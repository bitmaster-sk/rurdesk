package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// Must match api/internal/agent/git_token_cipher.go, or the gateway cannot read the token.
const gitTokenKeyLabel = "rurdesk git token v1"

func DecryptGitToken(webhookSecret []byte, encrypted string) (string, error) {
	mac := hmac.New(sha256.New, webhookSecret)
	mac.Write([]byte(gitTokenKeyLabel))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return "", fmt.Errorf("building the git token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("building the git token cipher: %w", err)
	}
	sealed, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("decoding the git token: %w", err)
	}
	if len(sealed) <= gcm.NonceSize() {
		return "", errors.New("the git token is too short")
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypting the git token: %w", err)
	}
	return string(plain), nil
}
