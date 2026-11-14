package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type enforceUser struct {
	idUser int64
	email  string
	token  string
}

func (f *ssoFixture) newPasswordUser(t *testing.T, prefix string, isAdmin bool) enforceUser {
	email := uniqueSsoEmail(prefix)
	t.Cleanup(func() { deleteUsers(f.app, email) })
	token := createUserAsAdmin(t, f.app, f.adminToken,
		fmt.Sprintf(`{"name":%q,"email":%q,"password":"secret1","isAdmin":%t}`, prefix, email, isAdmin))
	return enforceUser{idUser: idOfUser(t, f.app, f.adminToken, email), email: email, token: token}
}

func (f *ssoFixture) newBreakGlass(t *testing.T) enforceUser {
	user := f.newPasswordUser(t, "break-glass", true)
	res := patchUser(t, f.app, f.adminToken, user.idUser, `{"isBreakGlass":true}`)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	return user
}

func (f *ssoFixture) enforce(isEnforced bool) *http.Response {
	return Request(nil, f.app, "PUT", "/api/private/admin/sso/enforce", fmt.Sprintf(`{"isEnforced":%t}`, isEnforced), f.adminToken)
}

func (f *ssoFixture) requireEnforced(t *testing.T) {
	res := f.enforce(true)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
}

func passwordLogin(t *testing.T, f *ssoFixture, email, password string) (int, string) {
	res := Request(t, f.app, "POST", "/api/public/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password), "")
	return res.StatusCode, readBody(t, res)
}

func sessionStatus(t *testing.T, f *ssoFixture, token string) int {
	return Request(t, f.app, "GET", "/api/private/user", "", token).StatusCode
}

func TestSsoEnforce_OnlyBreakGlassSignsInWithAPassword(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	breakGlass := f.newBreakGlass(t)
	regular := f.newPasswordUser(t, "regular", false)
	f.requireEnforced(t)

	rightStatus, rightBody := passwordLogin(t, f, regular.email, "secret1")
	wrongStatus, wrongBody := passwordLogin(t, f, regular.email, "wrong-password")
	assert.Equal(t, wrongStatus, rightStatus, "a correct password must look like a wrong one")
	assert.Equal(t, wrongBody, rightBody)
	assert.NotEqual(t, http.StatusOK, rightStatus)

	status, body := passwordLogin(t, f, breakGlass.email, "secret1")
	assert.Equal(t, http.StatusOK, status, body)

	res := Request(t, f.app, "GET", "/api/public/auth/methods", "", "")
	var methods model.AuthMethodsRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&methods))
	require.NotNil(t, methods.Sso)
	assert.True(t, methods.Sso.IsEnforced)
	assert.Len(t, f.state(t).BreakGlassAccounts, 1)
}

func TestSsoEnforce_NeedsActiveSsoAndABreakGlassAccount(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})

	notEnabled := f.enforce(true)
	assert.Equal(t, http.StatusUnprocessableEntity, notEnabled.StatusCode)
	assert.Contains(t, readBody(t, notEnabled), "SSO_NOT_ENABLED")

	f.activate(t, true)
	withoutBreakGlass := f.enforce(true)
	assert.Equal(t, http.StatusUnprocessableEntity, withoutBreakGlass.StatusCode)
	assert.Contains(t, readBody(t, withoutBreakGlass), "SSO_BREAK_GLASS_REQUIRED")
	assert.False(t, f.state(t).IsEnforced)
}

func TestSsoEnforce_SignsOutEveryoneButBreakGlassAndTheActingSession(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	breakGlass := f.newBreakGlass(t)
	regular := f.newPasswordUser(t, "regular", false)
	otherAdminSession := Token(t, f.app)

	f.requireEnforced(t)

	assert.Equal(t, http.StatusOK, sessionStatus(t, f, f.adminToken), "the acting session stays")
	assert.Equal(t, http.StatusOK, sessionStatus(t, f, breakGlass.token))
	assert.Equal(t, http.StatusUnauthorized, sessionStatus(t, f, otherAdminSession))
	assert.Equal(t, http.StatusUnauthorized, sessionStatus(t, f, regular.token))
}

func TestSsoEnforce_TheLastBreakGlassAccountStays(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	breakGlass := f.newBreakGlass(t)
	second := f.newPasswordUser(t, "second-admin", true)
	f.requireEnforced(t)

	for _, body := range []string{`{"isActive":false}`, `{"isAdmin":false}`, `{"isBreakGlass":false}`} {
		res := patchUser(t, f.app, f.adminToken, breakGlass.idUser, body)
		assert.Equal(t, http.StatusConflict, res.StatusCode, body)
		assert.Contains(t, readBody(t, res), "LAST_BREAK_GLASS", body)
	}
	deleted := Request(t, f.app, "DELETE", fmt.Sprintf("/api/private/admin/user/%d", breakGlass.idUser), "", f.adminToken)
	assert.Equal(t, http.StatusConflict, deleted.StatusCode)

	require.Equal(t, http.StatusOK, patchUser(t, f.app, f.adminToken, second.idUser, `{"isBreakGlass":true}`).StatusCode)
	res := patchUser(t, f.app, f.adminToken, breakGlass.idUser, `{"isBreakGlass":false}`)
	assert.Equal(t, http.StatusOK, res.StatusCode, "a second break-glass account frees the first")
}

func TestSsoEnforce_DisablingSsoTurnsEnforcementOff(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	f.newBreakGlass(t)
	regular := f.newPasswordUser(t, "regular", false)
	f.requireEnforced(t)

	require.Equal(t, http.StatusOK, f.admin(t, "POST", "/disable", "").StatusCode)

	assert.False(t, f.state(t).IsEnforced)
	status, body := passwordLogin(t, f, regular.email, "secret1")
	assert.Equal(t, http.StatusOK, status, body)
}

func TestSsoEnforce_WithoutALicense(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	breakGlass := f.newBreakGlass(t)
	regular := f.newPasswordUser(t, "regular", false)
	f.requireEnforced(t)
	f.app = rebuildWithLicense(t, license.Free{})

	status, body := passwordLogin(t, f, regular.email, "secret1")
	assert.Equal(t, http.StatusOK, status, "an expired license never locks people out: %s", body)
	assert.Equal(t, http.StatusForbidden, f.enforce(true).StatusCode)

	res := patchUser(t, f.app, f.adminToken, breakGlass.idUser, `{"isBreakGlass":false}`)
	assert.Equal(t, http.StatusConflict, res.StatusCode, "the stored flag still guards the last break-glass account")

	assert.Equal(t, http.StatusOK, f.enforce(false).StatusCode)
	assert.False(t, f.state(t).IsEnforced)
}
