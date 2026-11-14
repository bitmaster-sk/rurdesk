package oidc

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	ErrNoIdToken       = errors.New("token response has no id_token")
	ErrNonceMismatch   = errors.New("id token nonce does not match")
	ErrSubjectMissing  = errors.New("id token has no subject")
	ErrUserinfoSubject = errors.New("userinfo subject does not match the id token")
)

const providerTtl = 10 * time.Minute

type IClients interface {
	Client(ctx context.Context, connection config.Connection, redirectUrl string) (*Client, error)
}

type cachedProvider struct {
	provider  *gooidc.Provider
	fetchedAt time.Time
}

type Clients struct {
	httpClient *http.Client
	mu         sync.Mutex
	providers  map[string]cachedProvider
}

var _ IClients = (*Clients)(nil)

func NewClients(httpClient *http.Client) *Clients {
	return &Clients{httpClient: httpClient, providers: map[string]cachedProvider{}}
}

func (c *Clients) Client(ctx context.Context, connection config.Connection, redirectUrl string) (*Client, error) {
	provider, err := c.provider(ctx, connection.Issuer)
	if err != nil {
		return nil, err
	}
	scopes := []string{gooidc.ScopeOpenID, "email", "profile"}
	if connection.HasGroupsScope {
		scopes = append(scopes, "groups")
	}
	return &Client{
		httpClient: c.httpClient,
		provider:   provider,
		connection: connection,
		oauth: oauth2.Config{
			ClientID:     connection.ClientId,
			ClientSecret: connection.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectUrl,
			Scopes:       scopes,
		},
		verifier: provider.Verifier(&gooidc.Config{ClientID: connection.ClientId}),
	}, nil
}

func (c *Clients) provider(ctx context.Context, issuer string) (*gooidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, isCached := c.providers[issuer]; isCached && time.Since(cached.fetchedAt) < providerTtl {
		return cached.provider, nil
	}
	provider, err := gooidc.NewProvider(gooidc.ClientContext(ctx, c.httpClient), issuer)
	if err != nil {
		return nil, fmt.Errorf("discovering %s: %w", issuer, err)
	}
	c.providers[issuer] = cachedProvider{provider: provider, fetchedAt: time.Now()}
	return provider, nil
}

type Client struct {
	httpClient *http.Client
	provider   *gooidc.Provider
	connection config.Connection
	oauth      oauth2.Config
	verifier   *gooidc.IDTokenVerifier
}

func (c *Client) AuthUrl(state, nonce, verifier string) string {
	return c.oauth.AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

func (c *Client) Identity(ctx context.Context, code, verifier, nonce string) (identity.Identity, error) {
	ctx = gooidc.ClientContext(ctx, c.httpClient)
	token, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return identity.Identity{}, fmt.Errorf("exchanging code: %w", err)
	}
	rawIdToken, _ := token.Extra("id_token").(string)
	if rawIdToken == "" {
		return identity.Identity{}, ErrNoIdToken
	}
	idToken, err := c.verifier.Verify(ctx, rawIdToken)
	if err != nil {
		return identity.Identity{}, fmt.Errorf("verifying id token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return identity.Identity{}, ErrNonceMismatch
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return identity.Identity{}, fmt.Errorf("reading id token claims: %w", err)
	}
	id := MapClaims(claims, c.connection.EmailClaim, c.connection.GroupsClaim)
	if id.Subject == "" {
		return identity.Identity{}, ErrSubjectMissing
	}
	if !needsUserinfo(id) || c.provider.UserInfoEndpoint() == "" {
		return id, nil
	}
	return c.fillFromUserinfo(ctx, id, token)
}

func (c *Client) fillFromUserinfo(ctx context.Context, id identity.Identity, token *oauth2.Token) (identity.Identity, error) {
	info, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return identity.Identity{}, fmt.Errorf("loading userinfo: %w", err)
	}
	if info.Subject != id.Subject {
		return identity.Identity{}, ErrUserinfoSubject
	}
	claims := map[string]any{}
	if err := info.Claims(&claims); err != nil {
		return identity.Identity{}, fmt.Errorf("reading userinfo claims: %w", err)
	}
	return FillFromUserinfo(id, claims, c.connection.EmailClaim, c.connection.GroupsClaim), nil
}
