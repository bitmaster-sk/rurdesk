package goose

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
)

func TestOllamaThinkingByModel(t *testing.T) {
	server := ollamaStub(t, map[string]string{
		"qwen3:8b":    `{"capabilities":["completion","tools","thinking"]}`,
		"llama3.3":    `{"capabilities":["completion","tools"]}`,
		"old-ollama":  `{}`,
		"broken-json": `not json`,
	})
	models := []common.Model{
		{Provider: "ollama", Name: "qwen3:8b"},
		{Provider: "ollama", Name: "llama3.3"},
		{Provider: "ollama", Name: "old-ollama"},
		{Provider: "ollama", Name: "broken-json"},
		{Provider: "ollama", Name: "missing"},
		{Provider: "openai", Name: "gpt-4o"},
	}

	isThinkingByModel, unanswered := ollamaThinkingByModel(context.Background(), server.URL, models)

	if want := map[string]bool{"ollama/qwen3:8b": true, "ollama/llama3.3": false}; !reflect.DeepEqual(isThinkingByModel, want) {
		t.Errorf("thinking by model = %v, want %v", isThinkingByModel, want)
	}
	if want := []common.Model{{Provider: "ollama", Name: "broken-json"}, {Provider: "ollama", Name: "missing"}}; !reflect.DeepEqual(unanswered, want) {
		t.Errorf("unanswered = %v, want %v", unanswered, want)
	}
}

func TestOllamaThinkingByModelWhenOllamaIsDown(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	models := []common.Model{{Provider: "ollama", Name: "llama3.3"}}

	isThinkingByModel, unanswered := ollamaThinkingByModel(context.Background(), server.URL, models)

	if len(isThinkingByModel) != 0 {
		t.Errorf("an unreachable ollama must report nothing, got %v", isThinkingByModel)
	}
	if !reflect.DeepEqual(unanswered, models) {
		t.Errorf("unanswered = %v, want %v", unanswered, models)
	}
}

func ollamaStub(t *testing.T, capabilitiesByModel map[string]string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var show struct {
			Model string `json:"model"`
		}
		if request.URL.Path != "/api/show" || json.NewDecoder(request.Body).Decode(&show) != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		body, ok := capabilitiesByModel[show.Model]
		if !ok {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGooseAsksOllamaWhichModelsThink(t *testing.T) {
	server := ollamaStub(t, map[string]string{"llama3.3": `{"capabilities":["completion","tools"]}`})
	adapter := NewGooseAdapter(&common.Config{Goose: &common.GooseConfig{OllamaHost: server.URL}}, nil)

	isThinkingByModel, unanswered := adapter.ModelThinking(context.Background(), []common.Model{
		{Provider: "ollama", Name: "llama3.3"},
		{Provider: "openai", Name: "gpt-4o"},
	})

	if want := map[string]bool{"ollama/llama3.3": false}; !reflect.DeepEqual(isThinkingByModel, want) {
		t.Errorf("thinking by model = %v, want %v", isThinkingByModel, want)
	}
	if len(unanswered) != 0 {
		t.Errorf("unanswered = %v, want none", unanswered)
	}
}

func TestOllamaBaseUrl(t *testing.T) {
	for host, want := range map[string]string{
		"":                      "http://localhost:11434",
		"ollama:11434":          "http://ollama:11434",
		"https://ollama.local/": "https://ollama.local",
		"http://10.0.0.5:11434": "http://10.0.0.5:11434",
	} {
		if got := ollamaBaseUrl(host); got != want {
			t.Errorf("ollamaBaseUrl(%q) = %q, want %q", host, got, want)
		}
	}
}
