package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
