package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerMCPHandler mounts the MCP HTTP handler (SSE + streamable transports)
// under /mcp on the shared engine. gin.WrapH forwards the unmodified
// *http.Request, so the handler's internal http.ServeMux still resolves its own
// exact-vs-subtree precedence (/mcp/http vs /mcp/).
//
// It must be registered BEFORE the global Logger/CORS middleware: MCP carries
// long-lived SSE streams that the request logger would only flush on close, and
// the CORS middleware's blanket OPTIONS short-circuit must not intercept MCP
// preflight.
func registerMCPHandler(engine *gin.Engine, handler http.Handler) {
	engine.Any("/mcp/*path", gin.WrapH(handler))
}
