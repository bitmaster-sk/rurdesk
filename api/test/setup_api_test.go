package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/migrations"
	"github.com/go-redis/redis/v8"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RebuildWithProvider swaps the AI provider and rebuilds the application so the
// routes bind to controllers backed by it. Controllers capture the provider at
// build time (routes are bound to controller instances in issue.New()), so
// setting the provider alone never reaches the already-registered handlers.
// Clearing every AI-backed controller/service plus the router and http-server
// forces issue.New() to rewire them against the injected provider. Callers must
// reassign their suite's App to the returned instance.
func RebuildWithProvider(t *testing.T, p ai.Provider) *issue.Application {
	injector.Set("ai-provider", p)
	for _, key := range []string{
		"quality-service", "quality-controller",
		"split-service", "split-controller",
		"project-builder-controller",
		"router", "http-server",
	} {
		injector.Clear(key)
	}
	app, err := issue.New()
	require.Nil(t, err)
	return app
}

// TestMain gates the whole integration package: it refuses to run unless the
// test database is explicitly configured. The suite registers
// users, creates projects and writes rows — pointing it at the real `issue`
// database would corrupt production data. Requiring TEST_DATABASE_* (and a name
// that contains "test") makes that mistake impossible instead of relying on a
// silent default.
func TestMain(m *testing.M) {
	requireTestDatabaseEnv()
	requireTestCacheEnv()
	resetTestState()
	os.Exit(m.Run())
}

// requireTestCacheEnv gates the suite on an explicitly configured, non-zero
// Redis DB. Before this guard TEST_CACHE_DB silently defaulted to 0 — the dev
// cache — and the pre-run flush deleted every dev user's session (instant
// logout for anyone using the app while tests ran).
func requireTestCacheEnv() {
	if _, err := testCacheDBFromEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "\nintegration tests refuse to run: %v\n", err)
		os.Exit(1)
	}
}

// testCacheDBFromEnv returns the Redis DB index the suite may use. It must be
// set explicitly and be positive: DB 0 is the dev cache and must never be
// flushed or written to by tests.
func testCacheDBFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv("TEST_CACHE_DB"))
	if raw == "" {
		return 0, fmt.Errorf("TEST_CACHE_DB is not set; set it to a non-zero Redis DB (e.g. TEST_CACHE_DB=1) so tests can never touch the dev cache (DB 0)")
	}
	db, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("TEST_CACHE_DB=%q is not a number", raw)
	}
	if db <= 0 {
		return 0, fmt.Errorf("TEST_CACHE_DB must be positive, got %d — DB 0 is the dev cache, flushing it logs every dev user out", db)
	}
	return db, nil
}

// resetTestState brings the shared integration Postgres + Redis back to a known
// clean baseline BEFORE any test runs. The suite shares one database and one
// cache across runs; without a reset, state leaks between runs:
//   - leftover rows survive (per-suite cleanup DELETEs abort on foreign keys and
//     the error is ignored), so the next run's user/project creation collides
//     with HTTP 409;
//   - the 24h ACL role cache in Redis outlives the DB, so a stale
//     acl:project:<idProject>:<idUser>=owner entry grants access a freshly
//     created user must not have (403-expecting tests see 200).
//
// Postgres is reset with a full goose down (Reset) + up rather than a
// hand-maintained TRUNCATE list: it needs zero knowledge of the table set,
// re-seeds reference data automatically, drops orphaned per-project sequences,
// and exercises every Down migration on each run. The Redis test DB is then
// flushed — the goose reset restarts sequences (ids begin at 1 again), so a
// surviving cache entry would poison a brand-new entity with the same id.
//
// Gated by TEST_DB_RESET (default on). Set TEST_DB_RESET=false to skip it, e.g.
// to iterate against an already-clean DB without paying the reset cost.
// CI is unaffected either way — it uses fresh service containers per run.
func resetTestState() {
	if !testResetEnabled() {
		return
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("TEST_DATABASE_HOST"),
		envOrDefault("TEST_DATABASE_PORT", "5432"),
		envOrDefault("TEST_DATABASE_USER", "rurdesk"),
		envOrDefault("TEST_DATABASE_PASSWORD", "rurdesk"),
		os.Getenv("TEST_DATABASE_NAME"))

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "test reset: open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "test reset: set dialect: %v\n", err)
		os.Exit(1)
	}
	// Reset fails on a never-migrated database; this makes an empty one valid.
	if _, err := goose.EnsureDBVersion(db); err != nil {
		fmt.Fprintf(os.Stderr, "test reset: ensure goose version table: %v\n", err)
		os.Exit(1)
	}
	if err := goose.Reset(db, "."); err != nil {
		fmt.Fprintf(os.Stderr, "test reset: goose down: %v\n", err)
		os.Exit(1)
	}
	if err := goose.Up(db, "."); err != nil {
		fmt.Fprintf(os.Stderr, "test reset: goose up: %v\n", err)
		os.Exit(1)
	}

	flushTestCache()
}

