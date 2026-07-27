package mcp

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	mcpsdk "github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
)

type MCPServer struct {
	config     *Config
	engine     *gin.Engine
	dispatcher *Dispatcher
}

func NewMCPServer(config *Config, engine *gin.Engine) *MCPServer {
	return &MCPServer{
		config:     config,
		engine:     engine,
		dispatcher: NewDispatcher(engine),
	}
}

// Handler builds the MCP HTTP handler, exposing two tool sets under /mcp:
//   - /mcp/sse + /mcp/message → full tool set (implement stage; default).
//   - /mcp/plan/sse + /mcp/plan/message → read-only context tools plus
//     submit_plan / request_clarification (plan stage), no writes. This bounds
//     a misbehaving model's blast radius by tool surface, not prompt compliance.
//
// Must mount ahead of the global Logger/CORS middleware (see
// router.registerMCPHandler).
func (s *MCPServer) Handler() http.Handler {
	fullServer := buildMCPServer(s.dispatcher, StageAll)
	planServer := buildMCPServer(s.dispatcher, StagePlan)

	internalBase := s.config.internalBaseURL()
	fullSSE := mcpsdk.NewSSEServer(fullServer,
		mcpsdk.WithBaseURL(internalBase),
		mcpsdk.WithStaticBasePath("/mcp"),
	)
	planSSE := mcpsdk.NewSSEServer(planServer,
		mcpsdk.WithBaseURL(internalBase),
		mcpsdk.WithStaticBasePath("/mcp/plan"),
	)

	// Streamable HTTP transport (one endpoint per server): Goose rejects SSE
	// remote extensions, so the same tool sets are served here too. Calls stay
	// discrete JSON-RPC requests, so the tracker still records one message per
	// complete_stage/post_issue_message call, not per token.
	fullHTTP := mcpsdk.NewStreamableHTTPServer(fullServer,
		mcpsdk.WithEndpointPath("/mcp/http"),
	)
	planHTTP := mcpsdk.NewStreamableHTTPServer(planServer,
		mcpsdk.WithEndpointPath("/mcp/plan/http"),
	)

	// Exact patterns (the /http endpoints) win over the SSE subtree patterns
	// (/mcp/plan/, /mcp/), so streamable requests aren't swallowed by SSE.
	mux := http.NewServeMux()
	mux.Handle("/mcp/plan/http", planHTTP)
	mux.Handle("/mcp/http", fullHTTP)
	mux.Handle("/mcp/plan/", planSSE)
	mux.Handle("/mcp/", fullSSE)

	log.Info().
		Str("fullSsePath", fullSSE.CompleteSsePath()).
		Str("planSsePath", planSSE.CompleteSsePath()).
		Msg("MCP handler mounted")

	return &originRewriteHandler{
		inner:         mux,
		internalBase:  []byte(internalBase),
		publicBaseURL: s.config.PublicBaseURL,
	}
}

func buildMCPServer(dispatcher *Dispatcher, stage string) *mcpsdk.MCPServer {
	name := "Issue Tracker"
	if stage == StagePlan {
		name = "Issue Tracker (Plan)"
	}
	srv := mcpsdk.NewMCPServer(
		name,
		"1.0.0",
		mcpsdk.WithToolCapabilities(true),
	)
	registerProjectTools(srv, dispatcher, stage)
	registerMetaTools(srv, dispatcher, stage)
	registerIssueTools(srv, dispatcher, stage)
	registerRelationTools(srv, dispatcher, stage)
	registerMessageTools(srv, dispatcher, stage)
	registerTrackerTools(srv, dispatcher, stage)
	registerPlanTools(srv, dispatcher, stage)
	return srv
}

// originRewriteHandler rewrites the internal base URL in SSE responses to the
// client's actual origin, so MCP works behind any reverse proxy without
// explicit URL config.
type originRewriteHandler struct {
	inner         http.Handler
	internalBase  []byte
	publicBaseURL string
}

// clientOrigin returns the origin advertised to the MCP client.
//
// The endpoint URL built from it is where the client sends its Authorization
// bearer, so the host must never come from an attacker-controlled header:
// X-Forwarded-Host is deliberately NOT read. A direct request carrying a forged
// one would otherwise redirect the bot's token to that host. Every proxy we ship
// sets Host to the client-facing value (docker/proxy/templates/default.conf.template),
// so r.Host already carries what X-Forwarded-Host would have.
//
// X-Forwarded-Proto IS honoured: it can only flip the scheme on an origin we
// already control, and without it every URL behind a TLS-terminating proxy would
// be advertised as http://. Deployments that cannot preserve Host pin the origin
// with MCP_PUBLIC_BASE_URL instead, which bypasses request headers entirely.
func (h *originRewriteHandler) clientOrigin(r *http.Request) string {
	if h.publicBaseURL != "" {
		return h.publicBaseURL
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (h *originRewriteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientOrigin := []byte(h.clientOrigin(r))

	log.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("host", r.Host).
		Str("clientOrigin", string(clientOrigin)).
		Msg("MCP request")

	rw := &rewritingWriter{ResponseWriter: w, from: h.internalBase, to: clientOrigin, status: 200}
	if f, ok := w.(http.Flusher); ok {
		rw.flusher = f
	}
	h.inner.ServeHTTP(rw, r)

	if rw.status >= 400 {
		log.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Msg("MCP request failed")
	}
}

type rewritingWriter struct {
	http.ResponseWriter
	from    []byte
	to      []byte
	flusher http.Flusher
	status  int
}

func (w *rewritingWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *rewritingWriter) Write(p []byte) (int, error) {
	rewritten := bytes.ReplaceAll(p, w.from, w.to)
	n, err := w.ResponseWriter.Write(rewritten)
	if w.flusher != nil {
		w.flusher.Flush()
	}
	if err == nil {
		return len(p), nil
	}
	return n, err
}

func (w *rewritingWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}
