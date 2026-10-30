package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// ApiKeyAuthenticator is the narrow interface required by the Auth middleware.
// Keeping it here (not importing the full service package) avoids an import
// cycle from middleware → service → repository → middleware via transitive
// DI dependencies, and follows the "accept interfaces at the point of use" rule.
type ApiKeyAuthenticator interface {
	LookupSession(ctx context.Context, rawKey string) (*model.ApiKeySession, error)
	CheckRateLimit(ctx context.Context, idApiKey int64, limitPerMin int) error
}

// SessionCache is the narrow slice of *redis.Client the Auth middleware needs
// to resolve an opaque, Redis-backed session token.
type SessionCache interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

// looksLikeApiKey reports whether bearer has the raw API key shape: 64
// lowercase hex chars. hex.EncodeToString only produces lowercase, so an
// uppercase bearer would fail the SHA256 lookup anyway — this check just
// skips that pointless DB roundtrip.
func looksLikeApiKey(bearer string) bool {
	if len(bearer) != 64 {
		return false
	}
	for _, r := range bearer {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// websocketProtocolMarker precedes the session token in the Sec-WebSocket-Protocol
// header. The browser WebSocket API cannot set request headers, so the SPA offers
// the token as a subprotocol value instead; the marker tells the two apart.
const websocketProtocolMarker = "Authorization"

// bearerFromWebsocketProtocol extracts the session token from a
// `Sec-WebSocket-Protocol: Authorization, <token>` handshake header. It returns
// "" for any other subprotocol offer, so unrelated protocols never authenticate.
func bearerFromWebsocketProtocol(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != websocketProtocolMarker {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func Auth(cache SessionCache, apiKeyAuth ApiKeyAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := extctx.BearerToken(c.GetHeader("Authorization"))
		if bearer == "" {
			bearer = bearerFromWebsocketProtocol(c.GetHeader("Sec-WebSocket-Protocol"))
		}
		if bearer == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Skip the Redis session lookup when the token has the shape of an API key.
		if !looksLikeApiKey(bearer) {
			var user model.User
			err := cache.Get(c.Request.Context(), bearer).Scan(&user)
			switch {
			case err == nil:
				// No isActive check here on purpose: snapshots minted before the
				// is_active column existed deserialize with IsActive=false and
				// would log every existing user out on deploy. A disable is
				// enforced by invalidating the user's sessions (UserService) —
				// Login refuses disabled accounts, so no isActive=false session
				// can be created after the deploy.
				ctx := extctx.WithUser(c.Request.Context(), user)
				c.Request = c.Request.WithContext(ctx)
				c.Next()
				return
			case !errors.Is(err, redis.Nil):
				// Cache infra failure, not an invalid token — 401 here would make
				// the client discard a still-valid session and force a re-login.
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		}

		// LookupSession returns nil, nil for an unknown/expired key; a non-nil
		// error means the lookup itself failed (cache/DB down).
		session, err := apiKeyAuth.LookupSession(c.Request.Context(), bearer)
		if err != nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if session == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		// A cached snapshot can predate a disable; reject it like an unknown key
		// (LookupSession already filters disabled users on the DB path and drops
		// stale entries on its cache path).
		if !session.User.IsActive {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if err := apiKeyAuth.CheckRateLimit(c.Request.Context(), session.IdApiKey, session.RateLimitPerMin); err != nil {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		ctx := extctx.WithApiKeyAuth(extctx.WithUser(c.Request.Context(), session.User))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
