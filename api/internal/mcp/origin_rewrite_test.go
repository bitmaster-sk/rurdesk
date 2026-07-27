package mcp

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// The origin returned here becomes the SSE endpoint URL the MCP client posts to
// with its Authorization bearer, so a forged header must never be able to steer
// it at another host.
func TestClientOrigin(t *testing.T) {
	tests := []struct {
		name           string
		publicBaseURL  string
		host           string
		forwardedHost  string
		forwardedProto string
		tlsConn        bool
		want           string
	}{
		{
			name: "plain request uses the Host header",
			host: "tracker.example.com",
			want: "http://tracker.example.com",
		},
		{
			name:          "forged X-Forwarded-Host is ignored",
			host:          "tracker.example.com",
			forwardedHost: "attacker.example.net",
			want:          "http://tracker.example.com",
		},
		{
			name:    "TLS connection advertises https",
			host:    "tracker.example.com",
			tlsConn: true,
			want:    "https://tracker.example.com",
		},
		{
			name:           "X-Forwarded-Proto https survives a terminating proxy",
			host:           "tracker.example.com",
			forwardedProto: "https",
			want:           "https://tracker.example.com",
		},
		{
			name:           "X-Forwarded-Proto is matched case-insensitively",
			host:           "tracker.example.com",
			forwardedProto: "HTTPS",
			want:           "https://tracker.example.com",
		},
		{
			name:           "X-Forwarded-Proto cannot change the host",
			host:           "tracker.example.com",
			forwardedHost:  "attacker.example.net",
			forwardedProto: "https",
			want:           "https://tracker.example.com",
		},
		{
			name:          "configured base URL wins over the request",
			publicBaseURL: "https://pinned.example.com",
			host:          "tracker.example.com",
			forwardedHost: "attacker.example.net",
			want:          "https://pinned.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/mcp/sse", nil)
			request.Host = tt.host
			if tt.forwardedHost != "" {
				request.Header.Set("X-Forwarded-Host", tt.forwardedHost)
			}
			if tt.forwardedProto != "" {
				request.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.tlsConn {
				request.TLS = &tls.ConnectionState{}
			}

			handler := &originRewriteHandler{publicBaseURL: tt.publicBaseURL}

			assert.Equal(t, tt.want, handler.clientOrigin(request))
		})
	}
}

func TestLoadConfig_PublicBaseURL(t *testing.T) {
	tests := []struct {
		name string
		set  string
		want string
	}{
		{name: "unset means derive per request", set: "", want: ""},
		{name: "trailing slash is trimmed", set: "https://tracker.example.com/", want: "https://tracker.example.com"},
		{name: "surrounding whitespace is trimmed", set: "  https://tracker.example.com  ", want: "https://tracker.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := viper.GetString("MCP_PUBLIC_BASE_URL")
			viper.Set("MCP_PUBLIC_BASE_URL", tt.set)
			defer viper.Set("MCP_PUBLIC_BASE_URL", previous)

			assert.Equal(t, tt.want, LoadConfig().PublicBaseURL)
		})
	}
}
