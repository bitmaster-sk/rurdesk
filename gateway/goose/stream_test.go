package goose

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/bitmaster-sk/rurdesk/gateway/goose/entity"
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
	if res.Tokens != 305 {
		t.Errorf("tokens = %d, want 305 (from complete event)", res.Tokens)
	}
	if res.ProviderError != nil {
		t.Errorf("providerError = %v, want nil on a clean answer", res.ProviderError)
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
func collectEvents() (func(entity.StreamEvent), *[]entity.StreamEvent) {
	var got []entity.StreamEvent
	return func(ev entity.StreamEvent) { got = append(got, ev) }, &got
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
	if res.ProviderError != nil {
		t.Errorf("providerError = %v, want nil on clean run", res.ProviderError)
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

	if res.ToolCalls != 1 {
		t.Errorf("toolCalls = %d, want 1", res.ToolCalls)
	}
	var tool *entity.StreamEvent
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

// toolResponseLine builds the toolResponse block goose emits once the tool
// with the given call id returned.
func toolResponseLine(callId string, isError bool) string {
	return `{"type":"message","message":{"role":"user","content":[{"type":"toolResponse","id":` + jsonStr(callId) + `,"toolResult":{"status":"success","value":{"content":[],"isError":` + strconv.FormatBool(isError) + `}}}]}}`
}

// The aggregator must record whether the tracker accepted `complete_stage` —
// the only real signal that the stage was submitted. The tracker namespaces it
// as `tracker__complete_stage`, so a suffix match is required, and a call the
// tracker rejected does not count.
func TestStreamAggregator_TracksCompleteStageCall(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"accepted", []string{textLine("here is the design"), toolLine("tracker__complete_stage"), toolResponseLine("c1", false)}, true},
		{"rejected by the tracker", []string{toolLine("tracker__complete_stage"), toolResponseLine("c1", true)}, false},
		{"no response yet", []string{toolLine("tracker__complete_stage")}, false},
		{"response to another tool", []string{toolLine("tracker__complete_stage"), toolResponseLine("other", false)}, false},
		{"never called", []string{textLine("here is the design"), toolLine("edit"), toolLine("shell")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emit, _ := collectEvents()
			a := newStreamAggregator(1, emit)
			feed(a, tt.lines...)
			if got := a.finish().CompleteStageCalled; got != tt.want {
				t.Errorf("completeStageCalled = %v, want %v", got, tt.want)
			}
		})
	}
}

// The tracker must learn about a submission as soon as the call is made, so its
// heartbeat does not stop the agent while the tool result is still on its way.
func TestStreamAggregator_ReportsCompleteStageOnRequest(t *testing.T) {
	emit, _ := collectEvents()
	a := newStreamAggregator(1, emit)
	isReported := false
	a.onCompleteStage = func() { isReported = true }

	feed(a, toolLine("tracker__complete_stage"))

	if !isReported {
		t.Error("complete_stage request was not reported")
	}
}

func feedFixture(t *testing.T, a *streamAggregator, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "stream-json", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		a.line([]byte(line))
	}
}

// Real goose v1.56.0 captures: the stage counts as submitted only when the
// tracker accepted complete_stage, and the Stop hook notices in between are
// ignored.
func TestStreamAggregator_CompleteStageCaptures(t *testing.T) {
	tests := []struct {
		fixture string
		want    bool
	}{
		{"v1.56-complete-stage-accepted.ndjson", true},
		{"v1.56-complete-stage-rejected.ndjson", false},
		{"v1.56-complete-stage-never-called.ndjson", false},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			emit, _ := collectEvents()
			a := newStreamAggregator(1, emit)
			feedFixture(t, a, tt.fixture)
			res := a.finish()
			if res.CompleteStageCalled != tt.want {
				t.Errorf("completeStageCalled = %v, want %v", res.CompleteStageCalled, tt.want)
			}
			if res.ProviderError != nil {
				t.Errorf("providerError = %v, want nil", res.ProviderError)
			}
		})
	}
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

	if res.Tokens != 305 {
		t.Errorf("tokens = %d, want 305", res.Tokens)
	}
}

