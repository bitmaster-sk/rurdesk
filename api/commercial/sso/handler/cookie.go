package handler

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/gin-gonic/gin"
)

func setBindingCookie(c *gin.Context, binding flow.Binding) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     flow.CookieName(binding.Hash),
		Value:    binding.Value,
		Path:     flow.CookiePath,
		MaxAge:   int(flow.StateTtl.Seconds()),
		HttpOnly: true,
		Secure:   isSecureDeployment(),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearBindingCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Path:     flow.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureDeployment(),
		SameSite: http.SameSiteLaxMode,
	})
}

func isSecureDeployment() bool {
	baseUrl, err := publicurl.Load()
	return err == nil && strings.HasPrefix(baseUrl, "https://")
}

const redirectedParam = "redirected"

func redirectedTarget(requestUrl *url.URL) string {
	query := requestUrl.Query()
	query.Set(redirectedParam, "true")
	return requestUrl.Path + "?" + query.Encode()
}

func isSameHost(requestHost, baseUrl string) bool {
	parsed, err := url.Parse(baseUrl)
	if err != nil {
		return false
	}
	host := requestHost
	if withoutPort, _, err := net.SplitHostPort(requestHost); err == nil {
		host = withoutPort
	}
	return strings.EqualFold(strings.Trim(host, "[]"), parsed.Hostname())
}
