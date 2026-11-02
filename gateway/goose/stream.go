package goose

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/bitmaster-sk/rurdesk/gateway/goose/entity"
	"github.com/bitmaster-sk/rurdesk/gateway/goose/model"
	"github.com/bitmaster-sk/rurdesk/gateway/goose/toolcall"
	"github.com/rs/zerolog/log"
)

// scanGooseStream reads goose's stream-json stdout line-by-line and feeds each
// line to the aggregator.
func scanGooseStream(r io.Reader, maxLineBytes int, idRun int64, agg *streamAggregator) {
	skipped, err := common.ReadLines(r, maxLineBytes, agg.line)
	common.LogReadOutcome(idRun, "goose stdout", skipped, err)
}

// logStreamEvent writes one grouped stream event to the gateway log. Thinking
// goes to debug (only visible when LOG_LEVEL=debug); answer text and tool
// calls go to info so an info-level log still shows the run's skeleton.
func logStreamEvent(ev entity.StreamEvent, idRun int64) {
	switch ev.Kind {
	case "thinking":
		log.Debug().Int64("idRun", idRun).Str("phase", "thinking").Int("chars", ev.Chars).Str("text", ev.Text).Msg("goose stream")
	case "text":
		log.Info().Int64("idRun", idRun).Str("phase", "text").Int("chars", ev.Chars).Str("text", ev.Text).Msg("goose stream")
	case "tool":
		log.Info().Int64("idRun", idRun).Str("phase", "tool").Str("tool", ev.Tool).Str("detail", ev.ToolDetail).Msg("goose stream")
	}
}

// Answer text is skipped: it already becomes the stage's output message.
func queueThinkingEvent(ev entity.StreamEvent, batcher *common.ThinkingBatcher) {
	switch ev.Kind {
	case "thinking":
		batcher.Add(common.ThinkingEvent{Kind: common.ThinkingKindThinking, Text: ev.Text})
	case "tool":
		batcher.Add(common.ThinkingEvent{Kind: common.ThinkingKindTool, Tool: ev.Tool, Text: ev.ToolDetail})
	}
}

// flushChars is the size at which a growing thinking/text buffer flushes to a
// log line, keeping a long phase readable instead of one giant blob or one
// line per token. Chosen for readability; no data is lost either way.
const flushChars = 200

// streamAggregator consumes goose `--output-format stream-json` line by line:
// coalesces thinking/answer fragments into grouped events (emitted live via
// emit), counts tool calls, captures the token total from the terminal
// "complete" event, and records provider failures from goose's `error` and
// `systemNotification` content blocks.
type streamAggregator struct {
	idRun int64
	emit  func(entity.StreamEvent)

	phase string          // current coalescing phase: "thinking" | "text" | ""
	buf   strings.Builder // pending text for the current phase

	tokens  int
	toolCnt int

	// completeStageAccepted tracks whether a `complete_stage` call came back
	// without an error. Submission rides entirely on that call; without it the
	// adapter must fail the stage rather than report a silent success that
	// hangs the run.
	completeStageRequestIds map[string]bool
	completeStageAccepted   bool

	providerError   *common.AgentError
	runErrorMessage string

	// turnLimitHit is set when the agent's answer text contains goose's
	// turn-limit sentinel ("reached the maximum number of actions").
	turnLimitHit bool

	onCompleteStage func()
}

func newStreamAggregator(idRun int64, emit func(entity.StreamEvent)) *streamAggregator {
	return &streamAggregator{idRun: idRun, emit: emit, completeStageRequestIds: map[string]bool{}}
}

// line feeds one raw stream-json line to the aggregator. Non-JSON banner lines
// and unknown event types are ignored.
func (a *streamAggregator) line(raw []byte) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return // goose banner / decoration
	}
	var sl model.StreamLine
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
	case "error":
		a.runErrorMessage = sl.Error
	}
}

func (a *streamAggregator) handleMessage(sl model.StreamLine) {
	for _, block := range sl.Message.Content {
		switch {
		case block.Type == "thinking":
			a.append("thinking", block.Thinking)
		case block.Type == "text":
			a.append("text", block.Text)
			if isTurnLimitSentinel(block.Text) {
				a.turnLimitHit = true
			}
		case block.Type == "error":
			if a.providerError == nil {
				a.providerError = classifyProviderErrorKind(block.Kind, block.Message)
			}
		case block.Type == "systemNotification" && block.NotificationType == "creditsExhausted":
			if a.providerError == nil {
				a.providerError = &common.AgentError{Code: common.ErrCodeProviderCreditExhausted, Detail: firstLine(block.Msg)}
			}
		case block.Type == "toolResponse":
			if a.completeStageRequestIds[block.Id] && isSuccessfulToolResult(block.ToolResult) {
				a.completeStageAccepted = true
			}
		case toolcall.IsToolCall(block.Type):
			a.flush() // close the current text/thinking group first
			a.toolCnt++
			call := toolcall.Parse(block.ToolCall)
			// Tracker exposes the tool namespaced (e.g. `tracker__complete_stage`),
			// so match on the suffix rather than an exact name.
			if strings.Contains(call.Name, "complete_stage") {
				a.completeStageRequestIds[block.Id] = true
				if a.onCompleteStage != nil {
					a.onCompleteStage()
				}
			}
			a.emit(entity.StreamEvent{Kind: "tool", Tool: call.Name, ToolDetail: call.Detail})
		}
	}
}

func isSuccessfulToolResult(result *model.ToolResult) bool {
	return result != nil && result.Status == "success" && !result.Value.IsError
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
	a.emit(entity.StreamEvent{Kind: a.phase, Text: text, Chars: len(text)})
	a.buf.Reset()
}

// finish flushes any trailing buffer and returns the accumulated result.
func (a *streamAggregator) finish() entity.StreamResult {
	a.flush()
	return entity.StreamResult{
		Tokens:              a.tokens,
		ToolCalls:           a.toolCnt,
		ProviderError:       a.providerError,
		RunErrorMessage:     a.runErrorMessage,
		CompleteStageCalled: a.completeStageAccepted,
		TurnLimitHit:        a.turnLimitHit,
	}
}

// isTurnLimitSentinel returns true when text contains goose's turn-limit
// message. Goose exits 0 on --max-turns, so this sentinel is the only signal
// that the cap was hit. Matching a stable substring, not the full sentence.
func isTurnLimitSentinel(text string) bool {
	return strings.Contains(strings.ToLower(text), "reached the maximum number of actions")
}