// Real goose v1.56.0 captures of provider failures: each must map to the code
// the UI translates, and a run that failed must keep goose's own error text.
func TestStreamAggregator_ClassifiesProviderErrors(t *testing.T) {
	tests := []struct {
		fixture             string
		wantCode            string
		wantDetail          string
		wantRunErrorMessage bool
	}{
		{"v1.56-provider-auth-401.ndjson", common.ErrCodeProviderAuthFailed, "Authentication failed", true},
		{"v1.56-provider-credits-402.ndjson", common.ErrCodeProviderCreditExhausted, "add more credits", false},
		{"v1.56-provider-rate-limit-429.ndjson", common.ErrCodeProviderRateLimited, "Rate limit exceeded", true},
		{"v1.56-provider-low-credit-400.ndjson", common.ErrCodeProviderCreditExhausted, "credit balance is too low", true},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			emit, _ := collectEvents()
			a := newStreamAggregator(1, emit)
			feedFixture(t, a, tt.fixture)
			res := a.finish()

			if res.ProviderError == nil {
				t.Fatal("expected a provider error, got nil")
			}
			if res.ProviderError.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", res.ProviderError.Code, tt.wantCode)
			}
			if !strings.Contains(res.ProviderError.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", res.ProviderError.Detail, tt.wantDetail)
			}
			if (res.RunErrorMessage != "") != tt.wantRunErrorMessage {
				t.Errorf("runErrorMessage = %q, want present=%v", res.RunErrorMessage, tt.wantRunErrorMessage)
			}
		})
	}
}

func TestClassifyProviderErrorKind(t *testing.T) {
	tests := []struct {
		kind, message, want string
	}{
		{"authentication", "Authentication failed", common.ErrCodeProviderAuthFailed},
		{"creditsExhausted", "no credit", common.ErrCodeProviderCreditExhausted},
		{"contextLengthExceeded", "Context length exceeded", common.ErrCodeProviderError},
		{"other", "Rate limit exceeded: slow down", common.ErrCodeProviderRateLimited},
		{"other", "Server error (500)", common.ErrCodeProviderError},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.message, func(t *testing.T) {
			if got := classifyProviderErrorKind(tt.kind, tt.message).Code; got != tt.want {
				t.Errorf("code = %q, want %q", got, tt.want)
			}
		})
	}
}

// recordingSender captures the batches the thinking relay posts.
type recordingSender struct {
	mu     sync.Mutex
	events []common.ThinkingEvent
}

func (r *recordingSender) SendThinking(_ context.Context, _ int64, _ int, events []common.ThinkingEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
	return nil
}

// Thinking and tool calls reach the relay; answer text does not, because it
// already becomes the stage's output message.
func TestStreamAggregator_RelaysThinkingAndToolsOnly(t *testing.T) {
	sender := &recordingSender{}
	batcher := common.NewThinkingBatcher(sender, 42)
	batcher.Start(context.Background())

	a := newStreamAggregator(1, func(ev entity.StreamEvent) {
		logStreamEvent(ev, 1)
		queueThinkingEvent(ev, batcher)
	})
	a.line([]byte(thinkingLine("weighing the tokenizer options")))
	a.line([]byte(toolLine("developer__text_editor")))
	a.line([]byte(textLine("Done — the tokenizer is in place.")))
	a.finish()
	batcher.Stop()

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.events) != 2 {
		t.Fatalf("relayed %d events, want 2: %+v", len(sender.events), sender.events)
	}
	if sender.events[0].Kind != "thinking" || sender.events[0].Text != "weighing the tokenizer options" {
		t.Errorf("first relayed event = %+v", sender.events[0])
	}
	if sender.events[1].Kind != "tool" || sender.events[1].Tool != "developer__text_editor" {
		t.Errorf("second relayed event = %+v", sender.events[1])
	}
}

// toolLineWithArgs builds a toolRequest block carrying arguments, the shape
// goose emits for a real call (MCP ToolCall is {name, arguments}).
func toolLineWithArgs(name, args string) string {
	return `{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"c1","toolCall":{"status":"success","value":{"name":` + jsonStr(name) + `,"arguments":` + args + `}}}]}}`
}

// The queued event carries the detail alongside the name so the feed can show both.
func TestQueueThinkingEvent_CarriesToolDetail(t *testing.T) {
	sender := &recordingSender{}
	batcher := common.NewThinkingBatcher(sender, 42)
	batcher.Start(context.Background())

	a := newStreamAggregator(1, func(ev entity.StreamEvent) { queueThinkingEvent(ev, batcher) })
	a.line([]byte(toolLineWithArgs("developer__shell", `{"command":"npm test"}`)))
	a.finish()
	batcher.Stop()

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.events) != 1 {
		t.Fatalf("relayed %d events, want 1", len(sender.events))
	}
	if sender.events[0].Tool != "developer__shell" || sender.events[0].Text != "npm test" {
		t.Errorf("relayed event = %+v", sender.events[0])
	}
}
