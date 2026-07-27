package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/genai"
)

// GeminiProvider calls the Google Gemini API via the official genai SDK.
type GeminiProvider struct {
	apiKey string
	model  string
	client *http.Client // optional; nil uses SDK default
}

// NewGeminiProvider creates a new GeminiProvider.
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete calls Gemini in JSON mode: the first tool's InputSchema becomes the
// response schema, and the response text becomes ToolUseInput.
func (p *GeminiProvider) Complete(ctx context.Context, req CompletionReq) (*CompletionRes, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	sdkClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     p.apiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: p.client,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}

	contents := make([]*genai.Content, len(req.Messages))
	for i, m := range req.Messages {
		contents[i] = &genai.Content{
			Role:  m.Role,
			Parts: []*genai.Part{{Text: m.Content}},
		}
	}

	cfg := &genai.GenerateContentConfig{
		MaxOutputTokens: int32(req.MaxTokens),
	}
	if len(req.Tools) > 0 {
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseJsonSchema = req.Tools[0].InputSchema
	}

	res, err := sdkClient.Models.GenerateContent(ctx, model, contents, cfg)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}

	for _, candidate := range res.Candidates {
		if candidate.Content == nil {
			continue
		}
		if candidate.FinishReason == "MAX_TOKENS" {
			return nil, fmt.Errorf("gemini: response truncated by token limit (finish_reason=MAX_TOKENS)")
		}
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				return &CompletionRes{
					ToolUseInput: json.RawMessage(part.Text),
					StopReason:   string(candidate.FinishReason),
				}, nil
			}
		}
	}

	finishReason := ""
	if len(res.Candidates) > 0 {
		finishReason = string(res.Candidates[0].FinishReason)
	}
	return nil, fmt.Errorf("gemini: no text in response (finish_reason=%s)", finishReason)
}
