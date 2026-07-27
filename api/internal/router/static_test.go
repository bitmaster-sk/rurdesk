package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newStaticEngine(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa-shell</html>"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644))

	engine := gin.New()
	registerStaticServing(engine, dir)
	return engine, dir
}

func doGet(engine *gin.Engine, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	engine.ServeHTTP(w, req)
	return w
}

func TestStaticServesIndexForUnknownSpaRoute(t *testing.T) {
	engine, _ := newStaticEngine(t)

	w := doGet(engine, "/projects/42/board")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "spa-shell")
}

func TestStaticServesExistingAssetDirectly(t *testing.T) {
	engine, _ := newStaticEngine(t)

	w := doGet(engine, "/assets/app.js")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "console.log(1)")
}

func TestStaticDoesNotSwallowApiNotFound(t *testing.T) {
	engine, _ := newStaticEngine(t)

	w := doGet(engine, "/api/does/not/exist")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "spa-shell")
}

func TestStaticDoesNotSwallowMcpNotFound(t *testing.T) {
	engine, _ := newStaticEngine(t)

	w := doGet(engine, "/mcp/does/not/exist")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "spa-shell")
}

func TestStaticRejectsPathTraversal(t *testing.T) {
	engine, dir := newStaticEngine(t)
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("top-secret"), 0o644))

	w := doGet(engine, "/../secret.txt")

	assert.NotContains(t, w.Body.String(), "top-secret")
}
