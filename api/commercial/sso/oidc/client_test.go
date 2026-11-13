package oidc

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const testRedirectUrl = "https://tracker.example.com/api/public/sso/oidc/callback"

func newIdp(t *testing.T, options ssotest.Options) *ssotest.Idp {
	idp, err := ssotest.New(options)
	require.NoError(t, err)
	t.Cleanup(idp.Close)
	return idp
}

func connectionFor(idp *ssotest.Idp) config.Connection {
	return config.Connection{
		Protocol:     config.ProtocolOidc,
		Issuer:       idp.Issuer(),
		ClientId:     ssotest.ClientId,
		ClientSecret: ssotest.ClientSecret,
		EmailClaim:   config.DefaultEmailClaim,
		GroupsClaim:  config.DefaultGroupsClaim,
	}
}

func signIn(t *testing.T, idp *ssotest.Idp, connection config.Connection, claims map[string]any) (identity.Identity, error) {
	client, err := NewClients(&http.Client{Timeout: 5 * time.Second}).Client(context.Background(), connection, testRedirectUrl)
	require.NoError(t, err)
	verifier := oauth2.GenerateVerifier()
	values, err := idp.Authorize(client.AuthUrl("state-1", "nonce-1", verifier), claims)
	require.NoError(t, err)
	return client.Identity(context.Background(), values.Get("code"), verifier, "nonce-1")
}

func TestClient_AuthUrlUsesPkceNonceAndScopes(t *testing.T) {
	idp := newIdp(t, ssotest.Options{})
	connection := connectionFor(idp)
	connection.HasGroupsScope = true

	client, err := NewClients(http.DefaultClient).Client(context.Background(), connection, testRedirectUrl)
	require.NoError(t, err)
	parsed, err := url.Parse(client.AuthUrl("state-1", "nonce-1", oauth2.GenerateVerifier()))
	require.NoError(t, err)

	query := parsed.Query()
	assert.Equal(t, idp.Issuer()+"/authorize", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	assert.Equal(t, "state-1", query.Get("state"))
	assert.Equal(t, "nonce-1", query.Get("nonce"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, "openid email profile groups", query.Get("scope"))
	assert.Equal(t, testRedirectUrl, query.Get("redirect_uri"))
}

func TestClient_IdentityFromAVerifiedToken(t *testing.T) {
	idp := newIdp(t, ssotest.Options{})

	id, err := signIn(t, idp, connectionFor(idp), map[string]any{"sub": "sub-1", "email": "Jana@Acme.test", "name": "Jana", "groups": []any{"dev"}})

	require.NoError(t, err)
	assert.Equal(t, idp.Issuer(), id.Issuer)
	assert.Equal(t, "sub-1", id.Subject)
	assert.Equal(t, "jana@acme.test", id.Email)
	assert.Equal(t, []string{"dev"}, id.Groups)
}

func TestClient_RejectsBadTokens(t *testing.T) {
	tests := []struct {
		name    string
		options ssotest.Options
		claims  map[string]any
		err     error
	}{
		{name: "wrong nonce", options: ssotest.Options{IsNonceWrong: true}, claims: map[string]any{"sub": "sub-1"}, err: ErrNonceMismatch},
		{name: "foreign signature", options: ssotest.Options{IsSignedByOtherKey: true}, claims: map[string]any{"sub": "sub-1"}},
		{name: "missing subject", claims: map[string]any{"email": "jana@acme.test"}, err: ErrSubjectMissing},
		{name: "userinfo of another subject", options: ssotest.Options{UserinfoClaims: map[string]any{"sub": "sub-2", "email": "jana@acme.test"}}, claims: map[string]any{"sub": "sub-1"}, err: ErrUserinfoSubject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idp := newIdp(t, tt.options)

			_, err := signIn(t, idp, connectionFor(idp), tt.claims)

			require.Error(t, err)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestClient_RejectsAWrongPkceVerifier(t *testing.T) {
	idp := newIdp(t, ssotest.Options{})
	client, err := NewClients(http.DefaultClient).Client(context.Background(), connectionFor(idp), testRedirectUrl)
	require.NoError(t, err)
	values, err := idp.Authorize(client.AuthUrl("state-1", "nonce-1", oauth2.GenerateVerifier()), map[string]any{"sub": "sub-1"})
	require.NoError(t, err)

	_, err = client.Identity(context.Background(), values.Get("code"), oauth2.GenerateVerifier(), "nonce-1")

	assert.Error(t, err)
}

func TestClient_FillsMissingDataFromUserinfo(t *testing.T) {
	idp := newIdp(t, ssotest.Options{UserinfoClaims: map[string]any{"sub": "sub-1", "email": "jana@acme.test", "email_verified": true}})

	id, err := signIn(t, idp, connectionFor(idp), map[string]any{"sub": "sub-1", "name": "Jana"})

	require.NoError(t, err)
	assert.Equal(t, "jana@acme.test", id.Email)
	assert.Equal(t, "Jana", id.Name)
}

func TestClient_WithoutUserinfoKeepsMissingData(t *testing.T) {
	idp := newIdp(t, ssotest.Options{})

	id, err := signIn(t, idp, connectionFor(idp), map[string]any{"sub": "sub-1"})

	require.NoError(t, err)
	assert.Empty(t, id.Email)
}
