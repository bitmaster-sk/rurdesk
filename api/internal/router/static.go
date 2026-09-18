package router

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// staticExts lists file extensions that are always static assets, never
// Angular routes. Requesting a missing file with one of these must produce a
// 404 — not the index.html SPA fallback — so the browser sees a proper error
// instead of an HTML body served with the wrong MIME type. This is what breaks
// dynamic import() of stale hashed chunks after a redeploy or tab duplicate.
var staticExts = map[string]struct{}{
	".js": {}, ".mjs": {}, ".css": {}, ".woff": {}, ".woff2": {},
	".ttf": {}, ".eot": {}, ".svg": {}, ".png": {}, ".jpg": {},
	".jpeg": {}, ".gif": {}, ".webp": {}, ".ico": {}, ".map": {},
	".json": {},
}

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
			setCacheHeaders(c, path)
			c.File(candidate)
			return
		}

		// A missing file with a static-file extension is never a client-side
		// route. Return 404 instead of SPA-fallback so the browser sees a
		// proper error rather than index.html served as text/html — which
		// breaks ES module loading for stale hashed chunks.
		if _, isStatic := staticExts[filepath.Ext(path)]; isStatic {
			c.Status(http.StatusNotFound)
			return
		}

		// SPA fallback: hand the client shell to the Angular router.
		setCacheHeaders(c, "/index.html")
		c.File(indexPath)
	})
}

// hashedAssetPattern matches the content-hash filenames the Angular build
// emits under outputHashing: "all" (main-7LTDLGCT.js, styles-QK3RZ2XA.css).
// Only those may be cached forever: everything copied verbatim from
// src/assets (translations, images, favicons) keeps its name across deploys,
// so caching it immutably would pin a stale copy in the browser for a year.
var hashedAssetPattern = regexp.MustCompile(`-[A-Z0-9]{8,}\.[a-zA-Z0-9]+$`)

// setCacheHeaders caches content-hashed assets for a year and makes the
// browser revalidate everything else — index.html above all, so it picks up
// new chunk hashes after a deploy.
func setCacheHeaders(c *gin.Context, path string) {
	if hashedAssetPattern.MatchString(filepath.Base(path)) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	c.Header("Cache-Control", "no-cache")
}