// testResetEnabled reports whether the pre-run reset should happen. Default on;
// disabled only by an explicit falsey TEST_DB_RESET value.
func testResetEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TEST_DB_RESET"))) {
	case "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

// flushTestCache empties the test Redis DB. This MUST succeed: pairing a freshly
// reset DB (sequences restarted) with a live cache is the exact contamination
// resetTestState exists to prevent, so a failure aborts the run.
func flushTestCache() {
	host := os.Getenv("TEST_CACHE_HOST")
	if host == "" {
		fmt.Fprintln(os.Stderr, "test reset: TEST_CACHE_HOST is not set; cannot flush the cache")
		os.Exit(1)
	}

	cacheDB, err := testCacheDBFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "test reset: %v\n", err)
		os.Exit(1)
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, envOrDefaultInt("TEST_CACHE_PORT", 6379)),
		Password: envOrDefault("TEST_CACHE_PASSWORD", ""),
		DB:       cacheDB,
	})
	defer client.Close()

	if err := client.FlushDB(context.Background()).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "test reset: flush cache: %v\n", err)
		os.Exit(1)
	}
}

func requireTestDatabaseEnv() {
	name := strings.TrimSpace(os.Getenv("TEST_DATABASE_NAME"))
	host := strings.TrimSpace(os.Getenv("TEST_DATABASE_HOST"))

	var missing []string
	if name == "" {
		missing = append(missing, "TEST_DATABASE_NAME")
	}
	if host == "" {
		missing = append(missing, "TEST_DATABASE_HOST")
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr,
			"\nintegration tests refuse to run: missing %s.\n"+
				"Set the TEST_DATABASE_* env so tests can never touch the real database, e.g.:\n"+
				"  TEST_DATABASE_HOST=rurdesk-db TEST_DATABASE_NAME=rurdesk_test \\\n"+
				"  TEST_DATABASE_USER=rurdesk TEST_DATABASE_PASSWORD=rurdesk TEST_CACHE_HOST=issue-cache\n\n",
			strings.Join(missing, ", "))
		os.Exit(1)
	}

	if !strings.Contains(strings.ToLower(name), "test") {
		fmt.Fprintf(os.Stderr,
			"\nintegration tests refuse to run: TEST_DATABASE_NAME=%q does not look like a test database "+
				"(its name must contain \"test\").\n"+
				"This guard prevents the destructive integration suite from running against the real database.\n\n",
			name)
		os.Exit(1)
	}
}

