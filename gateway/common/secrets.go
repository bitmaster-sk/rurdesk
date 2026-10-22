package common

import (
	"bytes"
	"encoding/json"
	"sync"
)

// Shorter values are left alone so masking never rewrites ordinary words.
const minRedactedSecretLength = 8

const redactedSecret = "***"

// Counted, because stages of one repo share a token.
type SecretSet struct {
	mu      sync.Mutex
	secrets map[string]int
}

func NewSecretSet() *SecretSet {
	return &SecretSet{secrets: map[string]int{}}
}

func (s *SecretSet) Add(secret string) {
	if len(secret) < minRedactedSecretLength {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[secret]++
}

func (s *SecretSet) Remove(secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secrets[secret] <= 1 {
		delete(s.secrets, secret)
		return
	}
	s.secrets[secret]--
}

func (s *SecretSet) RedactJson(document []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	for secret := range s.secrets {
		encoded, err := json.Marshal(secret)
		if err != nil {
			continue
		}
		document = bytes.ReplaceAll(document, encoded[1:len(encoded)-1], []byte(redactedSecret))
	}
	return document
}
