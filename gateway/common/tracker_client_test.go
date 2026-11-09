package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTrackerClientSendsAgentKeyAsBearer(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		got = request.Header.Get("Authorization")
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	client := NewTrackerClient(&Config{TrackerAPIUrl: server.URL, WebhookSecret: []byte("secret"), AgentApiKey: "agent-key"})

	if err := client.SendTaskHeartbeat(context.Background(), 7); err != nil {
		t.Fatalf("SendTaskHeartbeat: %v", err)
	}
	if got != "Bearer agent-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer agent-key")
	}
}

func TestUpdateTaskStatsSendsAllUsageFields(t *testing.T) {
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	client := NewTrackerClient(&Config{TrackerAPIUrl: server.URL, WebhookSecret: []byte("secret"), AgentApiKey: "agent-key"})

	cost := 0.42
	err := client.UpdateTaskStats(context.Background(), 7, RunStats{
		TokensUsed: 30, DurationMs: 1200, ToolCallsCount: 2,
		InputTokens: 12, OutputTokens: 3456, CacheReadTokens: 150000, CacheWriteTokens: 20000,
		CostUsd: &cost, CostEstimated: true,
	})
	if err != nil {
		t.Fatalf("UpdateTaskStats: %v", err)
	}
	if path != "/agent/task/7/stats" {
		t.Errorf("path = %q, want /agent/task/7/stats", path)
	}
	want := map[string]any{
		"tokensUsed": 30.0, "durationMs": 1200.0, "toolCallsCount": 2.0,
		"inputTokens": 12.0, "outputTokens": 3456.0,
		"cacheReadTokens": 150000.0, "cacheWriteTokens": 20000.0,
		"costUsd": 0.42, "costEstimated": true,
	}
	for key, val := range want {
		if body[key] != val {
			t.Errorf("body[%q] = %v, want %v", key, body[key], val)
		}
	}

	body = nil
	err = client.UpdateTaskStats(context.Background(), 8, RunStats{TokensUsed: 5})
	if err != nil {
		t.Fatalf("UpdateTaskStats: %v", err)
	}
	if raw, ok := body["costUsd"]; !ok || raw != nil {
		t.Errorf("body[costUsd] = %v (present=%v), want explicit null", raw, ok)
	}
	if body["costEstimated"] != false {
		t.Errorf("body[costEstimated] = %v, want false", body["costEstimated"])
	}
}

func TestReportRecoveredSendsTheModelCatalog(t *testing.T) {
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	catalog, err := parseModelCatalog("goose", "openai/a,ollama/b:8b", "ollama/b:8b")
	if err != nil {
		t.Fatalf("parseModelCatalog: %v", err)
	}
	client := NewTrackerClient(&Config{TrackerAPIUrl: server.URL, WebhookSecret: []byte("secret"), Models: catalog, MaxTurnsLimit: 600})

	if err := client.ReportRecovered(context.Background()); err != nil {
		t.Fatalf("ReportRecovered: %v", err)
	}
	if path != "/agent/gateway/recovered" {
		t.Errorf("path = %q", path)
	}
	models, _ := body["models"].([]any)
	if len(models) != 2 || models[0] != "openai/a" || models[1] != "ollama/b:8b" {
		t.Errorf("models = %v", body["models"])
	}
	if body["modelDefault"] != "ollama/b:8b" {
		t.Errorf("modelDefault = %v", body["modelDefault"])
	}
	if body["maxTurnsLimit"] != float64(600) {
		t.Errorf("maxTurnsLimit = %v", body["maxTurnsLimit"])
	}
}

func TestReportModelThinking(t *testing.T) {
	var path string
	var body map[string]any
	tracker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		_ = json.NewDecoder(request.Body).Decode(&body)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(tracker.Close)
	client := NewTrackerClient(&Config{TrackerAPIUrl: tracker.URL, WebhookSecret: []byte("secret")})

	if err := client.ReportModelThinking(context.Background(), map[string]bool{"ollama/llama3.3": false}); err != nil {
		t.Fatalf("ReportModelThinking: %v", err)
	}
	if path != "/agent/gateway/thinking" {
		t.Errorf("path = %q", path)
	}
	if want := map[string]any{"ollama/llama3.3": false}; !reflect.DeepEqual(body["isThinkingByModel"], want) {
		t.Errorf("isThinkingByModel = %v, want %v", body["isThinkingByModel"], want)
	}
}
