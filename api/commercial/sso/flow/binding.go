package flow

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

const (
	CookiePrefix = "rurdesk_sso_"
	CookiePath   = "/api/public/sso"
)

type Binding struct {
	Value string
	Hash  string
}

func NewBinding() (Binding, error) {
	value, err := RandomToken()
	if err != nil {
		return Binding{}, err
	}
	return Binding{Value: value, Hash: HashBinding(value)}, nil
}

func HashBinding(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func CookieName(bindingHash string) string {
	return CookiePrefix + bindingHash[:16]
}

func IsBindingValid(cookieValue, bindingHash string) bool {
	return cookieValue != "" && subtle.ConstantTimeCompare([]byte(HashBinding(cookieValue)), []byte(bindingHash)) == 1
}
