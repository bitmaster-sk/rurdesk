package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Must match gateway/common/git_token_cipher.go, or no gateway can read the token.
const gitTokenKeyLabel = "rurdesk git token v1"

func EncryptGitToken(webhookSecret []byte, token string) (string, error) {
	gcm, err := gitTokenCipher(webhookSecret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating the git token nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(token), nil)), nil
}

func gitTokenCipher(webhookSecret []byte) (cipher.AEAD, error) {
	if len(webhookSecret) == 0 {
		return nil, fmt.Errorf("the gateway has no webhook secret to encrypt the git token with")
	}
	mac := hmac.New(sha256.New, webhookSecret)
	mac.Write([]byte(gitTokenKeyLabel))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, fmt.Errorf("building the git token cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
