package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

type fakeRunTokenAuth struct {
	user         *model.User
	scope        *extctx.RunScope
	lookupErr    error
	lookupCalled bool
}

func (f *fakeRunTokenAuth) LookupRunToken(_ context.Context, _ string) (*model.User, *extctx.RunScope, error) {
	f.lookupCalled = true
	return f.user, f.scope, f.lookupErr
}

const chatBearer = "rdr_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func validRunTokenAuth() *fakeRunTokenAuth {
	return &fakeRunTokenAuth{
		user:  &model.User{IdUser: 5, IsAgent: true, IsActive: true},
		scope: &extctx.RunScope{IdRun: 9, IdsProject: []int64{1}, MaxRole: model.RoleViewer},
	}
}

// performChatRequest serves one request on a router with the real route pattern, so the
// middleware sees the production FullPath.
func performChatRequest(t *testing.T, runTokenAuth RunTokenAuthenticator, apiKeyAuth *fakeApiKeyAuth, method, route, target string) (*httptest.ResponseRecorder, *extctx.RunScope) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var seen *extctx.RunScope
	router.Handle(method, route, Auth(&fakeSessionCache{err: errors.New("session cache must not be read")}, apiKeyAuth, runTokenAuth), func(c *gin.Context) {
		if scope, ok := extctx.GetRunScope(c.Request.Context()); ok {
			seen = &scope
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+chatBearer)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder, seen
}

func TestAuthRunTokenCarriesScopeAndSkipsOtherLookups(t *testing.T) {
	apiKeyAuth := &fakeApiKeyAuth{}
	recorder, scope := performChatRequest(t, validRunTokenAuth(), apiKeyAuth,
		http.MethodGet, "/api/private/project/:idProject/issue", "/api/private/project/1/issue")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, &extctx.RunScope{IdRun: 9, IdsProject: []int64{1}, MaxRole: model.RoleViewer}, scope)
	assert.False(t, apiKeyAuth.lookupCalled)
}

