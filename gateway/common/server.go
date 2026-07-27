package common

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type WebhookEvent struct {
	EventId string
	Type    string
	Payload map[string]any
}

type Server struct {
	cfg          *Config
	orchestrator *Orchestrator
	dedup        *DedupCache
}

func NewServer(cfg *Config, orchestrator *Orchestrator, dedup *DedupCache) *Server {
	return &Server{cfg: cfg, orchestrator: orchestrator, dedup: dedup}
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /event", s.handleEvent)
	mux.HandleFunc("GET /health", s.handleHealth)
	return mux
}

// shutdownGrace bounds how long Serve waits for in-flight webhook handlers after
// a stop signal. Kept well under Docker's default 10s SIGTERM→SIGKILL window so
// the process exits on its own terms rather than being killed.
const shutdownGrace = 5 * time.Second

// Serve listens until ctx is cancelled, then drains in-flight requests.
//
// main registers SIGINT/SIGTERM with signal.NotifyContext, which suppresses Go's
// default terminate-on-signal — so something has to act on the cancelled
// context, or the container only dies to SIGKILL with agents mid-stage.
func (s *Server) Serve(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.ListenPort)
	server := &http.Server{Addr: addr, Handler: s.buildMux()}

	serveErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", addr).Msg("gateway listening")
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("gateway server: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received, draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("gateway shutdown: %w", err)
		}
		log.Info().Msg("gateway stopped")
		return nil
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("health check")
	log.Debug().
		Int("count", s.orchestrator.ActiveTaskCount()).
		Interface("tasks", s.orchestrator.ActiveTasks()).
		Msg("gateway task queue")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"ok":true}`)
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	log.Debug().Str("method", r.Method).Str("url", r.URL.Path).Msg("event request received")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Tracker-Signature")
	if err := verifyHMAC(s.cfg.WebhookSecret, sig, body); err != nil {
		log.Warn().Err(err).Msg("webhook signature verification failed")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	eventID := r.Header.Get("X-Tracker-Event-Id")
	if eventID == "" {
		http.Error(w, "missing X-Tracker-Event-Id", http.StatusBadRequest)
		return
	}

	if s.dedup.IsProcessed(eventID) {
		log.Info().Str("eventId", eventID).Msg("duplicate event ignored")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Validated, not acted on — dedup is by event id. Catches a drifted sender.
	seq, err := strconv.ParseInt(r.Header.Get("X-Tracker-Sequence"), 10, 64)
	if err != nil {
		http.Error(w, "invalid X-Tracker-Sequence", http.StatusBadRequest)
		return
	}

	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
	}

	// Type normally rides the X-Tracker-Event-Type header; fall back to the
	// body's "event" field so a proxy stripping the custom header doesn't
	// silently turn every event into a no-op.
	eventType := r.Header.Get("X-Tracker-Event-Type")
	if eventType == "" {
		if bodyType, ok := payload["event"].(string); ok {
			eventType = bodyType
		}
	}
	log.Info().Str("eventId", eventID).Str("type", eventType).Int64("sequence", seq).Msg("event accepted")

	event := WebhookEvent{
		EventId: eventID,
		Type:    eventType,
		Payload: payload,
	}

	if err := s.orchestrator.HandleEvent(event); err != nil {
		log.Error().Err(err).Str("eventId", eventID).Msg("orchestrator failed to handle event")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.dedup.MarkProcessed(eventID)
	w.WriteHeader(http.StatusOK)
	log.Info().Str("eventId", eventID).Msg("event processed")
}

func verifyHMAC(secret []byte, signatureHeader string, body []byte) error {
	if signatureHeader == "" {
		return fmt.Errorf("missing X-Tracker-Signature")
	}

	var timestamp int64
	var receivedHex string
	for _, part := range strings.Split(signatureHeader, ",") {
		if strings.HasPrefix(part, "t=") {
			ts, err := strconv.ParseInt(strings.TrimPrefix(part, "t="), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid timestamp in signature")
			}
			timestamp = ts
		} else if strings.HasPrefix(part, "v1=") {
			receivedHex = strings.TrimPrefix(part, "v1=")
		}
	}

	if timestamp == 0 || receivedHex == "" {
		return fmt.Errorf("malformed signature header")
	}

	diff := time.Now().Unix() - timestamp
	if diff < -300 || diff > 300 {
		return fmt.Errorf("request timestamp outside 300s window")
	}

	payload := fmt.Sprintf("%d.%s", timestamp, body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	receivedBytes, err := hex.DecodeString(receivedHex)
	if err != nil {
		return fmt.Errorf("invalid hex in signature")
	}
	expectedBytes, _ := hex.DecodeString(expectedHex)

	if subtle.ConstantTimeCompare(receivedBytes, expectedBytes) != 1 {
		return fmt.Errorf("HMAC mismatch")
	}
	return nil
}
