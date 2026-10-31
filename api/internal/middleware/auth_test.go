package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSessionCache struct {
	value string
	err   error
}

func (f *fakeSessionCache) Get(_ context.Context, _ string) *redis.StringCmd {
	return redis.NewStringResult(f.value, f.err)
}

type fakeApiKeyAuth struct {
	session      *model.ApiKeySession
	lookupErr    error
	rateLimitErr error
	lookupCalled bool
}

func (f *fakeApiKeyAuth) LookupSession(_ context.Context, _ string) (*model.ApiKeySession, error) {
	f.lookupCalled = true
	return f.session, f.lookupErr
}

func (f *fakeApiKeyAuth) CheckRateLimit(_ context.Context, _ int64, _ int) error {
	return f.rateLimitErr
}

func performAuthRequest(t *testing.T, cache SessionCache, apiKeyAuth ApiKeyAuthenticator, token string) *httptest.ResponseRecorder {
	t.Helper()
	if token == "" {
		return performAuthHeaderRequest(t, cache, apiKeyAuth, "")
	}
	return performAuthHeaderRequest(t, cache, apiKeyAuth, "Bearer "+token)
}

func performAuthHeaderRequest(t *testing.T, cache SessionCache, apiKeyAuth ApiKeyAuthenticator, header string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", Auth(cache, apiKeyAuth), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func sessionJSON(t *testing.T) string {
	t.Helper()
	payload, err := json.Marshal(model.User{IdUser: 1, Name: "John Snow", IsActive: true})
	require.NoError(t, err)
	return string(payload)
}

const rawApiKeyBearer = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestAuthMissingBearerIsUnauthorized(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, apiKeyAuth.lookupCalled)
}

func TestAuthValidSessionPasses(t *testing.T) {
	cache := &fakeSessionCache{value: sessionJSON(t)}
	recorder := performAuthRequest(t, cache, &fakeApiKeyAuth{}, "some-session-token")

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// A session snapshot predating the is_active column deserializes with
// IsActive=false; it must still authenticate — otherwise deploying the column
// would log every existing user out. Disabling a user is enforced by session
// invalidation (and Login refusing disabled accounts), not by this middleware.
func TestAuthLegacySessionSnapshotWithoutIsActivePasses(t *testing.T) {
	payload, err := json.Marshal(model.User{IdUser: 1, Name: "John Snow", IsActive: false})
	require.NoError(t, err)
	cache := &fakeSessionCache{value: string(payload)}
	recorder := performAuthRequest(t, cache, &fakeApiKeyAuth{}, "some-session-token")

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAuthUnknownTokenIsUnauthorized(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, "expired-session-token")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.True(t, apiKeyAuth.lookupCalled)
}

func TestAuthBareSessionTokenIsUnauthorized(t *testing.T) {
	cache := &fakeSessionCache{value: sessionJSON(t)}
	recorder := performAuthHeaderRequest(t, cache, &fakeApiKeyAuth{}, "some-session-token")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuthBareApiKeyIsUnauthorized(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{session: &model.ApiKeySession{IdApiKey: 7, User: model.User{IdUser: 2, IsActive: true}}}
	recorder := performAuthHeaderRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, apiKeyAuth.lookupCalled)
}

func TestAuthBearerSchemeIsCaseInsensitive(t *testing.T) {
	cache := &fakeSessionCache{value: sessionJSON(t)}
	recorder := performAuthHeaderRequest(t, cache, &fakeApiKeyAuth{}, "bearer some-session-token")

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAuthBearerWithoutTokenIsUnauthorized(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{}
	recorder := performAuthHeaderRequest(t, &fakeSessionCache{value: sessionJSON(t)}, apiKeyAuth, "Bearer ")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, apiKeyAuth.lookupCalled)
}

func TestAuthCacheInfraErrorIsServiceUnavailable(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{}
	cache := &fakeSessionCache{err: errors.New("dial tcp: connection refused")}
	recorder := performAuthRequest(t, cache, apiKeyAuth, "some-session-token")

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.False(t, apiKeyAuth.lookupCalled, "infra error must not fall through to the api-key path")
}

func TestAuthApiKeyLookupInfraErrorIsServiceUnavailable(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{lookupErr: errors.New("looking up api key: db down")}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestAuthUnknownApiKeyIsUnauthorized(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuthDisabledApiKeyUserIsUnauthorized(t *testing.T) {
	// Mirrors a snapshot cached before the user was disabled: the session user
	// carries isActive=false and must be rejected like an unknown key.
	apiKeyAuth := &fakeApiKeyAuth{session: &model.ApiKeySession{
		IdApiKey: 7,
		User:     model.User{IdUser: 2, Name: "Agent", IsActive: false},
	}}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuthValidApiKeyPasses(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{session: &model.ApiKeySession{
		IdApiKey: 7,
		User:     model.User{IdUser: 2, Name: "Agent", IsActive: true},
	}}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAuthRateLimitedApiKeyIsTooManyRequests(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{
		session:      &model.ApiKeySession{IdApiKey: 7, User: model.User{IdUser: 2, IsActive: true}},
		rateLimitErr: errors.New("rate limited"),
	}
	recorder := performAuthRequest(t, &fakeSessionCache{err: redis.Nil}, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
}

// The api-key fast path skips the session-cache lookup entirely, so a cache
// outage must not block api-key authentication.
func TestAuthApiKeyBearerSkipsSessionCache(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{session: &model.ApiKeySession{
		IdApiKey: 7,
		User:     model.User{IdUser: 2, IsActive: true},
	}}
	cache := &fakeSessionCache{err: errors.New("dial tcp: connection refused")}
	recorder := performAuthRequest(t, cache, apiKeyAuth, rawApiKeyBearer)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// performSubprotocolRequest issues a request authenticated the way the browser
// authenticates a WebSocket handshake: the token rides in Sec-WebSocket-Protocol,
// because the WebSocket API cannot set an Authorization header.
func performSubprotocolRequest(t *testing.T, cache SessionCache, protocols string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", Auth(cache, &fakeApiKeyAuth{}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Sec-WebSocket-Protocol", protocols)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestAuthAcceptsTokenFromWebsocketSubprotocol(t *testing.T) {
	cache := &fakeSessionCache{value: sessionJSON(t)}
	recorder := performSubprotocolRequest(t, cache, "Authorization, some-session-token")

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAuthUnknownSubprotocolTokenIsUnauthorized(t *testing.T) {
	recorder := performSubprotocolRequest(t, &fakeSessionCache{err: redis.Nil}, "Authorization, expired-session-token")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

// Only the "Authorization" marker carries a token. A handshake offering some
// other subprotocol must not be treated as authenticated.
func TestAuthIgnoresUnrelatedSubprotocol(t *testing.T) {
	cache := &fakeSessionCache{value: sessionJSON(t)}
	recorder := performSubprotocolRequest(t, cache, "graphql-ws")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuthMarkerWithoutTokenIsUnauthorized(t *testing.T) {
	cache := &fakeSessionCache{value: sessionJSON(t)}
	recorder := performSubprotocolRequest(t, cache, "Authorization")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

// The Authorization cookie must not authenticate: it is a second copy of the
// session token with different scoping (cookies ignore the port) and a
// different lifetime (session cookie vs localStorage), which would silently
// break the WebSocket while REST kept working.
func TestAuthCookieIsNotAnAuthSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", Auth(&fakeSessionCache{value: sessionJSON(t)}, &fakeApiKeyAuth{}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "Authorization", Value: strings.Repeat("a", 36)})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