func Setup(t *testing.T) *issue.Application {
	injector.ClearAll()

	// TEST_DATABASE_HOST / TEST_DATABASE_NAME are guaranteed non-empty by
	// requireTestDatabaseEnv (TestMain) — read them directly, with NO fallback,
	// so a missing value can never silently resolve to the real database.
	viper.Set("DATABASE_HOST", os.Getenv("TEST_DATABASE_HOST"))
	viper.Set("DATABASE_NAME", os.Getenv("TEST_DATABASE_NAME"))
	viper.Set("DATABASE_USER", envOrDefault("TEST_DATABASE_USER", "rurdesk"))
	viper.Set("DATABASE_PASSWORD", envOrDefault("TEST_DATABASE_PASSWORD", "rurdesk"))
	viper.Set("CACHE_HOST", envOrDefault("TEST_CACHE_HOST", "cache"))
	viper.Set("CACHE_PASSWORD", envOrDefault("TEST_CACHE_PASSWORD", ""))
	// Validated by requireTestCacheEnv (TestMain) — never defaults to the dev
	// cache (DB 0).
	testCacheDB, err := testCacheDBFromEnv()
	require.Nil(t, err)
	viper.Set("CACHE_DB", testCacheDB)
	viper.Set("CACHE_PORT", envOrDefaultInt("TEST_CACHE_PORT", 6379))

	// Production sets this; without it viper.GetInt returns 0 and the project
	// builder's "description too long" guard rejects every non-empty description.
	viper.Set("PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH", 10000)

	// The AI features no longer fall back to a hardcoded model — with no model
	// configured they return errs.ErrAiNotConfigured (503) before the injected
	// mock provider is ever reached. Deployments set this; the suite pins the
	// former default so the mock provider stays the thing under test.
	viper.Set("AI_MODEL", "claude-sonnet-4-5")

	app, err := issue.New()
	require.Nil(t, err)

	require.Nil(t, app.Pool.Ping(context.Background()), "postgres is unavailable for integration tests")

	cache, err := injector.GetCache()
	require.Nil(t, err)
	require.Nil(t, cache.Ping(context.Background()).Err(), "redis is unavailable for integration tests")

	var userTable *string
	err = app.Pool.QueryRow(context.Background(), "SELECT to_regclass('users.user')").Scan(&userTable)
	require.Nil(t, err)
	require.NotNil(t, userTable, "database schema is not initialized, run migrations first")

	seedUser(t, app)
	return app
}

func Request(t *testing.T, app *issue.Application, method, url, body, token string) *http.Response {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w.Result()
}

func RequestWithHeaders(t *testing.T, app *issue.Application, method, url, body string, headers map[string]string) *http.Response {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	return w.Result()
}

func Token(t *testing.T, app *issue.Application) string {
	rb := `{"email":"test@test.sk","password":"kreslo"}`
	res := Request(t, app, "POST", "/api/public/login", rb, "")
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var tk struct {
		Token string `json:"token"`
	}
	err := json.NewDecoder(res.Body).Decode(&tk)
	assert.Nil(t, err)
	assert.Greater(t, len(tk.Token), 0)
	return tk.Token
}

func seedUser(t *testing.T, app *issue.Application) {
	rb := `{"name":"tester","email":"test@test.sk","password":"kreslo"}`
	res := Request(t, app, "POST", "/api/public/register", rb, "")
	// 200 = bootstrapped this run; 409 = already exists (older behavior);
	// 403 = registration already closed and the bootstrap admin already exists.
	require.Contains(t, []int{http.StatusOK, http.StatusConflict, http.StatusForbidden}, res.StatusCode)
}

// createUserAsAdmin creates a human user via the admin endpoint and returns a login token
// for that user. body is the AdminCreateUserReq JSON (must include email+password).
func createUserAsAdmin(t *testing.T, app *issue.Application, adminToken, body string) string {
	res := Request(t, app, "POST", "/api/private/admin/user", body, adminToken)
	require.Contains(t, []int{http.StatusOK, http.StatusConflict}, res.StatusCode)

	var creds struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	require.Nil(t, json.Unmarshal([]byte(body), &creds))
	login := Request(t, app, "POST", "/api/public/login",
		`{"email":"`+creds.Email+`","password":"`+creds.Password+`"}`, "")
	require.Equal(t, http.StatusOK, login.StatusCode)
	var tk struct {
		Token string `json:"token"`
	}
	require.Nil(t, json.NewDecoder(login.Body).Decode(&tk))
	return tk.Token
}

func readBody(t *testing.T, res *http.Response) string {
	b, err := io.ReadAll(res.Body)
	require.Nil(t, err)
	return string(b)
}

// idOfUser looks up a user id by email via the admin list endpoint.
func idOfUser(t *testing.T, app *issue.Application, adminToken, email string) int64 {
	res := Request(t, app, "GET", "/api/private/admin/user", "", adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var users []struct {
		IdUser int64  `json:"idUser"`
		Email  string `json:"email"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&users))
	for _, u := range users {
		if u.Email == email {
			return u.IdUser
		}
	}
	t.Fatalf("user %s not found", email)
	return 0
}

// createProject creates a project owned by the token's user and returns its id.
func createProject(t *testing.T, app *issue.Application, token, name string) int64 {
	res := Request(t, app, "POST", "/api/private/project",
		`{"name":"`+name+`","color":"#123456"}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var prj model.Project
	require.Nil(t, json.NewDecoder(res.Body).Decode(&prj))
	return prj.IdProject
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envOrDefaultInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
