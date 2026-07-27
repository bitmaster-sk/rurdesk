package goose

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
)

// A real goose v1.38.0 stream-json capture (157 token events for a two-sentence
// answer) must collapse into a handful of grouped log events while preserving
// the token total from the terminal "complete" event and never panicking.
func TestStreamAggregator_RealCapture(t *testing.T) {
	f, err := os.Open("testdata/stream-json/kimi-answer.ndjson")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	emit, got := collectEvents()
	a := newStreamAggregator(4, emit)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lines := 0
	for sc.Scan() {
		lines++
		a.line(sc.Bytes())
	}
	res := a.finish()

	if lines < 100 {
		t.Fatalf("fixture only had %d lines, expected the full capture", lines)
	}
	if len(*got) >= lines {
		t.Errorf("no coalescing: %d events from %d lines", len(*got), lines)
	}
	if len(*got) == 0 {
		t.Fatal("no events emitted from real capture")
	}
	if res.tokens != 305 {
		t.Errorf("tokens = %d, want 305 (from complete event)", res.tokens)
	}
	if res.provErr != nil {
		t.Errorf("provErr = %v, want nil on a clean answer", res.provErr)
	}
	// The run went thinking→text; both phases must appear in the grouped output.
	var sawThinking, sawText bool
	for _, ev := range *got {
		sawThinking = sawThinking || ev.Kind == "thinking"
		sawText = sawText || ev.Kind == "text"
	}
	if !sawThinking || !sawText {
		t.Errorf("expected both thinking and text groups, got thinking=%v text=%v", sawThinking, sawText)
	}
}

// collectEvents returns an emit func plus a pointer to the slice it appends to,
// so a test can drive the aggregator and then assert on the grouped output.
func collectEvents() (func(streamEvent), *[]streamEvent) {
	var got []streamEvent
	return func(ev streamEvent) { got = append(got, ev) }, &got
}

// thinkingLine/textLine build one stream-json "message" event line carrying a
// single content block — mirrors what goose v1.38.0 emits per token.
func thinkingLine(frag string) string {
	return `{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":` + jsonStr(frag) + `,"signature":""}]}}`
}
func textLine(frag string) string {
	return `{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":` + jsonStr(frag) + `}]}}`
}

