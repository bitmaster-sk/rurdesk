package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// Cors answers cross-origin requests.
//
// The default is `*`, which is safe here because authentication is a bearer
// token the caller has to attach deliberately — never a cookie the browser
// sends on its own — so a wildcard grants no ambient authority. Deployments that
// want the browser to enforce a narrower set anyway list them in
// ALLOWED_ORIGINS (comma-separated); an origin outside the list then gets no
// CORS headers at all.
func Cors() gin.HandlerFunc {
	allowed := allowedOrigins()

	return func(c *gin.Context) {
		if origin := resolveAllowOrigin(allowed, c.GetHeader("Origin")); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			// The response varies per request Origin once a list is configured,
			// so caches must not reuse one origin's response for another.
			if len(allowed) > 0 {
				c.Header("Vary", "Origin")
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func allowedOrigins() []string {
	raw := strings.TrimSpace(viper.GetString("ALLOWED_ORIGINS"))
	if raw == "" {
		return nil
	}
	var out []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// resolveAllowOrigin returns the value for Access-Control-Allow-Origin, or ""
// when the request origin is not permitted.
func resolveAllowOrigin(allowed []string, requestOrigin string) string {
	if len(allowed) == 0 {
		return "*"
	}
	for _, o := range allowed {
		if strings.EqualFold(o, requestOrigin) {
			return requestOrigin
		}
	}
	return ""
}
