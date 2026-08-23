package goose

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/rs/zerolog/log"
)

// scanGooseStream reads goose's stream-json stdout line-by-line and feeds each
// line to the aggregator. The buffer cap matches drainStderr so a long single
// event line doesn't overflow the default scanner limit.
func scanGooseStream(r io.Reader, agg *streamAggregator) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		agg.line(scanner.Bytes())
	}
}

// logStreamEvent writes one grouped stream event to the gateway log. Thinking
// goes to debug (only visible when LOG_LEVEL=debug); answer text and tool
// calls go to info so an info-level log still shows the run's skeleton.
func logStreamEvent(ev streamEvent, idRun int64) {
	switch ev.Kind {
	case "thinking":
		log.Debug().Int64("idRun", idRun).Str("phase", "thinking").Int("chars", ev.Chars).Str("text", ev.Text).Msg("goose stream")
	case "text":
		log.Info().Int64("idRun", idRun).Str("phase", "text").Int("chars", ev.Chars).Str("text", ev.Text).Msg("goose stream")
	case "tool":
		log.Info().Int64("idRun", idRun).Str("phase", "tool").Str("tool", ev.Tool).Msg("goose stream")
	}
}

// flushChars is the size at which a growing thinking/text buffer flushes to a
// log line, keeping a long phase readable instead of one giant blob or one
// line per token. Chosen for readability; no data is lost either way.
const flushChars = 200

// streamEvent is one grouped, human-facing unit the aggregator emits: a
// coalesced run of thinking/answer text, or a discrete tool call. The
// adapter's emit callback turns these into zerolog lines; tests collect them
// to assert the grouping.
type streamEvent struct {
	Kind  string // "thinking" | "text" | "tool"
	Text  string // coalesced text for thinking/text kinds
	Chars int    // len(Text), for the log
	Tool  string // tool name for the "tool" kind
}

// streamResult holds the run outcome (status, tokens, toolCalls, provErr)
// built incrementally while scanning the stream. stream-json sends no
// terminal status field, so status stays "" — treated as success unless the
// exit code says otherwise.
type streamResult struct {
	status              string
	tokens              int
	toolCalls           int
	provErr             *common.AgentError
	turnLimitHit        bool
	completeStageCalled bool
}

// streamAggregator consumes goose `--output-format stream-json` line by line:
// coalesces thinking/answer fragments into grouped events (emitted live via
// emit), counts tool calls, captures the token total from the terminal
// "complete" event, and accumulates answer text so finish() can classify a
// provider error (goose reports these as exit-0 text — block/goose#4612).
type streamAggregator struct {
	idRun int64
	emit  func(streamEvent)

	phase string          // current coalescing phase: "thinking" | "text" | ""
	buf   strings.Builder // pending text for the current phase

	answer  strings.Builder // all "text" fragments, for provider-error detection
	tokens  int
	toolCnt int

	// sawCompleteStage tracks whether the agent invoked `complete_stage`.
	// Submission rides entirely on that call; without it the adapter must fail
	// the stage rather than report a silent success that hangs the run.
	sawCompleteStage bool

	// turnLimitHit is set when the agent's answer text contains goose's
	// turn-limit sentinel ("reached the maximum number of actions").
	turnLimitHit bool
}

func newStreamAggregator(idRun int64, emit func(streamEvent)) *streamAggregator {
	return &streamAggregator{idRun: idRun, emit: emit}
}

// streamLine is the decoded shape of one stream-json line we care about.
// Fields not present on a given line stay zero — a "message" line fills
// Message, a "complete" line fills the token counts.
type streamLine struct {
	Type    string `json:"type"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
			ToolCall json.RawMessage `json:"toolCall"`
		} `json:"content"`
	} `json:"message"`
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// line feeds one raw stream-json line to the aggregator. Non-JSON banner lines
// and unknown event types are ignored.
func (a *streamAggregator) line(raw []byte) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return // goose banner / decoration
	}
	var sl streamLine
	if err := json.Unmarshal(raw, &sl); err != nil {
		return
	}
	switch sl.Type {
	case "message":
		a.handleMessage(sl)
	case "complete":
		a.tokens = tokensFromMap(map[string]any{
			"total_tokens":  float64(sl.TotalTokens),
			"input_tokens":  float64(sl.InputTokens),
			"output_tokens": float64(sl.OutputTokens),
		})
	}
}

func (a *streamAggregator) handleMessage(sl streamLine) {
	for _, block := range sl.Message.Content {
		switch {
		case block.Type == "thinking":
			a.append("thinking", block.Thinking)
		case block.Type == "text":
			a.answer.WriteString(block.Text)
			a.append("text", block.Text)
			if isTurnLimitSentinel(block.Text) {
				a.turnLimitHit = true
			}
		case isToolRequestType(block.Type):
			a.flush() // close the current text/thinking group first
			a.toolCnt++
			name := toolNameFromRaw(block.ToolCall)
			// Tracker exposes the tool namespaced (e.g. `tracker__complete_stage`),
			// so match on the suffix rather than an exact name.
			if strings.Contains(name, "complete_stage") {
				a.sawCompleteStage = true
			}
			a.emit(streamEvent{Kind: "tool", Tool: name})
		}
	}
}

// append adds a fragment to the current phase buffer, flushing first on a
// phase change and again once the buffer grows past flushChars.
func (a *streamAggregator) append(phase, frag string) {
	if frag == "" {
		return
	}
	if a.phase != phase {
		a.flush()
		a.phase = phase
	}
	a.buf.WriteString(frag)
	if a.buf.Len() >= flushChars {
		a.flush()
	}
}

// flush emits the pending coalesced buffer (if any) as one event and resets it.
func (a *streamAggregator) flush() {
	if a.buf.Len() == 0 {
		return
	}
	text := a.buf.String()
	a.emit(streamEvent{Kind: a.phase, Text: text, Chars: len(text)})
	a.buf.Reset()
}

// finish flushes any trailing buffer and returns the accumulated result,
// classifying a provider error from the answer text if one is present.
func (a *streamAggregator) finish() streamResult {
	a.flush()
	res := streamResult{tokens: a.tokens, toolCalls: a.toolCnt, completeStageCalled: a.sawCompleteStage, turnLimitHit: a.turnLimitHit}
	if text := a.answer.String(); isGooseErrorText(text) {
		res.provErr = classifyProviderError(text)
	}
	return res
}

// toolNameFromRaw pulls the tool name out of a toolRequest's toolCall block
// ({"value":{"name":"write"}}), returning "" if the shape is unexpected.
func toolNameFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var tc struct {
		Value struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return ""
	}
	return tc.Value.Name
}

// isTurnLimitSentinel returns true when text contains goose's turn-limit
// message. Goose exits 0 on --max-turns, so this sentinel is the only signal
// that the cap was hit. Matching a stable substring, not the full sentence.
func isTurnLimitSentinel(text string) bool {
	return strings.Contains(strings.ToLower(text), "reached the maximum number of actions")
}
