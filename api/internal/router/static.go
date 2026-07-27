package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// registerStaticServing serves the Angular production build as an SPA
// fallback: files that exist under staticDir are served as-is; any other
// non-API/non-MCP path returns index.html so client-side routing survives
// deep links and refreshes.
//
// Registered via NoRoute so it only fires when no real route matched — it
// must never shadow /api or /mcp, whose unmatched paths stay 404.
func registerStaticServing(engine *gin.Engine, staticDir string) {
	root := filepath.Clean(staticDir)
	indexPath := filepath.Join(root, "index.html")

	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/mcp") {
			c.Status(http.StatusNotFound)
			return
		}

		// Resolve the requested path against the static root, guarding against traversal.
		candidate := filepath.Join(root, filepath.Clean("/"+path))
		if candidate != root && !strings.HasPrefix(candidate, root+string(os.PathSeparator)) {
			c.Status(http.StatusNotFound)
			return
		}

		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			c.File(candidate)
			return
		}

		// SPA fallback: hand the client shell to the Angular router.
		c.File(indexPath)
	})
}
