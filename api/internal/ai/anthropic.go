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

const anthropicDefaultHost = "https://api.anthropic.com"
const anthropicVersion = "2023-06-01"

// AnthropicProvider calls the Anthropic Messages API using raw net/http.
type AnthropicProvider struct {
	host   string
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicProvider creates a new AnthropicProvider.
// host overrides the base URL (e.g. for proxies); pass "" to use the default.
func NewAnthropicProvider(host, apiKey, model string) *AnthropicProvider {
	if host == "" {
		host = anthropicDefaultHost
	}
	return &AnthropicProvider{
		host:   host,
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// anthropicTool is the Anthropic-specific tool definition shape.
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicRequest is the request body for the Anthropic Messages API.
type anthropicRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Tools     []anthropicTool `json:"tools"`
	Messages  []Message       `json:"messages"`
}

// anthropicContentBlock represents one element in the content array of the response.
type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Input json.RawMessage `json:"input,omitempty"`
}

// anthropicResponse is the response body from the Anthropic Messages API.
type anthropicResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a request to the Anthropic Messages API and returns the tool_use input.
func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionReq) (*CompletionRes, error) {
	tools := make([]anthropicTool, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	body := anthropicRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		Tools:     tools,
		Messages:  req.Messages,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.host+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apiResp.Error != nil {
			return nil, fmt.Errorf("anthropic: API error %d: %s: %s", resp.StatusCode, apiResp.Error.Type, apiResp.Error.Message)
		}
		return nil, fmt.Errorf("anthropic: API error %d", resp.StatusCode)
	}

	for _, block := range apiResp.Content {
		if block.Type == "tool_use" {
			return &CompletionRes{
				ToolUseInput: block.Input,
				StopReason:   apiResp.StopReason,
			}, nil
		}
	}

	return nil, fmt.Errorf("anthropic: no tool_use block in response (stop_reason=%s)", apiResp.StopReason)
}
