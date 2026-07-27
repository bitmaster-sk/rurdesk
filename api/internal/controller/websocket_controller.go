package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

// The SPA sends `Sec-WebSocket-Protocol: Authorization, <token>` because the
// browser WebSocket API can't set an Authorization header. The handshake fails
// unless we echo back one of the offered subprotocols, so we echo the marker —
// never the token itself, which would leak it into response logs.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin:  checkWSOrigin,
	Subprotocols: []string{"Authorization"},
}

// checkWSOrigin guards against Cross-Site WebSocket Hijacking. The
// token now lives in the handshake subprotocol, not the Authorization cookie,
// so a cross-site page can no longer authenticate via ambient credentials — but
// the origin check stays as defense in depth against any future ambient-
// credential mechanism reopening the hole. Reject any handshake whose Origin
// host does not match the request Host (same-origin is how the SPA connects).
func checkWSOrigin(r *http.Request) bool {
	return isAllowedWSOrigin(r.Header.Get("Origin"), r.Host, allowedWSOrigins())
}

// allowedWSOrigins returns extra origin hosts whitelisted via ALLOWED_WS_ORIGINS
// (comma-separated env), for split-host deployments where the SPA is served from
// a different host than the API. Empty by default — same-origin is the only gate.
func allowedWSOrigins() []string {
	raw := viper.GetString("ALLOWED_WS_ORIGINS")
	if raw == "" {
		return nil
	}
	hosts := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		if host := strings.TrimSpace(part); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// isAllowedWSOrigin decides whether a handshake with the given Origin may
// proceed against host. An empty Origin means a non-browser client (curl, Go,
// API-key integrations) with no ambient cookie — nothing to hijack — so it's
// allowed. A present Origin must match the request host or a whitelisted host.
func isAllowedWSOrigin(origin, host string, allowedExtra []string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if strings.EqualFold(parsed.Host, host) {
		return true
	}
	for _, allowed := range allowedExtra {
		if strings.EqualFold(parsed.Host, allowed) {
			return true
		}
	}
	return false
}

type WebsocketController struct {
	notifier *notify.Notifier
}

func NewWebsocketController(nf *notify.Notifier) *WebsocketController {
	return &WebsocketController{notifier: nf}
}

func (wc *WebsocketController) Connect(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	conn := notify.NewNotifyConnection(user.IdUser, wc.notifier, ws)
	wc.notifier.Add(conn)
	<-conn.Close
}
