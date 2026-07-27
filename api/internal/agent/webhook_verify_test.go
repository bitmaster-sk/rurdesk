package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeSignature(secret []byte, ts int64, body []byte) string {
	payload := fmt.Sprintf("%d.%s", ts, body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifySignature_Valid(t *testing.T) {
	secret := []byte("supersecret")
	body := []byte(`{"event":"assigned"}`)
	ts := time.Now().Unix()
	sig := makeSignature(secret, ts, body)
	assert.NoError(t, VerifySignature(secret, sig, body))
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	secret := []byte("supersecret")
	body := []byte(`{"event":"assigned"}`)
	ts := time.Now().Unix()
	sig := makeSignature(secret, ts, body)
	err := VerifySignature(secret, sig, []byte(`{"event":"tampered"}`))
	assert.ErrorIs(t, err, ErrSignatureInvalid)
}

func TestVerifySignature_OldTimestamp(t *testing.T) {
	secret := []byte("supersecret")
	body := []byte(`{}`)
	ts := time.Now().Unix() - 400
	sig := makeSignature(secret, ts, body)
	err := VerifySignature(secret, sig, body)
	assert.ErrorIs(t, err, ErrSignatureReplayed)
}

func TestVerifySignature_MissingT(t *testing.T) {
	err := VerifySignature([]byte("s"), "v1=deadbeef", []byte(`{}`))
	assert.ErrorIs(t, err, ErrSignatureInvalid)
}

func TestVerifySignature_MissingV1(t *testing.T) {
	ts := time.Now().Unix()
	sig := fmt.Sprintf("t=%d", ts)
	err := VerifySignature([]byte("s"), sig, []byte(`{}`))
	assert.ErrorIs(t, err, ErrSignatureInvalid)
}

func TestVerifySignature_EmptyHeader(t *testing.T) {
	err := VerifySignature([]byte("s"), "", []byte(`{}`))
	assert.ErrorIs(t, err, ErrSignatureMissing)
}
