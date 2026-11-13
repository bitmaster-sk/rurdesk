package ssotest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	ClientId     = "rurdesk"
	ClientSecret = "s3cret"
	keyId        = "test-key"
)

type Options struct {
	UserinfoClaims     map[string]any
	IsNonceWrong       bool
	IsSignedByOtherKey bool
}

type grant struct {
	claims      map[string]any
	challenge   string
	redirectUri string
}

type signingKeys struct {
	published *rsa.PrivateKey
	other     *rsa.PrivateKey
}

var loadKeys = sync.OnceValues(func() (signingKeys, error) {
	published, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return signingKeys{}, err
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return signingKeys{}, err
	}
	return signingKeys{published: published, other: other}, nil
})

type Idp struct {
	server  *httptest.Server
	keys    signingKeys
	options Options
	mu      sync.Mutex
	grants  map[string]grant
}

func New(options Options) (*Idp, error) {
	keys, err := loadKeys()
	if err != nil {
		return nil, fmt.Errorf("generating idp keys: %w", err)
	}
	idp := &Idp{keys: keys, options: options, grants: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("GET /jwks", idp.jwks)
	mux.HandleFunc("POST /token", idp.token)
	mux.HandleFunc("GET /userinfo", idp.userinfo)
	idp.server = httptest.NewServer(mux)
	return idp, nil
}

func (idp *Idp) Issuer() string {
	return idp.server.URL
}

func (idp *Idp) Close() {
	idp.server.Close()
}

func (idp *Idp) Authorize(authUrl string, claims map[string]any) (url.Values, error) {
	parsed, err := url.Parse(authUrl)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	if query.Get("client_id") != ClientId {
		return nil, errors.New("unknown client_id")
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		return nil, errors.New("PKCE with S256 is required")
	}
	nonce := query.Get("nonce")
	if idp.options.IsNonceWrong {
		nonce = "wrong-" + nonce
	}
	tokenClaims := map[string]any{"nonce": nonce}
	maps.Copy(tokenClaims, claims)
	code := randomValue()
	idp.mu.Lock()
	idp.grants[code] = grant{claims: tokenClaims, challenge: query.Get("code_challenge"), redirectUri: query.Get("redirect_uri")}
	idp.mu.Unlock()
	return url.Values{"code": {code}, "state": {query.Get("state")}}, nil
}

func (idp *Idp) discovery(w http.ResponseWriter, _ *http.Request) {
	issuer := idp.Issuer()
	document := map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/authorize",
		"token_endpoint":                        issuer + "/token",
		"jwks_uri":                              issuer + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	if idp.options.UserinfoClaims != nil {
		document["userinfo_endpoint"] = issuer + "/userinfo"
	}
	writeJson(w, http.StatusOK, document)
}

func (idp *Idp) jwks(w http.ResponseWriter, _ *http.Request) {
	publicKey := idp.keys.published.PublicKey
	writeJson(w, http.StatusOK, map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"alg": "RS256",
		"use": "sig",
		"kid": keyId,
		"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}}})
}

func (idp *Idp) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJson(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	clientId, clientSecret, hasBasicAuth := r.BasicAuth()
	if !hasBasicAuth {
		clientId, clientSecret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if clientId != ClientId || clientSecret != ClientSecret {
		writeJson(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client"})
		return
	}
	code := r.PostForm.Get("code")
	idp.mu.Lock()
	found, isFound := idp.grants[code]
	delete(idp.grants, code)
	idp.mu.Unlock()
	if !isFound || found.redirectUri != r.PostForm.Get("redirect_uri") || !isPkceValid(found.challenge, r.PostForm.Get("code_verifier")) {
		writeJson(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	idToken, err := idp.sign(found.claims)
	if err != nil {
		writeJson(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	writeJson(w, http.StatusOK, map[string]any{
		"access_token": "access-" + code,
		"token_type":   "Bearer",
		"expires_in":   300,
		"id_token":     idToken,
	})
}

func (idp *Idp) userinfo(w http.ResponseWriter, r *http.Request) {
	if idp.options.UserinfoClaims == nil || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer access-") {
		writeJson(w, http.StatusUnauthorized, map[string]any{"error": "invalid_token"})
		return
	}
	writeJson(w, http.StatusOK, idp.options.UserinfoClaims)
}

func (idp *Idp) sign(claims map[string]any) (string, error) {
	key := idp.keys.published
	if idp.options.IsSignedByOtherKey {
		key = idp.keys.other
	}
	now := time.Now().Unix()
	payload := map[string]any{"iss": idp.Issuer(), "aud": ClientId, "iat": now, "exp": now + 300}
	maps.Copy(payload, claims)
	header, err := encodeSegment(map[string]any{"alg": "RS256", "typ": "JWT", "kid": keyId})
	if err != nil {
		return "", err
	}
	body, err := encodeSegment(payload)
	if err != nil {
		return "", err
	}
	signingInput := header + "." + body
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func isPkceValid(challenge, verifier string) bool {
	digest := sha256.Sum256([]byte(verifier))
	return verifier != "" && base64.RawURLEncoding.EncodeToString(digest[:]) == challenge
}

func encodeSegment(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func randomValue() string {
	buffer := make([]byte, 24)
	_, _ = rand.Read(buffer)
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func writeJson(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
