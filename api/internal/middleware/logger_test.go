package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestLogger_HidesQueryOnSsoAndScimPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		target        string
		isQueryLogged bool
	}{
		{"/api/public/sso/oidc/callback?code=secret-code&state=s1", false},
		{"/api/scim/v2/Users?filter=userName%20eq%20%22secret%22", false},
		{"/api/private/issue?query=secret-visible", true},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			var logged bytes.Buffer
			engine := gin.New()
			engine.Use(Logger(zerolog.New(&logged)))
			engine.GET("/*any", func(c *gin.Context) { c.Status(http.StatusOK) })

			engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.isQueryLogged, strings.Contains(logged.String(), "secret"))
		})
	}
}
