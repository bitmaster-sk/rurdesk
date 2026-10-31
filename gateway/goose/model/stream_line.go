package model

import "encoding/json"

// StreamLine is the decoded shape of one goose stream-json line we care about.
// Fields not present on a given line stay zero — a "message" line fills
// Message, a "complete" line fills the token counts, an "error" line fills Error.
type StreamLine struct {
	Type    string `json:"type"`
	Message struct {
		Role    string         `json:"role"`
		Content []ContentBlock `json:"content"`
	} `json:"message"`
	Error        string `json:"error"`
	TotalTokens  int    `json:"total_tokens"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

type ContentBlock struct {
	Type             string          `json:"type"`
	Id               string          `json:"id"`
	Text             string          `json:"text"`
	Thinking         string          `json:"thinking"`
	ToolCall         json.RawMessage `json:"toolCall"`
	ToolResult       *ToolResult     `json:"toolResult"`
	Kind             string          `json:"kind"`
	Message          string          `json:"message"`
	NotificationType string          `json:"notificationType"`
	Msg              string          `json:"msg"`
}

type ToolResult struct {
	Status string `json:"status"`
	Value  struct {
		IsError bool `json:"isError"`
	} `json:"value"`
}
