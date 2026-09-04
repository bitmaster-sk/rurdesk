package common

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// TrackerClient sends HMAC-signed requests to the tracker REST API on behalf
// of the gateway. Surface is intentionally small — per-task heartbeats and
// per-stage completion; the API drives queueing, scheduling, and crash
// recovery, and run-level transitions are not the gateway's concern.
type TrackerClient struct {
	cfg    *Config
	client *http.Client
}

func NewTrackerClient(cfg *Config) *TrackerClient {
	return &TrackerClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// CompleteStagePayload mirrors api/internal/model.CompleteStageReq.
type CompleteStagePayload struct {
	Outcome        string `json:"outcome"`
	Message        string `json:"message,omitempty"`
	MessageKind    string `json:"messageKind,omitempty"`
	PrUrl          string `json:"prUrl,omitempty"`
	BranchName     string `json:"branchName,omitempty"`
	TokensUsed     *int   `json:"tokensUsed,omitempty"`
	DurationMs     *int   `json:"durationMs,omitempty"`
	ToolCallsCount *int   `json:"toolCallsCount,omitempty"`
	ErrorReason    string `json:"errorReason,omitempty"`
	ErrorDetail    string `json:"errorDetail,omitempty"`
}

// CompleteStage posts the outcome of a stage attempt. Called by the
// orchestrator only on the error path; the success path goes through the
// agent's own `complete_stage` MCP tool call, which hits the same endpoint.
func (tc *TrackerClient) CompleteStage(ctx context.Context, idTask int64, payload CompleteStagePayload) error {
	return tc.post(ctx, fmt.Sprintf("/agent/task/%d/complete", idTask), payload)
}

// ReportRunRepo tells the tracker which repo (owner/repo) a run pushes to, so
// the API can resolve the matching git_integration onto the run before the
// agent reports complete_stage. Deliberately a reliable, non-LLM channel —
// repo identity must not flow through the agent. Best-effort: on failure the
// run stays unresolved and the complete path errors clearly.
func (tc *TrackerClient) ReportRunRepo(ctx context.Context, idRun int64, repoPath string) error {
	type body struct {
		RepoPath string `json:"repoPath"`
	}
	return tc.post(ctx, fmt.Sprintf("/agent/run/%d/repo", idRun), body{RepoPath: repoPath})
}

// SendTaskHeartbeat refreshes last_heartbeat_at on the agent_task row.
func (tc *TrackerClient) SendTaskHeartbeat(ctx context.Context, idTask int64) error {
	return tc.post(ctx, fmt.Sprintf("/agent/task/%d/heartbeat", idTask), nil)
}

// ReportRecovered tells the tracker this gateway just (re)started, so any
// tasks it was running are orphaned; the API fails them and their runs (the
// user sees Continue/Restart). Called once on startup; best-effort.
func (tc *TrackerClient) ReportRecovered(ctx context.Context) error {
	return tc.post(ctx, "/agent/gateway/recovered", nil)
}

// UpdateTaskStats overwrites tokens / duration / tool-call counters on the
// agent_task row with what the adapter measured — the agent rarely passes
// these via complete_stage, so without this call they stay zero.
func (tc *TrackerClient) UpdateTaskStats(ctx context.Context, idTask int64, stats RunStats) error {
	type body struct {
		TokensUsed     int `json:"tokensUsed"`
		DurationMs     int `json:"durationMs"`
		ToolCallsCount int `json:"toolCallsCount"`
	}
	return tc.post(ctx, fmt.Sprintf("/agent/task/%d/stats", idTask), body{
		TokensUsed:     stats.TokensUsed,
		DurationMs:     stats.DurationMs,
		ToolCallsCount: stats.ToolCallsCount,
	})
}

// SendThinking posts one batch of the task's thinking events to the tracker.
func (tc *TrackerClient) SendThinking(ctx context.Context, idTask int64, seq int, events []ThinkingEvent) error {
	type body struct {
		Seq    int             `json:"seq"`
		Events []ThinkingEvent `json:"events"`
	}
	return tc.post(ctx, fmt.Sprintf("/agent/task/%d/thinking", idTask), body{Seq: seq, Events: events})
}

func (tc *TrackerClient) post(ctx context.Context, path string, body any) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}
	}

	url := tc.cfg.TrackerAPIUrl + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	tc.signRequest(req, bodyBytes)

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(resp.Body)
		// The tracker renders a uniform { code, message, translateKey } error body;
		// surface the code so failures are diagnosable without the raw payload.
		var apiErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(responseBody, &apiErr)
		log.Warn().
			Str("path", path).
			Int("status", resp.StatusCode).
			Str("code", apiErr.Code).
			Str("body", string(responseBody)).
			Msg("tracker API error")
		if apiErr.Code != "" {
			return fmt.Errorf("POST %s: status %d (%s: %s)", path, resp.StatusCode, apiErr.Code, apiErr.Message)
		}
		return fmt.Errorf("POST %s: status %d", path, resp.StatusCode)
	}
	return nil
}

func (tc *TrackerClient) signRequest(req *http.Request, body []byte) {
	ts := time.Now().Unix()
	payload := fmt.Sprintf("%d.%s", ts, body)
	mac := hmac.New(sha256.New, tc.cfg.WebhookSecret)
	mac.Write([]byte(payload))
	sig := fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))

	req.Header.Set("Authorization", tc.cfg.BotApiKey)
	req.Header.Set("X-Tracker-Signature", sig)
	req.Header.Set("X-Tracker-Event-Id", fmt.Sprintf("%d", time.Now().UnixNano()))
}
