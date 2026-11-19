package model

import "encoding/json"

// StreamLine is the decoded shape of one Claude Code stream-json line we read.
// Fields not present on a given line stay zero: an "assistant" line fills
// Message, a "stream_event" line fills Event, a "result" line fills Usage and
// TotalCostUsd. The embedded TokenUsage catches token counts a CLI version puts
// on the result event itself instead of under "usage".
type StreamLine struct {
	Type         string        `json:"type"`
	Subtype      string        `json:"subtype,omitempty"`
	IsError      bool          `json:"is_error,omitempty"`
	Message      *Message      `json:"message,omitempty"`
	Event        *PartialEvent `json:"event,omitempty"`
	Usage        *TokenUsage   `json:"usage,omitempty"`
	TotalCostUsd *float64      `json:"total_cost_usd,omitempty"`
	TokenUsage
}

type Message struct {
	Content []ContentBlock `json:"content,omitempty"`
	Usage   *TokenUsage    `json:"usage,omitempty"`
}

type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// ToolInput holds the input fields that identify a tool call.
type ToolInput struct {
	Command  string `json:"command"`
	FilePath string `json:"file_path"`
	Pattern  string `json:"pattern"`
}

// PartialEvent is the API stream event a "stream_event" line wraps.
type PartialEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type     string `json:"type"`
		Thinking string `json:"thinking"`
	} `json:"delta"`
}

type TokenUsage struct {
	TotalTokens              int `json:"total_tokens,omitempty"`
	InputTokens              int `json:"input_tokens,omitempty"`
	OutputTokens             int `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Total is TotalTokens when the CLI reports it, else the sum of the parts.
func (u TokenUsage) Total() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}