// jsonStr is a tiny JSON string encoder for test fragments (no control chars).
func jsonStr(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func feed(a *streamAggregator, lines ...string) {
	for _, l := range lines {
		a.line([]byte(l))
	}
}

// A run's thinking then answer must collapse from many token events into a
// small number of grouped events, split at the thinking→text boundary, with the
// text of each phase preserved verbatim.
func TestStreamAggregator_CoalescesThinkingThenText(t *testing.T) {
	emit, got := collectEvents()
	a := newStreamAggregator(1, emit)

	feed(a,
		thinkingLine("The"), thinkingLine(" user"), thinkingLine(" wants"),
		textLine("An"), textLine(" MD5"), textLine(" hash"),
	)
	res := a.finish()

	if len(*got) != 2 {
		t.Fatalf("expected 2 coalesced events, got %d: %+v", len(*got), *got)
	}
	if (*got)[0].Kind != "thinking" || (*got)[0].Text != "The user wants" {
		t.Errorf("event[0] = %+v, want thinking 'The user wants'", (*got)[0])
	}
	if (*got)[1].Kind != "text" || (*got)[1].Text != "An MD5 hash" {
		t.Errorf("event[1] = %+v, want text 'An MD5 hash'", (*got)[1])
	}
	if res.provErr != nil {
		t.Errorf("provErr = %v, want nil on clean run", res.provErr)
	}
}

// A long uninterrupted phase must be flushed in chunks (size threshold) rather
// than buffered into one giant unreadable line — but no fragment is lost.
func TestStreamAggregator_FlushesOnSizeThreshold(t *testing.T) {
	emit, got := collectEvents()
	a := newStreamAggregator(1, emit)

	// 20 fragments of 40 chars each = 800 chars, well over the flush threshold.
	frag := strings.Repeat("x", 40)
	want := ""
	for i := 0; i < 20; i++ {
		a.line([]byte(thinkingLine(frag)))
		want += frag
	}
	a.finish()

	if len(*got) < 2 {
		t.Fatalf("expected multiple flushes for 800 chars, got %d", len(*got))
	}
	joined := ""
	for _, ev := range *got {
		if ev.Kind != "thinking" {
			t.Errorf("unexpected event kind %q", ev.Kind)
		}
		if ev.Chars != len(ev.Text) {
			t.Errorf("Chars %d != len(Text) %d", ev.Chars, len(ev.Text))
		}
		joined += ev.Text
	}
	if joined != want {
		t.Errorf("reassembled text length %d, want %d — data lost across flushes", len(joined), len(want))
	}
}

// The goose banner (non-JSON preamble lines) and unknown event types must be
// ignored without emitting groups or panicking.
func TestStreamAggregator_SkipsBannerLines(t *testing.T) {
	emit, got := collectEvents()
	a := newStreamAggregator(1, emit)

	feed(a,
		`__( O)>  ● new session · openai kimi-k2.7-code`,
		`   \____)    20260705_4 · /tmp`,
		`     L L     goose is ready`,
		``,
		thinkingLine("hi"),
	)
	a.finish()

	if len(*got) != 1 || (*got)[0].Text != "hi" {
		t.Fatalf("banner not skipped cleanly, got %+v", *got)
	}
}

// A tool call in the stream is counted and surfaced as a discrete tool event
// carrying the tool name, and it forces the preceding text buffer to flush.
func TestStreamAggregator_CountsAndEmitsToolRequests(t *testing.T) {
	emit, got := collectEvents()
	a := newStreamAggregator(1, emit)

	feed(a,
		textLine("writing file"),
		`{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"call_1","toolCall":{"status":"success","value":{"name":"write"}}}]}}`,
	)
	res := a.finish()

	if res.toolCalls != 1 {
		t.Errorf("toolCalls = %d, want 1", res.toolCalls)
	}
	var tool *streamEvent
	for i := range *got {
		if (*got)[i].Kind == "tool" {
			tool = &(*got)[i]
		}
	}
	if tool == nil || tool.Tool != "write" {
		t.Fatalf("expected a tool event named 'write', got %+v", *got)
	}
}

// toolLine builds one stream-json "message" event carrying a toolRequest
// block for the named tool.
func toolLine(name string) string {
	return `{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"c1","toolCall":{"status":"success","value":{"name":` + jsonStr(name) + `}}}]}}`
}

// The aggregator must record whether the agent called `complete_stage` — the
// only real signal that the stage was submitted. The tracker namespaces it as
// `tracker__complete_stage`, so a suffix match is required.
func TestStreamAggregator_TracksCompleteStageCall(t *testing.T) {
	t.Run("submitted", func(t *testing.T) {
		emit, _ := collectEvents()
		a := newStreamAggregator(1, emit)
		feed(a, textLine("here is the design"), toolLine("tracker__complete_stage"))
		if res := a.finish(); !res.completeStageCalled {
			t.Error("completeStageCalled = false, want true when tracker__complete_stage was invoked")
		}
	})
	t.Run("not submitted", func(t *testing.T) {
		emit, _ := collectEvents()
		a := newStreamAggregator(1, emit)
		feed(a, textLine("here is the design"), toolLine("text_editor"), toolLine("shell"))
		if res := a.finish(); res.completeStageCalled {
			t.Error("completeStageCalled = true, want false when complete_stage was never called")
		}
	})
}

// Token totals come from the terminal "complete" event, not from a messages[]
// array (which stream-json does not send at the end).
func TestStreamAggregator_ExtractsTokensFromComplete(t *testing.T) {
	emit, _ := collectEvents()
	a := newStreamAggregator(1, emit)

	feed(a,
		thinkingLine("ok"),
		`{"type":"complete","total_tokens":305,"input_tokens":146,"output_tokens":159}`,
	)
	res := a.finish()

	if res.tokens != 305 {
		t.Errorf("tokens = %d, want 305", res.tokens)
	}
}

// Provider failures surface (exit 0) as assistant text; detection must work off
// the streamed text fragments, classifying the credit-exhausted case.
func TestStreamAggregator_DetectsProviderErrorFromStream(t *testing.T) {
	emit, _ := collectEvents()
	a := newStreamAggregator(1, emit)

	// The error arrives fragmented across several text events, as it would live.
	for _, frag := range []string{
		"Ran into this error: ", "Request failed: Bad request (400): ",
		"Your credit balance is too low", " to access the API.",
	} {
		a.line([]byte(textLine(frag)))
	}
	res := a.finish()

	if res.provErr == nil {
		t.Fatalf("expected a provider error, got nil")
	}
	if res.provErr.Code != common.ErrCodeProviderCreditExhausted {
		t.Errorf("code = %q, want %q", res.provErr.Code, common.ErrCodeProviderCreditExhausted)
	}
}
