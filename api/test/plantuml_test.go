package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/require"
)

// trap: the handler runs on the server's own goroutine, so hits/bodies/svg/failWith are shared — every access must take mu or -race reports even sequential reads.
type fakePlantUml struct {
	*httptest.Server
	mu       sync.Mutex
	hits     int
	bodies   []string
	svg      string
	failWith int
}

func newFakePlantUml(t *testing.T) *fakePlantUml {
	f := &fakePlantUml{svg: "<svg>sequence</svg>"}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.hits++
		f.bodies = append(f.bodies, string(body))
		failWith, svg := f.failWith, f.svg
		f.mu.Unlock()
		if r.URL.Path != "/svg" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if failWith != 0 {
			w.Header().Set("X-PlantUML-Diagram-Error", "syntax fail")
			w.WriteHeader(failWith)
			return
		}
		_, _ = w.Write([]byte(svg))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakePlantUml) fail(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failWith = status
}

func (f *fakePlantUml) setSvg(svg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.svg = svg
}

func (f *fakePlantUml) requests() (int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	bodies := append([]string(nil), f.bodies...)
	return f.hits, bodies
}

// trap: every test must set its own URL — the app-settings service singleton survives Setup(t), so a stale value from another test leaks in.
func setPlantUmlServerUrl(t *testing.T, app *issue.Application, adminToken, url string) {
	t.Helper()
	res := Request(t, app, http.MethodPatch, "/api/private/admin/settings",
		fmt.Sprintf(`{"plantUmlServerUrl":%q}`, url), adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
}

func renderPlantUml(t *testing.T, app *issue.Application, token, source string) *http.Response {
	t.Helper()
	return Request(t, app, http.MethodPost, "/api/private/plantuml/svg",
		fmt.Sprintf(`{"source":%q}`, source), token)
}

func TestPlantUml_DisabledWithoutServerUrl(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	f := newFakePlantUml(t)
	setPlantUmlServerUrl(t, app, adminToken, "")

	res := renderPlantUml(t, app, adminToken, "Alice -> Bob: hi")
	require.Equal(t, http.StatusNotFound, res.StatusCode, readBody(t, res))
	hits, _ := f.requests()
	require.Equal(t, 0, hits, "a disabled render must not call any server")
}

func TestPlantUml_RendersWrapsAndCaches(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	f := newFakePlantUml(t)
	setPlantUmlServerUrl(t, app, adminToken, f.URL)

	var rendered struct {
		Svg string `json:"svg"`
	}
	// readBody drains res.Body, so read once and decode from the string —
	// testify evaluates assertion messages eagerly.
	res := renderPlantUml(t, app, adminToken, "Alice -> Bob: hi")
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	require.Nil(t, json.Unmarshal([]byte(body), &rendered))
	require.Contains(t, rendered.Svg, "<svg>sequence</svg>")
	hits, bodies := f.requests()
	require.Equal(t, 1, hits)
	require.Equal(t, []string{"@startuml\nAlice -> Bob: hi\n@enduml"}, bodies, "a bare source is wrapped before it is sent")

	res = renderPlantUml(t, app, adminToken, "Alice -> Bob: hi")
	body = readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	require.Nil(t, json.Unmarshal([]byte(body), &rendered))
	require.Contains(t, rendered.Svg, "<svg>sequence</svg>")
	hits, _ = f.requests()
	require.Equal(t, 1, hits, "the second render must come from the Redis cache, not the server")
}

func TestPlantUml_SyntaxErrorMapsTo422(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	f := newFakePlantUml(t)
	f.fail(http.StatusBadRequest)
	setPlantUmlServerUrl(t, app, adminToken, f.URL)

	res := renderPlantUml(t, app, adminToken, "@startuml\nBob ->? Alice\n@enduml")
	body := readBody(t, res)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode, body)
	var syntaxErr struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.Nil(t, json.Unmarshal([]byte(body), &syntaxErr))
	require.Equal(t, "PLANTUML_SYNTAX", syntaxErr.Code)
	require.Equal(t, "syntax fail", syntaxErr.Message)
}

func TestPlantUml_UnavailableMapsTo502(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	f := newFakePlantUml(t)
	f.fail(http.StatusInternalServerError)
	setPlantUmlServerUrl(t, app, adminToken, f.URL)

	res := renderPlantUml(t, app, adminToken, "Alice -> Bob: hi")
	require.Equal(t, http.StatusBadGateway, res.StatusCode, readBody(t, res))

	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadUrl := dead.URL
	dead.Close()
	setPlantUmlServerUrl(t, app, adminToken, deadUrl)
	res = renderPlantUml(t, app, adminToken, "Alice -> Bob: bye")
	require.Equal(t, http.StatusBadGateway, res.StatusCode, readBody(t, res))
}

// trap: an svg above the 2 MB cap must 502 on the spot — a silently truncated
// copy would otherwise sit in Redis for 7 days and render as a broken diagram.
func TestPlantUml_OversizeSvgIsRejectedAndNotCached(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	f := newFakePlantUml(t)
	f.setSvg("<svg>" + strings.Repeat("x", 2<<20+10) + "</svg>")
	setPlantUmlServerUrl(t, app, adminToken, f.URL)

	res := renderPlantUml(t, app, adminToken, "Alice -> Bob: big")
	require.Equal(t, http.StatusBadGateway, res.StatusCode, readBody(t, res))

	res = renderPlantUml(t, app, adminToken, "Alice -> Bob: big")
	require.Equal(t, http.StatusBadGateway, res.StatusCode, readBody(t, res))
	hits, _ := f.requests()
	require.Equal(t, 2, hits, "an oversize svg must not be cached")
}

func TestPlantUml_RequestValidation(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	f := newFakePlantUml(t)
	setPlantUmlServerUrl(t, app, adminToken, f.URL)

	oversize := fmt.Sprintf(`{"source":%q}`, strings.Repeat("a", 64<<10+1))
	cases := []struct {
		name string
		body string
		want int
	}{
		{"blank source", `{"source":"   "}`, http.StatusBadRequest},
		{"missing source", `{}`, http.StatusBadRequest},
		{"oversize source", oversize, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Request(t, app, http.MethodPost, "/api/private/plantuml/svg", tc.body, adminToken)
			require.Equal(t, tc.want, res.StatusCode, readBody(t, res))
		})
	}
	hits, _ := f.requests()
	require.Equal(t, 0, hits, "a rejected body never reaches the configured server")
}

func TestPlantUml_SettingsAreAdminOnly(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	memberToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"plantuml member","email":"plantuml-member@test.sk","password":"kreslo"}`)

	res := Request(t, app, http.MethodPatch, "/api/private/admin/settings",
		`{"plantUmlServerUrl":"http://x:8080"}`, memberToken)
	require.Equal(t, http.StatusForbidden, res.StatusCode, readBody(t, res))

	setPlantUmlServerUrl(t, app, adminToken, "http://admin-sets.example:8080")

	res = Request(t, app, http.MethodGet, "/api/private/settings", "", adminToken)
	body := readBody(t, res)
	require.Equal(t, http.StatusOK, res.StatusCode, body)
	var settings struct {
		PlantUmlServerUrl string `json:"plantUmlServerUrl"`
	}
	require.Nil(t, json.Unmarshal([]byte(body), &settings))
	require.Equal(t, "http://admin-sets.example:8080", settings.PlantUmlServerUrl)
}