func TestAuthUnknownRunTokenIsUnauthorized(t *testing.T) {
	recorder, _ := performChatRequest(t, &fakeRunTokenAuth{}, &fakeApiKeyAuth{},
		http.MethodGet, "/api/private/project", "/api/private/project")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuthChatLookupInfraErrorIsServiceUnavailable(t *testing.T) {
	recorder, _ := performChatRequest(t, &fakeRunTokenAuth{lookupErr: errors.New("db down")}, &fakeApiKeyAuth{},
		http.MethodGet, "/api/private/project", "/api/private/project")

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestAuthRunTokenRoutePolicy(t *testing.T) {
	cases := []struct {
		method string
		route  string
		target string
		want   int
	}{
		{http.MethodGet, "/api/private/project", "/api/private/project", http.StatusOK},
		{http.MethodGet, "/api/private/project/:idProject", "/api/private/project/1", http.StatusOK},
		{http.MethodGet, "/api/private/project/:idProject/wiki/agent/search", "/api/private/project/1/wiki/agent/search", http.StatusOK},
		{http.MethodGet, "/api/private/state", "/api/private/state", http.StatusOK},
		{http.MethodGet, "/api/private/severity", "/api/private/severity", http.StatusOK},
		{http.MethodGet, "/api/private/issue-type", "/api/private/issue-type", http.StatusOK},
		{http.MethodGet, "/api/private/tag", "/api/private/tag", http.StatusOK},
		{http.MethodGet, "/api/private/custom-field", "/api/private/custom-field", http.StatusOK},
		{http.MethodGet, "/api/private/user", "/api/private/user", http.StatusOK},
		{http.MethodPost, "/api/private/agent/task/:idTask/complete", "/api/private/agent/task/4/complete", http.StatusOK},

		{http.MethodGet, "/api/private/projects", "/api/private/projects", http.StatusForbidden},
		{http.MethodGet, "/api/private/my-issues", "/api/private/my-issues", http.StatusForbidden},
		{http.MethodGet, "/api/private/track", "/api/private/track", http.StatusForbidden},
		{http.MethodGet, "/api/private/message", "/api/private/message?idRecipient=7&idMessageRecipientType=4", http.StatusOK},
		{http.MethodGet, "/api/private/message", "/api/private/message?idRecipient=1&idMessageRecipientType=3", http.StatusOK},
		{http.MethodGet, "/api/private/message", "/api/private/message", http.StatusForbidden},
		{http.MethodGet, "/api/private/message", "/api/private/message?idRecipient=2&idMessageRecipientType=1", http.StatusForbidden},
		{http.MethodGet, "/api/private/message", "/api/private/message?idRecipient=2&idMessageRecipientType=2", http.StatusForbidden},
		{http.MethodGet, "/api/private/message", "/api/private/message?idMessageRecipientType=1&idMessageRecipientType=4", http.StatusForbidden},
		{http.MethodGet, "/api/private/message/unread", "/api/private/message/unread", http.StatusForbidden},
		{http.MethodGet, "/api/private/ws", "/api/private/ws", http.StatusForbidden},
		{http.MethodGet, "/api/private/admin/user", "/api/private/admin/user", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/run/:idRun", "/api/private/agent/run/9", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/conversation/:id/thinking", "/api/private/agent/conversation/9/thinking?idTask=4", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/conversation", "/api/private/agent/conversation", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/conversation", "/api/private/agent/conversation", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/conversation/agents", "/api/private/agent/conversation/agents", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/conversation/unread-count", "/api/private/agent/conversation/unread-count", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/conversation/:id", "/api/private/agent/conversation/9", http.StatusForbidden},
		{http.MethodDelete, "/api/private/agent/conversation/:id", "/api/private/agent/conversation/9", http.StatusForbidden},
		{http.MethodGet, "/api/private/agent/conversation/:id/message", "/api/private/agent/conversation/9/message", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/conversation/:id/message", "/api/private/agent/conversation/9/message", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/conversation/:id/project", "/api/private/agent/conversation/9/project", http.StatusForbidden},
		{http.MethodDelete, "/api/private/agent/conversation/:id/project/:idProject", "/api/private/agent/conversation/9/project/1", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/conversation/:id/read", "/api/private/agent/conversation/9/read", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/conversation/:id/task-draft", "/api/private/agent/conversation/9/task-draft", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/conversation/:id/retry", "/api/private/agent/conversation/9/retry", http.StatusForbidden},
		{http.MethodPut, "/api/private/agent/conversation/:id/pin", "/api/private/agent/conversation/9/pin", http.StatusForbidden},
		{http.MethodPut, "/api/private/agent/conversation/:id/settings", "/api/private/agent/conversation/9/settings", http.StatusForbidden},
		{http.MethodPost, "/api/private/project/:idProject/issue", "/api/private/project/1/issue", http.StatusForbidden},
		{http.MethodPatch, "/api/private/project/:idProject/issue/:idIssuePublic", "/api/private/project/1/issue/2", http.StatusForbidden},
		{http.MethodDelete, "/api/private/project/:idProject/issue/:idIssuePublic", "/api/private/project/1/issue/2", http.StatusForbidden},
		{http.MethodPut, "/api/private/project/:idProject", "/api/private/project/1", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/task/:idTask/heartbeat", "/api/private/agent/task/4/heartbeat", http.StatusForbidden},
		{http.MethodPost, "/api/private/agent/task/:idTask/thinking", "/api/private/agent/task/4/thinking", http.StatusForbidden},
		{http.MethodHead, "/api/private/project", "/api/private/project", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			recorder, _ := performChatRequest(t, validRunTokenAuth(), &fakeApiKeyAuth{}, tc.method, tc.route, tc.target)
			assert.Equal(t, tc.want, recorder.Code)
		})
	}
}

// A session or API-key request never carries a run scope.
func TestAuthApiKeyRequestHasNoRunScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiKeyAuth := &fakeApiKeyAuth{session: &model.ApiKeySession{IdApiKey: 7, User: model.User{IdUser: 5, IsActive: true}}}
	runTokenAuth := &fakeRunTokenAuth{}
	hasScope := true
	router.POST("/api/private/project/:idProject/issue", Auth(&fakeSessionCache{err: redis.Nil}, apiKeyAuth, runTokenAuth), func(c *gin.Context) {
		_, hasScope = extctx.GetRunScope(c.Request.Context())
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/private/project/1/issue", nil)
	req.Header.Set("Authorization", "Bearer "+rawApiKeyBearer)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, hasScope)
	assert.False(t, runTokenAuth.lookupCalled)
}
