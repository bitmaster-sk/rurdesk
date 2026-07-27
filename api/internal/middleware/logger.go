package middleware

import (
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func Logger(base zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		reqLogger := base.With().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.RawPath).
			Logger()
		ctx := extctx.WithLogger(c.Request.Context(), reqLogger)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)

		event := reqLogger.Info()
		if status >= 500 {
			event = reqLogger.Error()
		} else if status >= 400 {
			event = reqLogger.Warn()
		}

		logEvent := event.
			Int("status", status).
			Str("latency", latency.String()).
			Str("url", c.Request.URL.RequestURI())

		if mcpTool := c.GetHeader("X-MCP-Tool"); mcpTool != "" {
			logEvent = logEvent.Str("mcp_tool", mcpTool)
		}
		if mcpOrigin := c.GetHeader("X-MCP-Origin"); mcpOrigin != "" {
			logEvent = logEvent.Str("mcp_origin", mcpOrigin)
		}

		if len(c.Errors) > 0 {
			logEvent = logEvent.Str("errors", c.Errors.String())
		}

		logEvent.Msg("request")
	}
}
