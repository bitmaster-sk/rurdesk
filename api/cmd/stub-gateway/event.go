package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

// handleGatewayEvent receives the tracker's webhook and runs the addressed agent's stage in the background.
func handleGatewayEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	if agents.isEmpty() {
		http.Error(w, "gateway has no credentials yet, POST them to /configure", http.StatusServiceUnavailable)
		return
	}
	target := agents.bySignature(r.Header.Get("X-Tracker-Signature"), body)
	if target == nil {
		http.Error(w, "no configured agent signs this event", http.StatusUnauthorized)
		return
	}

	var event struct {
		IdRun   int64          `json:"idRun"`
		Event   string         `json:"event"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	log.Printf("[stub-gw] event=%s idRun=%d", event.Event, event.IdRun)

	if event.Event == "stage_execute" {
		go executeStage(target, event.IdRun, event.Payload)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func decodeHexSecret(value string) ([]byte, error) {
	secret, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(secret) == 0 {
		return nil, errors.New("empty secret")
	}
	return secret, nil
}
