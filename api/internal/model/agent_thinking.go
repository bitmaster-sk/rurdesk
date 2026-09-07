package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"strings"
)

const (
	ThinkingKindThinking  = "thinking"
	ThinkingKindTool      = "tool"
	ThinkingKindTruncated = "truncated"
)

// AgentThinkingEvent is one entry of an agent's thinking stream: either a piece
// of reasoning text, or a tool call with its name in Tool and its argument in
// Text.
type AgentThinkingEvent struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	Tool string `json:"tool,omitempty"`
	At   int64  `json:"at"`
}

func (e AgentThinkingEvent) IsEmpty() bool {
	return strings.TrimSpace(e.Tool) == "" && strings.TrimSpace(e.Text) == ""
}

func (e AgentThinkingEvent) Size() int {
	return len(e.Tool) + len(e.Text)
}

type AgentThinkingEvents []AgentThinkingEvent

// Accepted drops empty events and every kind other than thinking and tool,
// which leaves out `truncated` — the API's own marker, not something a gateway
// may forge.
func (e AgentThinkingEvents) Accepted() AgentThinkingEvents {
	accepted := make(AgentThinkingEvents, 0, len(e))
	for _, event := range e {
		if event.Kind != ThinkingKindThinking && event.Kind != ThinkingKindTool {
			continue
		}
		if event.IsEmpty() {
			continue
		}
		accepted = append(accepted, event)
	}
	return accepted
}

func (e AgentThinkingEvents) Gzip() ([]byte, error) {
	encoded, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(encoded); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *AgentThinkingEvents) Gunzip(blob []byte) error {
	reader, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return err
	}
	defer reader.Close()
	encoded, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, e)
}

type AgentThinkingReq struct {
	Seq    int                 `json:"seq" binding:"required"`
	Events AgentThinkingEvents `json:"events" binding:"required"`
}

type AgentThinkingRes struct {
	IdRun      int64               `json:"idRun"`
	IdTask     int64               `json:"idTask,omitempty"`
	Stage      string              `json:"stage"`
	Events     AgentThinkingEvents `json:"events"`
	LastSeq    int                 `json:"lastSeq,omitempty"`
	IsComplete bool                `json:"isComplete"`
}

type AgentThinkingNotice struct {
	IdRun  int64               `json:"idRun"`
	IdTask int64               `json:"idTask"`
	Stage  string              `json:"stage"`
	Seq    int                 `json:"seq"`
	Events AgentThinkingEvents `json:"events"`
}
