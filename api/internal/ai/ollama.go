package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaProvider calls a local Ollama instance via its native /api/chat
// endpoint. Structured output uses the `format` JSON schema parameter
// (grammar-constrained decoding) rather than tool calling: models such as
// qwen3-coder emit XML-style tool calls that Ollama fails to parse for nested
// schemas, whereas `format` forces valid JSON directly in the message content.
type OllamaProvider struct {
	host   string // e.g. "http://host.docker.internal:11434"
	model  string
	client *http.Client
}

// NewOllamaProvider creates a new OllamaProvider. host is the Ollama server's
// base URL; pass "" for the default (http://host.docker.internal:11434).
func NewOllamaProvider(host, model string) *OllamaProvider {
	if host == "" {
		host = "http://host.docker.internal:11434"
	}
	return &OllamaProvider{
		host:  host,
		model: model,
		// Local inference can be slow; use a generous timeout.
		client: &http.Client{Timeout: 300 * time.Second},
	}
}

// ollamaOptions holds generation options for the native /api/chat endpoint.
type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

// ollamaRequest is the request body for the Ollama native /api/chat endpoint.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []Message       `json:"messages"`
	Format   json.RawMessage `json:"format,omitempty"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options"`
}

// ollamaResponseMessage is the assistant message in a /api/chat response.
type ollamaResponseMessage struct {
	Content string `json:"content"`
}

// ollamaResponse is the response body from the Ollama native /api/chat endpoint.
// On failure Ollama returns a top-level string `error` field.
type ollamaResponse struct {
	Message    ollamaResponseMessage `json:"message"`
	DoneReason string                `json:"done_reason"`
	Error      string                `json:"error,omitempty"`
}

// Complete sends a request to Ollama using structured output (the `format`
// JSON schema) for reliable JSON generation.
func (p *OllamaProvider) Complete(ctx context.Context, req CompletionReq) (*CompletionRes, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Reinterpret the first tool's input schema as the structured-output format;
	// callers currently pass exactly one tool.
	var format json.RawMessage
	if len(req.Tools) > 0 {
		format = req.Tools[0].InputSchema
	}

	body := ollamaRequest{
		Model:    model,
		Messages: req.Messages,
		Format:   format,
		Stream:   false,
		Options:  ollamaOptions{NumPredict: req.MaxTokens},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	url := p.host + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("ollama: unmarshal response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("ollama: API error: %s", apiResp.Error)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: API error %d: %s", resp.StatusCode, string(respBytes))
	}

	content := apiResp.Message.Content
	if content == "" {
		return nil, fmt.Errorf("ollama: empty content in response (done_reason=%s)", apiResp.DoneReason)
	}

	var toolUseInput json.RawMessage
	if err := json.Unmarshal([]byte(content), &toolUseInput); err != nil {
		return nil, fmt.Errorf("ollama: unmarshal content as json: %w", err)
	}

	return &CompletionRes{
		ToolUseInput: toolUseInput,
		StopReason:   apiResp.DoneReason,
	}, nil
}
