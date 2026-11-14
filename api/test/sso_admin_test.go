package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ssoAdminPath = "/api/private/admin/sso"

func (f *ssoFixture) admin(t *testing.T, method, path, body string) *http.Response {
	return Request(t, f.app, method, ssoAdminPath+path, body, f.adminToken)
}

func (f *ssoFixture) saveDraft(t *testing.T, changes ...func(*config.SaveDraftReq)) {
	req := f.draft()
	for _, change := range changes {
		change(&req)
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)
	res := f.admin(t, "PUT", "/draft", string(body))
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func (f *ssoFixture) state(t *testing.T) config.StateRes {
	res := f.admin(t, "GET", "", "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var state config.StateRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&state))
	return state
}

func (f *ssoFixture) startTest(t *testing.T) string {
	res := f.admin(t, "POST", "/test", "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var started struct {
		Url string `json:"url"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&started))
	return started.Url
}

type ssoTestResult struct {
	IsSuccess bool     `json:"isSuccess"`
	Error     string   `json:"error"`
	Detail    string   `json:"detail"`
	Subject   string   `json:"subject"`
	Email     string   `json:"email"`
	Action    string   `json:"action"`
	Reason    string   `json:"reason"`
	Warnings  []string `json:"warnings"`
}

func (f *ssoFixture) testResult(t *testing.T, res *http.Response) ssoTestResult {
	require.Equal(t, http.StatusFound, res.StatusCode)
	location, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/admin/settings/sso", location.Path)
	loaded := f.admin(t, "GET", "/test/"+location.Query().Get("test"), "")
	require.Equal(t, http.StatusOK, loaded.StatusCode)
	var result ssoTestResult
	require.NoError(t, json.NewDecoder(loaded.Body).Decode(&result))
	return result
}

func countRows(t *testing.T, f *ssoFixture, query string) int {
	var count int
	require.NoError(t, f.app.Pool.QueryRow(context.Background(), query).Scan(&count))
	return count
}

func TestSsoAdmin_StateShowsIdpDataWithoutTheSecret(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)

	res := f.admin(t, "GET", "", "")
	body := readBody(t, res)

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.NotContains(t, body, ssotest.ClientSecret)
	var state config.StateRes
	require.NoError(t, json.Unmarshal([]byte(body), &state))
	assert.True(t, state.IsLicensed)
	assert.True(t, state.HasEncryptionKey)
	assert.Equal(t, ssoBaseUrl, state.PublicBaseUrl)
	assert.Equal(t, ssoBaseUrl+"/api/public/sso/oidc/callback", state.RedirectUri)
	require.NotNil(t, state.Draft)
	assert.True(t, state.Draft.HasClientSecret)
	assert.False(t, state.IsDraftTested)
}

func TestSsoAdmin_RejectsAMultiTenantIssuer(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})

	res := f.admin(t, "PUT", "/draft", `{"protocol":"oidc","issuer":"https://login.microsoftonline.com/common/v2.0","clientId":"x","clientSecret":"y"}`)

	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	assert.Contains(t, readBody(t, res), "SSO_ISSUER_MULTI_TENANT")
}

func TestSsoAdmin_ConnectionTestCreatesNothingAndUnlocksActivation(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)
	rules := f.admin(t, "PUT", "/settings", `{"allowedDomains":["acme.test"],"isAutoCreate":true}`)
	require.Equal(t, http.StatusOK, rules.StatusCode)
	assert.Equal(t, http.StatusUnprocessableEntity, f.admin(t, "POST", "/activate", "").StatusCode)
	users := countRows(t, f, `SELECT count(*) FROM users.user`)
	email := uniqueSsoEmail("probe")

	result := f.testResult(t, f.callbackWith(f.authorize(t, f.startTest(t), ssoClaims("sub-"+email, email))))

	assert.True(t, result.IsSuccess)
	assert.Equal(t, "sub-"+email, result.Subject)
	assert.Equal(t, "create", result.Action)
	assert.Equal(t, []string{"groups_claim_missing"}, result.Warnings)
	assert.Equal(t, users, countRows(t, f, `SELECT count(*) FROM users.user`))
	assert.Zero(t, countRows(t, f, `SELECT count(*) FROM users.user_identity`))
	assert.True(t, f.state(t).IsDraftTested)

	activated := f.admin(t, "POST", "/activate", "")
	require.Equal(t, http.StatusOK, activated.StatusCode)
	assert.True(t, f.state(t).IsEnabled)
}

func TestSsoAdmin_TestShowsAPredictedRejection(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)
	email := strings.Replace(uniqueSsoEmail("foreign"), "acme.test", "other.test", 1)

	result := f.testResult(t, f.callbackWith(f.authorize(t, f.startTest(t), ssoClaims("sub-"+email, email))))

	assert.True(t, result.IsSuccess)
	assert.Equal(t, "reject", result.Action)
	assert.Equal(t, "domain_not_allowed", result.Reason)
	assert.Contains(t, result.Warnings, "domain_not_allowed")
}

func TestSsoAdmin_ChangingTheDraftDuringTheTestInvalidatesIt(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)
	authUrl := f.startTest(t)
	f.saveDraft(t, func(req *config.SaveDraftReq) { req.EmailClaim = "preferred_username" })
	email := uniqueSsoEmail("changed")

	result := f.testResult(t, f.callbackWith(f.authorize(t, authUrl, ssoClaims("sub-"+email, email))))

	assert.False(t, result.IsSuccess)
	assert.Equal(t, "draft_changed", result.Error)
	assert.Equal(t, http.StatusUnprocessableEntity, f.admin(t, "POST", "/activate", "").StatusCode)
}

func TestSsoAdmin_TestReportsAnIdpError(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)
	authUrl, err := url.Parse(f.startTest(t))
	require.NoError(t, err)

	res := f.callbackWith(url.Values{"state": {authUrl.Query().Get("state")}, "error": {"access_denied"}})

	result := f.testResult(t, res)
	assert.False(t, result.IsSuccess)
	assert.Equal(t, "idp_error", result.Error)
}

func TestSsoAdmin_TestAndActivationNeedAValidPublicUrl(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)
	email := uniqueSsoEmail("url")
	f.testResult(t, f.callbackWith(f.authorize(t, f.startTest(t), ssoClaims("sub-"+email, email))))

	viper.Set(publicurl.EnvBaseUrl, "http://tracker.example.org")

	for _, path := range []string{"/test", "/activate"} {
		res := f.admin(t, "POST", path, "")
		assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode, path)
		assert.Contains(t, readBody(t, res), "SSO_PUBLIC_URL_INVALID", path)
	}
}

func TestSsoAdmin_DisableTurnsSsoOff(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)

	res := f.admin(t, "POST", "/disable", "")

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.False(t, f.state(t).IsEnabled)
	methods := Request(t, f.app, "GET", "/api/public/auth/methods", "", "")
	assert.JSONEq(t, `{"password":true,"sso":null,"ssoUnavailable":false}`, readBody(t, methods))
}

func TestSsoAdmin_RulesAreNormalized(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})

	res := f.admin(t, "PUT", "/settings", `{"allowedDomains":[" Acme.TEST ","acme.test"],"isAutoCreate":true,"adminGroup":" rurdesk-admins "}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	rules := f.state(t).Rules
	assert.Equal(t, []string{"acme.test"}, rules.AllowedDomains)
	assert.Equal(t, "rurdesk-admins", rules.AdminGroup)

	invalid := f.admin(t, "PUT", "/settings", `{"allowedDomains":["not a domain"]}`)
	assert.Equal(t, http.StatusUnprocessableEntity, invalid.StatusCode)
}

func TestSsoAdmin_WithoutALicenseOnlyStateAndDisableWork(t *testing.T) {
	f := newSsoFixture(t, license.Free{}, ssotest.Options{})

	assert.False(t, f.state(t).IsLicensed)
	assert.Equal(t, http.StatusOK, f.admin(t, "POST", "/disable", "").StatusCode)
	for _, call := range []struct{ method, path, body string }{
		{"PUT", "/draft", `{"protocol":"oidc","issuer":"https://idp.example.com","clientId":"x","clientSecret":"y"}`},
		{"POST", "/test", ""},
		{"GET", "/test/abc", ""},
		{"POST", "/activate", ""},
		{"PUT", "/settings", `{"allowedDomains":[]}`},
	} {
		res := f.admin(t, call.method, call.path, call.body)
		assert.Equal(t, http.StatusForbidden, res.StatusCode, call.path)
		assert.Contains(t, readBody(t, res), "LICENSE_REQUIRED", call.path)
	}
}

func TestSsoAdmin_IsAdminOnly(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	email := uniqueSsoEmail("member")
	member := createUserAsAdmin(t, f.app, f.adminToken, `{"name":"Člen tímu","email":"`+email+`","password":"kreslo123"}`)

	res := Request(t, f.app, "GET", ssoAdminPath, "", member)

	assert.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestSsoAdmin_ReportsAnUndecryptableSecret(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.saveDraft(t)

	viper.Set(config.EnvEncryptionKey, ssoOtherKey)

	assert.True(t, f.state(t).IsSecretUndecryptable)
	assert.Equal(t, http.StatusUnprocessableEntity, f.admin(t, "POST", "/test", "").StatusCode)
}

func TestSsoAdmin_UnknownTestResultIsNotFound(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})

	assert.Equal(t, http.StatusNotFound, f.admin(t, "GET", "/test/unknown", "").StatusCode)
}
