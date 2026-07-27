package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SPA authenticates the socket via a subprotocol-offered token. The browser
// aborts unless the server selects one of the offered protocols, so the upgrader
// must echo the "Authorization" marker — never the token, which would leak it
// into response logs.
func TestUpgraderSelectsAuthorizationSubprotocol(t *testing.T) {
	const token = "jwt-token-value"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("hello"))
	}))
	defer server.Close()

	dialer := websocket.Dialer{Subprotocols: []string{"Authorization", token}}
	conn, resp, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer conn.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	assert.Equal(t, "Authorization", conn.Subprotocol())
	assert.NotContains(t, resp.Header.Get("Sec-WebSocket-Protocol"), token)
}

func Test_isAllowedWSOrigin(t *testing.T) {
	tests := []struct {
		name         string
		origin       string
		host         string
		allowedExtra []string
		want         bool
	}{
		{
			name:   "empty origin (non-browser client) is allowed",
			origin: "",
			host:   "tracker.example.com",
			want:   true,
		},
		{
			name:   "same-origin handshake is allowed",
			origin: "https://tracker.example.com",
			host:   "tracker.example.com",
			want:   true,
		},
		{
			name:   "same host with explicit port is allowed",
			origin: "http://tracker.example.com:1000",
			host:   "tracker.example.com:1000",
			want:   true,
		},
		{
			name:   "scheme mismatch but same host is allowed (TLS terminated upstream)",
			origin: "http://tracker.example.com",
			host:   "tracker.example.com",
			want:   true,
		},
		{
			name:   "host comparison is case-insensitive",
			origin: "https://Tracker.Example.COM",
			host:   "tracker.example.com",
			want:   true,
		},
		{
			name:   "cross-origin handshake is rejected (CSWSH)",
			origin: "https://evil.attacker.com",
			host:   "tracker.example.com",
			want:   false,
		},
		{
			name:   "different port is treated as cross-origin and rejected",
			origin: "https://tracker.example.com:8443",
			host:   "tracker.example.com",
			want:   false,
		},
		{
			name:   "malformed origin is rejected",
			origin: "://not a url",
			host:   "tracker.example.com",
			want:   false,
		},
		{
			name:   "origin with no host is rejected",
			origin: "https://",
			host:   "tracker.example.com",
			want:   false,
		},
		{
			name:         "whitelisted split-host origin is allowed",
			origin:       "https://app.example.com",
			host:         "api.example.com",
			allowedExtra: []string{"app.example.com"},
			want:         true,
		},
		{
			name:         "origin not in whitelist is still rejected",
			origin:       "https://evil.attacker.com",
			host:         "api.example.com",
			allowedExtra: []string{"app.example.com"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowedWSOrigin(tt.origin, tt.host, tt.allowedExtra)
			if got != tt.want {
				t.Errorf("isAllowedWSOrigin(%q, %q, %v) = %v, want %v",
					tt.origin, tt.host, tt.allowedExtra, got, tt.want)
			}
		})
	}
}
