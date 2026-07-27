package middleware

import (
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/gin-gonic/gin"
)

// AdminOnly must run after Auth. It rejects any caller whose session user is not
// a global instance admin. Bots are never admins, so this also blocks API-key callers.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := extctx.GetUser(c.Request.Context())
		if !ok || !user.IsAdmin {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
