package test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSsoLogin_CreatesAnAccountAndA24HourSession(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email := uniqueSsoEmail("jit")

	session := f.signIn(t, "?extended=true&returnUrl=%2Fissue%2F5", ssoClaims("sub-"+email, strings.ToUpper(email)))

	assert.Equal(t, "/issue/5", session.ReturnUrl)
	res := Request(t, f.app, "GET", "/api/private/user", "", session.Token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var user struct {
		Email          string `json:"email"`
		IsPasswordless bool   `json:"isPasswordless"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&user))
	assert.Equal(t, email, user.Email)
	assert.True(t, user.IsPasswordless)

	cache, err := injector.GetCache()
	require.NoError(t, err)
	ttl := cache.TTL(context.Background(), session.Token).Val()
	assert.Greater(t, ttl, 23*time.Hour)
	assert.LessOrEqual(t, ttl, 24*time.Hour)
}

func TestSsoLogin_StartSetsTheBindingCookie(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)

	start := f.start(t, "")

	assert.True(t, strings.HasPrefix(start.authUrl, f.idp.Issuer()+"/authorize?"))
	require.Len(t, start.cookies, 1)
	cookie := start.cookies[0]
	assert.True(t, strings.HasPrefix(cookie.Name, "rurdesk_sso_"))
	assert.Equal(t, "/api/public/sso", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, 600, cookie.MaxAge)
}

func TestSsoLogin_StateAndCodeAreSingleUse(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email := uniqueSsoEmail("once")
	start := f.start(t, "")
	values := f.authorize(t, start.authUrl, ssoClaims("sub-"+email, email))

	code := codeFrom(t, f.callbackWith(values))
	assert.Equal(t, "invalid_state", ssoErrorFrom(t, f.callbackWith(values)))

	assert.Equal(t, http.StatusOK, f.exchange(code, start.cookies).StatusCode)
	res := f.exchange(code, start.cookies)
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	assert.Contains(t, readBody(t, res), "SSO_INVALID_CODE")
}

func TestSsoLogin_RejectsAWrongNonceAndAForeignSignature(t *testing.T) {
	for name, options := range map[string]ssotest.Options{
		"wrong nonce":       {IsNonceWrong: true},
		"foreign signature": {IsSignedByOtherKey: true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSsoFixture(t, ssoLicense, options)
			f.activate(t, true)
			email := uniqueSsoEmail("token")

			res := f.callback(t, f.start(t, ""), ssoClaims("sub-"+email, email))

			assert.Equal(t, "idp_error", ssoErrorFrom(t, res))
		})
	}
}

func TestSsoLogin_ExchangeIsBoundToTheBrowser(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email := uniqueSsoEmail("binding")
	claims := ssoClaims("sub-"+email, email)

	withoutCookie := f.start(t, "")
	assert.Equal(t, http.StatusUnprocessableEntity, f.exchange(codeFrom(t, f.callback(t, withoutCookie, claims)), nil).StatusCode)

	victim, attacker := f.start(t, ""), f.start(t, "")
	assert.Equal(t, http.StatusUnprocessableEntity, f.exchange(codeFrom(t, f.callback(t, victim, claims)), attacker.cookies).StatusCode)

	first, second := f.start(t, ""), f.start(t, "")
	firstCode := codeFrom(t, f.callback(t, first, claims))
	secondCode := codeFrom(t, f.callback(t, second, claims))
	bothCookies := append(append([]*http.Cookie{}, first.cookies...), second.cookies...)
	assert.Equal(t, http.StatusOK, f.exchange(secondCode, bothCookies).StatusCode)
	assert.Equal(t, http.StatusOK, f.exchange(firstCode, bothCookies).StatusCode)
}

func TestSsoLogin_ExchangeClearsTheCookie(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email := uniqueSsoEmail("clear")
	start := f.start(t, "")

	res := f.exchange(codeFrom(t, f.callback(t, start, ssoClaims("sub-"+email, email))), start.cookies)

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Len(t, res.Cookies(), 1)
	assert.Equal(t, start.cookies[0].Name, res.Cookies()[0].Name)
	assert.Less(t, res.Cookies()[0].MaxAge, 0)
}

func TestSsoStart_MovesToThePublicHostFirst(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)

	res := f.do("GET", "/api/public/sso/start?returnUrl=%2Fx", "", "other.example.org", nil)

	require.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, ssoBaseUrl+"/api/public/sso/start?redirected=true&returnUrl=%2Fx", res.Header.Get("Location"))
	assert.Empty(t, res.Cookies())

	email := uniqueSsoEmail("moved")
	session := f.signIn(t, "?redirected=true&returnUrl=%2Fx", ssoClaims("sub-"+email, email))
	assert.Equal(t, "/x", session.ReturnUrl)
}

func TestSsoStart_ReachesTheIdpBehindAHostRewritingProxy(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	const proxiedHost = "tracker:1000"

	moved := f.do("GET", "/api/public/sso/start", "", proxiedHost, nil)
	require.Equal(t, http.StatusFound, moved.StatusCode)
	require.Equal(t, ssoBaseUrl+"/api/public/sso/start?redirected=true", moved.Header.Get("Location"))
	assert.Empty(t, moved.Cookies())

	res := f.do("GET", strings.TrimPrefix(moved.Header.Get("Location"), ssoBaseUrl), "", proxiedHost, nil)
	require.Equal(t, http.StatusFound, res.StatusCode)
	start := ssoStart{authUrl: res.Header.Get("Location"), cookies: res.Cookies()}
	require.True(t, strings.HasPrefix(start.authUrl, f.idp.Issuer()), "the second request goes on to the identity provider: %s", start.authUrl)
	require.Len(t, start.cookies, 1)

	email := uniqueSsoEmail("proxied")
	exchanged := f.exchange(codeFrom(t, f.callback(t, start, ssoClaims("sub-"+email, email))), start.cookies)
	assert.Equal(t, http.StatusOK, exchanged.StatusCode)
}

func TestSsoLogin_ReplacesAnUnsafeReturnUrl(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email := uniqueSsoEmail("return")

	session := f.signIn(t, "?returnUrl=%2F%2Fevil.com", ssoClaims("sub-"+email, email))

	assert.Equal(t, "/", session.ReturnUrl)
}

func TestSsoLogin_RedirectsWithTheRejectionReason(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, false)

	unknown := uniqueSsoEmail("unknown")
	assert.Equal(t, "account_not_provisioned", ssoErrorFrom(t, f.callback(t, f.start(t, ""), ssoClaims("sub-"+unknown, unknown))))

	foreign := strings.Replace(uniqueSsoEmail("foreign"), "acme.test", "other.test", 1)
	assert.Equal(t, "domain_not_allowed", ssoErrorFrom(t, f.callback(t, f.start(t, ""), ssoClaims("sub-"+foreign, foreign))))
}

func TestSsoLogin_LinksAnExistingAccount(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, false)
	email := uniqueSsoEmail("existing")
	userToken := createUserAsAdmin(t, f.app, f.adminToken, `{"name":"Jana Nováková","email":"`+email+`","password":"kreslo123"}`)
	require.NotEmpty(t, userToken)

	session := f.signIn(t, "", ssoClaims("sub-"+email, email))

	res := Request(t, f.app, "GET", "/api/private/user", "", session.Token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, readBody(t, res), email)
}

func TestSsoLogin_WithoutALicense(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	injector.SetLicenseProvider(license.Free{})

	res := f.do("GET", "/api/public/sso/start", "", "", nil)
	assert.Equal(t, "sso_unavailable", ssoErrorFrom(t, res))

	exchange := f.exchange("anything", nil)
	assert.Equal(t, http.StatusForbidden, exchange.StatusCode)
	assert.Contains(t, readBody(t, exchange), "LICENSE_REQUIRED")

	methods := Request(t, f.app, "GET", "/api/public/auth/methods", "", "")
	assert.JSONEq(t, `{"password":true,"sso":null,"ssoUnavailable":true}`, readBody(t, methods))
}

func TestSsoRoutes_AreMissingWithoutTheHook(t *testing.T) {
	app := Setup(t)

	assert.Equal(t, http.StatusNotFound, Request(t, app, "GET", "/api/public/sso/start", "", "").StatusCode)
	assert.Equal(t, http.StatusNotFound, Request(t, app, "POST", "/api/public/sso/exchange", `{"code":"x"}`, "").StatusCode)
}

func TestAuthMethods_FollowTheSsoState(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	methods := func() string { return readBody(t, Request(t, f.app, "GET", "/api/public/auth/methods", "", "")) }

	assert.JSONEq(t, `{"password":true,"sso":null,"ssoUnavailable":false}`, methods())

	f.activate(t, true)
	assert.JSONEq(t, `{"password":true,"sso":{"protocol":"oidc","isEnforced":false,"canStaySignedIn":false},"ssoUnavailable":false}`, methods())

	require.Equal(t, http.StatusOK, f.admin(t, "PUT", "/scim", `{"isEnabled":true}`).StatusCode)
	assert.Contains(t, methods(), `"canStaySignedIn":false`, "SCIM without a token cannot end sessions")
	require.Equal(t, http.StatusOK, f.admin(t, "POST", "/scim/token", "").StatusCode)
	assert.Contains(t, methods(), `"canStaySignedIn":true`)

	viper.Set(publicurl.EnvBaseUrl, "http://tracker.example.org")
	assert.JSONEq(t, `{"password":true,"sso":null,"ssoUnavailable":true}`, methods())
}
