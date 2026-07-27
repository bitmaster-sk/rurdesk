package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSignatureMissing  = errors.New("missing X-Tracker-Signature header")
	ErrSignatureInvalid  = errors.New("signature verification failed")
	ErrSignatureReplayed = errors.New("request timestamp outside allowed window")
	ErrEventIdInvalid    = errors.New("invalid X-Tracker-Event-Id header")
	ErrSequenceInvalid   = errors.New("invalid X-Tracker-Sequence header")
)

// VerifySignature checks the X-Tracker-Signature header against the body.
// Format: t=<unix_ts>,v1=<hex_hmac>
// HMAC: HMAC-SHA256(secret, "<t>.<body>")
func VerifySignature(secret []byte, signatureHeader string, body []byte) error {
	if signatureHeader == "" {
		return ErrSignatureMissing
	}

	var timestamp int64
	var receivedHex string
	for _, part := range strings.Split(signatureHeader, ",") {
		if strings.HasPrefix(part, "t=") {
			var err error
			timestamp, err = strconv.ParseInt(strings.TrimPrefix(part, "t="), 10, 64)
			if err != nil {
				return ErrSignatureInvalid
			}
		} else if strings.HasPrefix(part, "v1=") {
			receivedHex = strings.TrimPrefix(part, "v1=")
		}
	}

	if timestamp == 0 || receivedHex == "" {
		return ErrSignatureInvalid
	}

	diff := time.Now().Unix() - timestamp
	if diff < -300 || diff > 300 {
		return ErrSignatureReplayed
	}

	payload := fmt.Sprintf("%d.%s", timestamp, body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	receivedBytes, err := hex.DecodeString(receivedHex)
	if err != nil {
		return ErrSignatureInvalid
	}
	expectedBytes, _ := hex.DecodeString(expectedHex)

	if subtle.ConstantTimeCompare(receivedBytes, expectedBytes) != 1 {
		return ErrSignatureInvalid
	}
	return nil
}

// SignPayload computes HMAC-SHA256(secret, "<timestamp>.<body>") and returns
// the full header value: "t=<ts>,v1=<hex>".
func SignPayload(secret []byte, timestamp int64, body []byte) string {
	payload := fmt.Sprintf("%d.%s", timestamp, body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

// ExtractEventId parses the X-Tracker-Event-Id header.
func ExtractEventId(header string) (uuid.UUID, error) {
	if header == "" {
		return uuid.Nil, ErrEventIdInvalid
	}
	id, err := uuid.Parse(header)
	if err != nil {
		return uuid.Nil, ErrEventIdInvalid
	}
	return id, nil
}

// ExtractSequence parses the X-Tracker-Sequence header.
func ExtractSequence(header string) (int64, error) {
	if header == "" {
		return 0, ErrSequenceInvalid
	}
	seq, err := strconv.ParseInt(header, 10, 64)
	if err != nil {
		return 0, ErrSequenceInvalid
	}
	return seq, nil
}
