package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/groups"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/bitmaster-sk/rurdesk/api/internal/router"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const (
	ssoTestKey  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	ssoOtherKey = "ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA="
	ssoBaseUrl  = "https://example.com"
)

var ssoLicense = stubLicense{features: []license.Feature{license.SSO}}

func useSsoEnv(t *testing.T) {
	previousKey, previousBaseUrl := viper.GetString(config.EnvEncryptionKey), viper.GetString(publicurl.EnvBaseUrl)
	viper.Set(config.EnvEncryptionKey, ssoTestKey)
	viper.Set(publicurl.EnvBaseUrl, ssoBaseUrl)
	t.Cleanup(func() {
		viper.Set(config.EnvEncryptionKey, previousKey)
		viper.Set(publicurl.EnvBaseUrl, previousBaseUrl)
	})
}

func resetSso(t *testing.T, app *issue.Application) {
	ctx := context.Background()
	for _, statement := range []string{
		`DELETE FROM users.user_identity`,
		`DELETE FROM users.sso_config`,
		`UPDATE users.sso_settings SET is_enabled = FALSE, allowed_domains = '{}', is_auto_create = FALSE, admin_group = '', is_enforced = FALSE, is_scim_enabled = FALSE`,
		`DELETE FROM users.scim_token`,
		`UPDATE users.sso_settings SET sp_cert = '', sp_key_enc = NULL, sp_key_nonce = NULL`,
		`UPDATE users.user SET is_break_glass = FALSE`,
		`DELETE FROM users.sso_group_mapping`,
	} {
		_, err := app.Pool.Exec(ctx, statement)
		require.NoError(t, err)
	}
}

func idOfTestUser(t *testing.T, app *issue.Application) int64 {
	var idUser int64
	err := app.Pool.QueryRow(context.Background(), `SELECT id_user FROM users.user WHERE email = 'test@test.sk'`).Scan(&idUser)
	require.NoError(t, err)
	return idUser
}

func newGroupSync(app *issue.Application) *groups.Service {
	return groups.NewService(app.Pool, injector.GetUserRepository(), injector.GetTeamRepository(), injector.GetAclService(), injector.GetUserService())
}

func uniqueSsoEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@acme.test", prefix, time.Now().UnixNano())
}

func installSso(t *testing.T, provider license.Provider) *issue.Application {
	useSsoEnv(t)
	sso.Install()
	t.Cleanup(func() {
		injector.SetRouteExtension(router.NoExtension)
		injector.SetAuthMethodsProvider(authpolicy.Free{})
		injector.SetPasswordLoginPolicy(authpolicy.Free{})
		injector.SetUserChangeGuard(authpolicy.Free{})
		injector.SetLicenseProvider(license.Free{})
	})
	return rebuildWithLicense(t, provider)
}

type ssoFixture struct {
	app        *issue.Application
	idp        *ssotest.Idp
	adminToken string
}

func newSsoFixture(t *testing.T, provider license.Provider, options ssotest.Options) *ssoFixture {
	Setup(t)
	app := installSso(t, provider)
	resetSso(t, app)
	t.Cleanup(func() { resetSso(t, app) })
	idp, err := ssotest.New(options)
	require.NoError(t, err)
	t.Cleanup(idp.Close)
	return &ssoFixture{app: app, idp: idp, adminToken: Token(t, app)}
}

func (f *ssoFixture) draft() config.SaveDraftReq {
	return config.SaveDraftReq{
		Protocol:     config.ProtocolOidc,
		Issuer:       f.idp.Issuer(),
		ClientId:     ssotest.ClientId,
		ClientSecret: ssotest.ClientSecret,
	}
}

func (f *ssoFixture) activate(t *testing.T, isAutoCreate bool) {
	ctx := context.Background()
	service := config.NewService(f.app.Pool)
	idAdmin := idOfTestUser(t, f.app)
	require.NoError(t, service.SaveDraft(ctx, idAdmin, f.draft()))
	_, hash, err := service.DraftForTest(ctx)
	require.NoError(t, err)
	require.NoError(t, service.MarkTested(ctx, hash))
	require.NoError(t, service.Activate(ctx, idAdmin))
	require.NoError(t, service.UpdateRules(ctx, config.UpdateRulesReq{AllowedDomains: []string{"acme.test"}, IsAutoCreate: isAutoCreate}))
}

func (f *ssoFixture) do(method, target, body, host string, cookies []*http.Cookie) *http.Response {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if host != "" {
		req.Host = host
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.app.ServeHTTP(w, req)
	return w.Result()
}

type ssoStart struct {
	authUrl string
	cookies []*http.Cookie
}

func (f *ssoFixture) start(t *testing.T, query string) ssoStart {
	res := f.do("GET", "/api/public/sso/start"+query, "", "", nil)
	require.Equal(t, http.StatusFound, res.StatusCode)
	return ssoStart{authUrl: res.Header.Get("Location"), cookies: res.Cookies()}
}

func (f *ssoFixture) authorize(t *testing.T, authUrl string, claims map[string]any) url.Values {
	values, err := f.idp.Authorize(authUrl, claims)
	require.NoError(t, err)
	return values
}

func (f *ssoFixture) callbackWith(values url.Values) *http.Response {
	return f.do("GET", "/api/public/sso/oidc/callback?"+values.Encode(), "", "", nil)
}

func (f *ssoFixture) callback(t *testing.T, start ssoStart, claims map[string]any) *http.Response {
	return f.callbackWith(f.authorize(t, start.authUrl, claims))
}

func codeFrom(t *testing.T, res *http.Response) string {
	require.Equal(t, http.StatusFound, res.StatusCode)
	location := res.Header.Get("Location")
	require.True(t, strings.HasPrefix(location, "/auth/sso#code="), location)
	return strings.TrimPrefix(location, "/auth/sso#code=")
}

func ssoErrorFrom(t *testing.T, res *http.Response) string {
	require.Equal(t, http.StatusFound, res.StatusCode)
	location, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/login", location.Path)
	return location.Query().Get("ssoError")
}

func (f *ssoFixture) exchange(code string, cookies []*http.Cookie) *http.Response {
	return f.do("POST", "/api/public/sso/exchange", `{"code":"`+code+`"}`, "", cookies)
}

type ssoSession struct {
	Token     string `json:"token"`
	ReturnUrl string `json:"returnUrl"`
}

func (f *ssoFixture) signIn(t *testing.T, query string, claims map[string]any) ssoSession {
	start := f.start(t, query)
	res := f.exchange(codeFrom(t, f.callback(t, start, claims)), start.cookies)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var session ssoSession
	require.NoError(t, json.Unmarshal([]byte(body), &session))
	return session
}

func ssoClaims(subject, email string) map[string]any {
	return map[string]any{"sub": subject, "email": email, "email_verified": true, "name": "Jana Nováková"}
}
