package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildFakeMCPMux mirrors the real MCP server's http.ServeMux pattern layout:
// exact /mcp/http and /mcp/plan/http endpoints alongside the /mcp/ and
// /mcp/plan/ SSE subtrees. It lets the spike verify that gin.WrapH preserves
// the ServeMux's exact-vs-subtree precedence when mounted under a wildcard.
func buildFakeMCPMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/plan/http", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("plan-http"))
	})
	mux.HandleFunc("/mcp/http", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("full-http"))
	})
	mux.HandleFunc("/mcp/plan/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("plan-sse"))
	})
	mux.HandleFunc("/mcp/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("full-sse"))
	})
	return mux
}

// buildMountedEngine reproduces the production mounting order: MCP wildcard
// route registered, an /api route, and the SPA NoRoute fallback all on one
// engine — the exact combination the review flagged as a potential gin
// route-tree panic.
func buildMountedEngine(t *testing.T) *gin.Engine {
	t.Helper()
	engine := gin.New()

	require.NotPanics(t, func() {
		registerMCPHandler(engine, buildFakeMCPMux())
		engine.GET("/api/issue/:id", func(c *gin.Context) {
			c.String(http.StatusOK, "api-issue")
		})
		engine.NoRoute(func(c *gin.Context) {
			c.String(http.StatusOK, "spa-shell")
		})
	}, "registering /mcp wildcard + /api + NoRoute must not panic")

	return engine
}

func TestMCPMountPreservesExactVsSubtreePrecedence(t *testing.T) {
	engine := buildMountedEngine(t)

	cases := []struct{ path, want string }{
		{"/mcp/http", "full-http"},
		{"/mcp/plan/http", "plan-http"},
		{"/mcp/sse", "full-sse"},
		{"/mcp/plan/sse", "plan-sse"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		assert.Equal(t, tc.want, w.Body.String(), "path %s", tc.path)
	}
}

func TestMCPMountDoesNotShadowApiOrSpa(t *testing.T) {
	engine := buildMountedEngine(t)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/issue/7", nil))
	assert.Equal(t, "api-issue", w.Body.String())

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/projects/1", nil))
	assert.Equal(t, "spa-shell", w.Body.String())
}
