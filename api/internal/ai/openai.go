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

const openAIDefaultHost = "https://api.openai.com"

// OpenAIProvider calls the OpenAI Chat Completions API using raw net/http.
type OpenAIProvider struct {
	host   string
	apiKey string
	model  string
	client *http.Client
}

// openAIDefaultTimeout applies when timeout <= 0. Cloud and large local
// models can take minutes to return, so it's generous.
const openAIDefaultTimeout = 300 * time.Second

// NewOpenAIProvider creates a new OpenAIProvider.
// host overrides the base URL (e.g. for proxies); pass "" to use the default.
// timeout bounds the whole HTTP request; pass <= 0 to use openAIDefaultTimeout.
func NewOpenAIProvider(host, apiKey, model string, timeout time.Duration) *OpenAIProvider {
	if host == "" {
		host = openAIDefaultHost
	}
	if timeout <= 0 {
		timeout = openAIDefaultTimeout
	}
	return &OpenAIProvider{
		host:   host,
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: timeout},
	}
}

// openAIFunction is the function definition nested inside an openAITool.
type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// openAITool is the OpenAI-specific tool definition shape.
type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

// openAIRequest is the request body for the OpenAI Chat Completions API.
type openAIRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	Tools     []openAITool `json:"tools"`
	Messages  []Message    `json:"messages"`
}

// openAIToolCallFunction holds the name and arguments of a tool call.
type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAIToolCall is a single tool call returned by the model.
type openAIToolCall struct {
	Function openAIToolCallFunction `json:"function"`
}

// openAIMessage is the assistant message inside a choice.
type openAIMessage struct {
	ToolCalls []openAIToolCall `json:"tool_calls"`
}

// openAIChoice is one element in the choices array of the response.
type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// openAIResponse is the response body from the OpenAI Chat Completions API.
type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Complete sends a request to the OpenAI Chat Completions API and returns the tool call input.
func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionReq) (*CompletionRes, error) {
	tools := make([]openAITool, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}

	model := req.Model
	if model == "" {
		model = p.model
	}

	body := openAIRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		Tools:     tools,
		Messages:  req.Messages,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.host+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("openai: API error: %s", apiResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: API error %d", resp.StatusCode)
	}

	if len(apiResp.Choices) == 0 || len(apiResp.Choices[0].Message.ToolCalls) == 0 {
		finishReason := ""
		if len(apiResp.Choices) > 0 {
			finishReason = apiResp.Choices[0].FinishReason
		}
		return nil, fmt.Errorf("openai: no tool call in response (finish_reason=%s)", finishReason)
	}

	choice := apiResp.Choices[0]
	argumentsStr := choice.Message.ToolCalls[0].Function.Arguments

	var toolUseInput json.RawMessage
	if err := json.Unmarshal([]byte(argumentsStr), &toolUseInput); err != nil {
		return nil, fmt.Errorf("openai: unmarshal tool arguments: %w", err)
	}

	return &CompletionRes{
		ToolUseInput: toolUseInput,
		StopReason:   choice.FinishReason,
	}, nil
}
