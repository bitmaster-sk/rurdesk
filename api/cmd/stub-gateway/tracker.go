package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
)

func sendHeartbeats(ctx context.Context, target *configuredAgent, idTask int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = sendToTracker(ctx, target, http.MethodPost, fmt.Sprintf("/agent/task/%d/heartbeat", idTask), "")
		}
	}
}

func sendComplete(ctx context.Context, target *configuredAgent, idTask int64, body map[string]any) error {
	encoded, _ := json.Marshal(body)
	return sendToTracker(ctx, target, http.MethodPost, fmt.Sprintf("/agent/task/%d/complete", idTask), string(encoded))
}

// sendToTracker signs the body with the agent's secret and posts it to the tracker's REST API.
func sendToTracker(ctx context.Context, target *configuredAgent, method, path, body string) error {
	req, err := http.NewRequestWithContext(ctx, method, trackerURL+path, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", target.apiKey)
	timestamp := time.Now().Unix()
	req.Header.Set("X-Tracker-Signature", agent.SignPayload(target.secret, timestamp, []byte(body)))
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
