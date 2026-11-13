package scim

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const tokenPrefix = "rdscim_"

func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating scim token: %w", err)
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isTokenValid(token string, hash *string) bool {
	if token == "" || hash == nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(*hash)) == 1
}
