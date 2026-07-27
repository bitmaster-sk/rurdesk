package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// redirectTransport rewrites all requests to the given test server base URL.
type redirectTransport struct {
	base string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(t.base, "http://")
	return http.DefaultTransport.RoundTrip(req2)
}

func newTestClient(serverURL string) *http.Client {
	return &http.Client{
		Transport: &redirectTransport{base: serverURL},
	}
}

func TestGeminiProvider_Complete_Success(t *testing.T) {
	// JSON mode: the model returns a text part containing a JSON string.
	responseJSON := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "{\"issues\": [{\"title\": \"Test issue\"}]}"}]
			},
			"finishReason": "STOP"
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-test")
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "create some issues"}},
		Tools: []Tool{{
			Name:        "create_backlog",
			Description: "creates issues",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}

	res, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(res.ToolUseInput, &got); err != nil {
		t.Fatalf("ToolUseInput is not valid JSON: %v", err)
	}

	issues, ok := got["issues"]
	if !ok {
		t.Fatalf("expected 'issues' key in ToolUseInput, got: %s", res.ToolUseInput)
	}

	issuesList, ok := issues.([]interface{})
	if !ok || len(issuesList) == 0 {
		t.Fatalf("expected non-empty issues array, got: %v", issues)
	}

	if res.StopReason != "STOP" {
		t.Errorf("expected StopReason 'STOP', got: %s", res.StopReason)
	}
}

func TestGeminiProvider_Complete_APIError(t *testing.T) {
	responseJSON := `{"error": {"code": 400, "message": "invalid", "status": "INVALID_ARGUMENT"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-test")
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []Tool{{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "gemini:") {
		t.Errorf("expected gemini error, got: %v", err)
	}
}

func TestGeminiProvider_Complete_NoText(t *testing.T) {
	// Candidate with no parts — simulates a blocked/empty response.
	responseJSON := `{
		"candidates": [{
			"content": {"parts": []},
			"finishReason": "SAFETY"
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-test")
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []Tool{{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "no text in response") {
		t.Errorf("expected 'no text in response' error, got: %v", err)
	}
}

func TestOpenAIProvider_Complete_Success(t *testing.T) {
	responseJSON := `{
		"choices": [{
			"message": {
				"tool_calls": [{
					"function": {
						"name": "create_backlog",
						"arguments": "{\"issues\":[{\"title\":\"Test issue\"}]}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("", "test-key", "gpt-test", 0)
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "create some issues"}},
		Tools: []Tool{{
			Name:        "create_backlog",
			Description: "creates issues",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}

	res, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(res.ToolUseInput, &got); err != nil {
		t.Fatalf("ToolUseInput is not valid JSON: %v", err)
	}

	issues, ok := got["issues"]
	if !ok {
		t.Fatalf("expected 'issues' key in ToolUseInput, got: %s", res.ToolUseInput)
	}

	issuesList, ok := issues.([]interface{})
	if !ok || len(issuesList) == 0 {
		t.Fatalf("expected non-empty issues array, got: %v", issues)
	}

	if res.StopReason != "tool_calls" {
		t.Errorf("expected StopReason 'tool_calls', got: %s", res.StopReason)
	}
}

func TestOpenAIProvider_Complete_APIError(t *testing.T) {
	responseJSON := `{"error": {"message": "invalid key", "type": "invalid_request_error"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("", "test-key", "gpt-test", 0)
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []Tool{{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("expected error containing 'API error', got: %v", err)
	}
}

func TestOpenAIProvider_Complete_NoToolCall(t *testing.T) {
	responseJSON := `{"choices": []}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("", "test-key", "gpt-test", 0)
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []Tool{{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "no tool call") {
		t.Errorf("expected error containing 'no tool call', got: %v", err)
	}
}

func TestOllamaProvider_Complete_Success(t *testing.T) {
	// Structured output: the model returns valid JSON in message.content.
	responseJSON := `{
		"message": {"role": "assistant", "content": "{\"issues\":[{\"title\":\"Test issue\"}]}"},
		"done": true,
		"done_reason": "stop"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewOllamaProvider("", "qwen-test")
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "create some issues"}},
		Tools: []Tool{{
			Name:        "create_backlog",
			Description: "creates issues",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}

	res, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(res.ToolUseInput, &got); err != nil {
		t.Fatalf("ToolUseInput is not valid JSON: %v", err)
	}

	issues, ok := got["issues"]
	if !ok {
		t.Fatalf("expected 'issues' key in ToolUseInput, got: %s", res.ToolUseInput)
	}

	issuesList, ok := issues.([]interface{})
	if !ok || len(issuesList) == 0 {
		t.Fatalf("expected non-empty issues array, got: %v", issues)
	}

	if res.StopReason != "stop" {
		t.Errorf("expected StopReason 'stop', got: %s", res.StopReason)
	}
}

func TestOllamaProvider_Complete_APIError(t *testing.T) {
	// Ollama native API returns a top-level string error field.
	responseJSON := `{"error": "model \"qwen-test\" not found"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewOllamaProvider("", "qwen-test")
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []Tool{{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("expected error containing 'API error', got: %v", err)
	}
}

func TestOllamaProvider_Complete_EmptyContent(t *testing.T) {
	responseJSON := `{"message": {"role": "assistant", "content": ""}, "done_reason": "length"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer srv.Close()

	p := NewOllamaProvider("", "qwen-test")
	p.client = newTestClient(srv.URL)

	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []Tool{{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}},
	}

	_, err := p.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "empty content") {
		t.Errorf("expected error containing 'empty content', got: %v", err)
	}
}

func TestOllamaProvider_SendsFormatSchema(t *testing.T) {
	// Capture the outgoing request to assert it uses `format`, not `tools`.
	var captured map[string]json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &captured)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":{"content":"{}"},"done_reason":"stop"}`))
	}))
	defer srv.Close()

	p := NewOllamaProvider("", "qwen-test")
	p.client = newTestClient(srv.URL)

	schema := json.RawMessage(`{"type":"object","required":["score"]}`)
	req := CompletionReq{
		Messages: []Message{{Role: "user", Content: "evaluate"}},
		Tools:    []Tool{{Name: "quality_report", Description: "d", InputSchema: schema}},
	}

	if _, err := p.Complete(context.Background(), req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	formatRaw, ok := captured["format"]
	if !ok {
		t.Fatalf("request did not include a 'format' field; got keys: %v", captured)
	}
	if !strings.Contains(string(formatRaw), "score") {
		t.Errorf("expected format to carry the tool schema, got: %s", formatRaw)
	}
	if _, hasTools := captured["tools"]; hasTools {
		t.Errorf("request should not include 'tools', got: %s", captured["tools"])
	}
}
