// stub-gateway exercises the agent state machine without a real LLM.
// It receives stage_execute webhook events and replies via the
// complete_stage HTTP endpoint with canned outputs per stage.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

const thinkingBatchDelay = 700 * time.Millisecond

var (
	trackerURL string

	credentialsMu sync.RWMutex
	botApiKey     string
	webhookSecret []byte
)

func main() {
	// Mirror the real gateway's contract: a single TRACKER_URL base, with the
	// fixed /api/private REST path appended here.
	trackerURL = strings.TrimRight(mustEnv("TRACKER_URL"), "/") + "/api/private"
	if key, secretHex := os.Getenv("GATEWAY_TO_TRACKER_TOKEN"), os.Getenv("TRACKER_TO_GATEWAY_TOKEN"); key != "" || secretHex != "" {
		secret, err := hex.DecodeString(secretHex)
		if err != nil {
			log.Fatalf("invalid TRACKER_TO_GATEWAY_TOKEN: %v", err)
		}
		setCredentials(key, secret)
	}

	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		port = "9090"
	}

	http.HandleFunc("/event", handleEvent)
	http.HandleFunc("/configure", handleConfigure)
	log.Printf("stub-gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func credentials() (string, []byte) {
	credentialsMu.RLock()
	defer credentialsMu.RUnlock()
	return botApiKey, webhookSecret
}

func setCredentials(apiKey string, secret []byte) {
	credentialsMu.Lock()
	defer credentialsMu.Unlock()
	botApiKey = apiKey
	webhookSecret = secret
}

// handleConfigure takes the bot's two tokens after the caller has created the
// bot, for a stack where the bot does not exist yet when this process starts.
func handleConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "post only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		GatewayToTrackerToken string `json:"gatewayToTrackerToken"`
		TrackerToGatewayToken string `json:"trackerToGatewayToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	secret, err := hex.DecodeString(body.TrackerToGatewayToken)
	if err != nil || body.GatewayToTrackerToken == "" || len(secret) == 0 {
		http.Error(w, "both tokens are required, the tracker one as hex", http.StatusBadRequest)
		return
	}
	setCredentials(body.GatewayToTrackerToken, secret)
	log.Print("[stub-gw] credentials configured")
	w.WriteHeader(http.StatusNoContent)
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	_, secret := credentials()
	if len(secret) == 0 {
		http.Error(w, "gateway has no credentials yet, POST them to /configure", http.StatusServiceUnavailable)
		return
	}
	sig := r.Header.Get("X-Tracker-Signature")
	if err := agent.VerifySignature(secret, sig, body); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
		go executeStage(event.IdRun, event.Payload)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func executeStage(idRun int64, payload map[string]any) {
	idTaskF, _ := payload["idTask"].(float64)
	idTask := int64(idTaskF)
	stage, _ := payload["stage"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go heartbeat(ctx, idTask)

	streamThinking(ctx, idTask, stage)

	body := stagedCannedBody(stage)
	if err := postComplete(ctx, idTask, body); err != nil {
		log.Printf("[stub-gw] complete error: %v", err)
	}
}

// streamThinking sends the canned thinking of a stage the way a real gateway
// does: one batch at a time while the stage runs, each under the next seq.
func streamThinking(ctx context.Context, idTask int64, stage string) {
	batches := []model.AgentThinkingEvents{
		{{Kind: model.ThinkingKindThinking, Text: "Picking up the " + stage + " stage. "}},
		{{Kind: model.ThinkingKindTool, Tool: "developer__shell", Text: "rg --files src"}},
		{{Kind: model.ThinkingKindThinking, Text: "That is everything I need."}},
	}
	for index, events := range batches {
		time.Sleep(thinkingBatchDelay)
		for eventIndex := range events {
			events[eventIndex].At = time.Now().UnixMilli()
		}
		body, err := json.Marshal(model.AgentThinkingReq{Seq: index + 1, Events: events})
		if err != nil {
			log.Printf("[stub-gw] thinking encode error: %v", err)
			continue
		}
		path := fmt.Sprintf("/agent/task/%d/thinking", idTask)
		if err := postRaw(ctx, http.MethodPost, path, string(body)); err != nil {
			log.Printf("[stub-gw] thinking error: %v", err)
		}
	}
}

func stagedCannedBody(stage string) map[string]any {
	switch stage {
	case "pickup":
		return map[string]any{"outcome": "no_action_needed", "tokensUsed": 0, "durationMs": 100, "toolCallsCount": 0}
	case "brainstorming":
		return map[string]any{
			"outcome":        "no_action_needed",
			"message":        "No clarifications needed.",
			"messageKind":    "brainstorming_complete",
			"tokensUsed":     500,
			"durationMs":     1000,
			"toolCallsCount": 0,
		}
	case "design":
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub design proposal.",
			"messageKind":    "design",
			"tokensUsed":     2000,
			"durationMs":     3000,
			"toolCallsCount": 2,
		}
	case "implementation_plan":
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub implementation plan.",
			"messageKind":    "implementation_plan",
			"tokensUsed":     3000,
			"durationMs":     4000,
			"toolCallsCount": 3,
		}
	case "implementation":
		return map[string]any{
			"outcome":        "output_submitted",
			"message":        "Stub push to PR.",
			"messageKind":    "pull_request_pushed",
			"prUrl":          "https://github.com/example/repo/pull/1",
			"branchName":     "stub/agent-run",
			"tokensUsed":     8000,
			"durationMs":     12000,
			"toolCallsCount": 14,
		}
	default:
		return map[string]any{"outcome": "no_action_needed"}
	}
}

func heartbeat(ctx context.Context, idTask int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = postRaw(ctx, http.MethodPost, fmt.Sprintf("/agent/task/%d/heartbeat", idTask), "")
		}
	}
}

func postComplete(ctx context.Context, idTask int64, body map[string]any) error {
	b, _ := json.Marshal(body)
	return postRaw(ctx, http.MethodPost, fmt.Sprintf("/agent/task/%d/complete", idTask), string(b))
}

func postRaw(ctx context.Context, method, path, body string) error {
	req, err := http.NewRequestWithContext(ctx, method, trackerURL+path, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey, secret := credentials()
	req.Header.Set("Authorization", apiKey)
	ts := time.Now().Unix()
	sig := agent.SignPayload(secret, ts, []byte(body))
	req.Header.Set("X-Tracker-Signature", sig)
	req.Header.Set("X-Tracker-Event-Id", strconv.FormatInt(time.Now().UnixNano(), 36))
	req.Header.Set("X-Tracker-Sequence", "0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tracker returned %d for %s %s: %s", resp.StatusCode, method, path, string(buf))
	}
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}
