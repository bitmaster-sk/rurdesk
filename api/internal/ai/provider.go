package ai

import (
	"context"
	"encoding/json"
)

// Provider is the provider-agnostic interface for LLM completion.
type Provider interface {
	Complete(ctx context.Context, req CompletionReq) (*CompletionRes, error)
}

// Message is a single turn in the conversation.
type Message struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

// Tool describes a tool the LLM can call.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// CompletionReq is the provider-agnostic request.
type CompletionReq struct {
	Model     string
	Messages  []Message
	Tools     []Tool
	MaxTokens int
}

// CompletionRes is the provider-agnostic response.
type CompletionRes struct {
	// ToolUseInput is the raw JSON extracted from the tool_use block.
	ToolUseInput json.RawMessage
	// StopReason is the stop reason string from the provider.
	StopReason string
}
