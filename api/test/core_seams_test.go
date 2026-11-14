package test

import (
	"context"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeAuthMethods struct{}

var _ authpolicy.AuthMethodsProvider = fakeAuthMethods{}

func (fakeAuthMethods) Methods(context.Context) model.AuthMethodsRes {
	return model.AuthMethodsRes{Password: true, Sso: &model.SsoMethod{Protocol: "oidc"}}
}

func TestAuthMethods_FreeDefaultOffersPasswordOnly(t *testing.T) {
	app := Setup(t)
	res := Request(t, app, "GET", "/api/public/auth/methods", "", "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.JSONEq(t, `{"password":true,"sso":null,"ssoUnavailable":false}`, readBody(t, res))
}

func TestAuthMethods_InstalledProviderIsUsedWithoutRebuild(t *testing.T) {
	app := Setup(t)
	injector.SetAuthMethodsProvider(fakeAuthMethods{})
	t.Cleanup(func() { injector.SetAuthMethodsProvider(authpolicy.Free{}) })

	res := Request(t, app, "GET", "/api/public/auth/methods", "", "")
	require.JSONEq(t, `{"password":true,"sso":{"protocol":"oidc","isEnforced":false,"canStaySignedIn":false},"ssoUnavailable":false}`, readBody(t, res))
}

func TestRouteExtension_RegistersRoutesOnTheSharedGroups(t *testing.T) {
	Setup(t)
	injector.SetRouteExtension(func(groups router.RouteGroups) {
		groups.Public.GET("/extension-probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		groups.Admin.GET("/extension-probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	})
	for _, key := range []string{"router", "http-server"} {
		injector.Clear(key)
	}
	app, err := issue.New()
	require.NoError(t, err)
	t.Cleanup(func() { Setup(t) })

	require.Equal(t, http.StatusNoContent, Request(t, app, "GET", "/api/public/extension-probe", "", "").StatusCode)
	require.Equal(t, http.StatusUnauthorized, Request(t, app, "GET", "/api/private/admin/extension-probe", "", "").StatusCode)
	require.Equal(t, http.StatusNoContent, Request(t, app, "GET", "/api/private/admin/extension-probe", "", Token(t, app)).StatusCode)
}
