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
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
)

var (
	trackerURL    string
	botApiKey     string
	webhookSecret []byte
)

func main() {
	// Mirror the real gateway's contract: a single TRACKER_URL base, with the
	// fixed /api/private REST path appended here.
	trackerURL = strings.TrimRight(mustEnv("TRACKER_URL"), "/") + "/api/private"
	botApiKey = mustEnv("GATEWAY_TO_TRACKER_TOKEN")
	secretHex := mustEnv("TRACKER_TO_GATEWAY_TOKEN")
	secret, err := hex.DecodeString(secretHex)
	if err != nil {
		log.Fatalf("invalid TRACKER_TO_GATEWAY_TOKEN: %v", err)
	}
	webhookSecret = secret

	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		port = "9090"
	}

	http.HandleFunc("/event", handleEvent)
	log.Printf("stub-gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	sig := r.Header.Get("X-Tracker-Signature")
	if err := agent.VerifySignature(webhookSecret, sig, body); err != nil {
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

	time.Sleep(2 * time.Second)

	body := stagedCannedBody(stage)
	if err := postComplete(ctx, idTask, body); err != nil {
		log.Printf("[stub-gw] complete error: %v", err)
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
	req.Header.Set("Authorization", botApiKey)
	ts := time.Now().Unix()
	sig := agent.SignPayload(webhookSecret, ts, []byte(body))
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
