package common

import (
	"context"
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
