package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

var testWebhookSecret = []byte("test-webhook-secret")

func encryptGitTokenForTest(webhookSecret []byte, token string) string {
	mac := hmac.New(sha256.New, webhookSecret)
	mac.Write([]byte(gitTokenKeyLabel))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(token), nil))
}

func TestDecryptGitToken(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	encryptedByTracker := "QGzUMnFYsW2F4YUBewWSickhvB9a31QgSVhHfOjbntboz1Fw4tdJsSq1CqDLYlmgcfjxOg=="

	t.Run("reads a token the tracker encrypted", func(t *testing.T) {
		token, err := DecryptGitToken(secret, encryptedByTracker)
		if err != nil {
			t.Fatalf("DecryptGitToken: %v", err)
		}
		if token != "glpat-cross-module-token" {
			t.Fatalf("token = %q", token)
		}
	})

	t.Run("another webhook secret cannot read it", func(t *testing.T) {
		if _, err := DecryptGitToken([]byte("another-secret-another-secret-xx"), encryptedByTracker); err == nil {
			t.Fatal("expected an error for the wrong secret")
		}
	})

	t.Run("a malformed value is an error", func(t *testing.T) {
		for _, encrypted := range []string{"", "not base64!", "c2hvcnQ="} {
			if _, err := DecryptGitToken(secret, encrypted); err == nil {
				t.Fatalf("expected an error for %q", encrypted)
			}
		}
	})
}
