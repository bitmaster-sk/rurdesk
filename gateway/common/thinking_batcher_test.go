package common

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type thinkingCall struct {
	seq    int
	events []ThinkingEvent
}

type fakeThinkingSender struct {
	mu           sync.Mutex
	calls        []thinkingCall
	err          error
	failuresLeft int
	delay        time.Duration
}

func (f *fakeThinkingSender) SendThinking(ctx context.Context, idTask int64, seq int, events []ThinkingEvent) error {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, thinkingCall{seq: seq, events: events})
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return errors.New("status 502")
	}
	return f.err
}

func (f *fakeThinkingSender) snapshot() []thinkingCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]thinkingCall(nil), f.calls...)
}

func waitForCalls(t *testing.T, sender *fakeThinkingSender, want int) []thinkingCall {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls := sender.snapshot(); len(calls) >= want {
			return calls
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("expected at least %d send(s), got %d", want, len(sender.snapshot()))
	return nil
}

func TestThinkingBatcher_FlushesOnSize(t *testing.T) {
	sender := &fakeThinkingSender{}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)
	defer batcher.Stop()

	batcher.Add(ThinkingEvent{Kind: "thinking", Text: strings.Repeat("x", thinkingFlushBytes+1)})

	calls := waitForCalls(t, sender, 1)
	if calls[0].seq != 1 {
		t.Errorf("seq = %d, want 1", calls[0].seq)
	}
	if len(calls[0].events) != 1 {
		t.Fatalf("events = %d, want 1", len(calls[0].events))
	}
	if calls[0].events[0].At == 0 {
		t.Error("event timestamp not stamped")
	}
}

func TestThinkingBatcher_FlushesOnInterval(t *testing.T) {
	sender := &fakeThinkingSender{}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)
	defer batcher.Stop()

	batcher.Add(ThinkingEvent{Kind: "tool", Tool: "read"})

	calls := waitForCalls(t, sender, 1)
	if len(calls[0].events) != 1 || calls[0].events[0].Tool != "read" {
		t.Errorf("unexpected batch: %+v", calls[0].events)
	}
}

func TestThinkingBatcher_SequenceIncrements(t *testing.T) {
	sender := &fakeThinkingSender{}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)
	defer batcher.Stop()

	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "one"})
	waitForCalls(t, sender, 1)
	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "two"})
	calls := waitForCalls(t, sender, 2)

	if calls[0].seq != 1 || calls[1].seq != 2 {
		t.Errorf("seqs = %d,%d, want 1,2", calls[0].seq, calls[1].seq)
	}
}

func TestThinkingBatcher_OverflowDropsOldest(t *testing.T) {
	sender := &fakeThinkingSender{}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)

	for i := 0; i < thinkingMaxPending+20; i++ {
		batcher.Add(ThinkingEvent{Kind: "tool", Tool: "t"})
	}
	batcher.Stop()

	calls := sender.snapshot()
	if len(calls) != 1 {
		t.Fatalf("sends = %d, want 1", len(calls))
	}
	if len(calls[0].events) != thinkingMaxPending {
		t.Errorf("events = %d, want %d", len(calls[0].events), thinkingMaxPending)
	}
	if batcher.DroppedCount() != 20 {
		t.Errorf("dropped = %d, want 20", batcher.DroppedCount())
	}
}

func TestThinkingBatcher_StopFlushesTail(t *testing.T) {
	sender := &fakeThinkingSender{}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)

	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "tail"})
	batcher.Stop()

	calls := sender.snapshot()
	if len(calls) != 1 || len(calls[0].events) != 1 || calls[0].events[0].Text != "tail" {
		t.Fatalf("tail not flushed: %+v", calls)
	}
}

func TestThinkingBatcher_ResendsAfterTransientFailure(t *testing.T) {
	sender := &fakeThinkingSender{failuresLeft: 2}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)

	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "one"})
	calls := waitForCalls(t, sender, 3)

	for index, call := range calls[:3] {
		if call.seq != 1 {
			t.Errorf("call %d seq = %d, want 1 (a resend keeps its seq)", index, call.seq)
		}
		if len(call.events) != 1 || call.events[0].Text != "one" {
			t.Errorf("call %d events = %+v, want the original batch", index, call.events)
		}
	}

	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "two"})
	calls = waitForCalls(t, sender, 4)
	last := calls[len(calls)-1]
	if last.seq != 2 || len(last.events) != 1 || last.events[0].Text != "two" {
		t.Errorf("after recovery got seq %d %+v, want seq 2 carrying \"two\"", last.seq, last.events)
	}
}

func TestThinkingBatcher_DisablesAfterRepeatedFailures(t *testing.T) {
	sender := &fakeThinkingSender{err: errors.New("status 404")}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)

	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "one"})
	waitForCalls(t, sender, thinkingMaxFailures)
	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "two"})
	time.Sleep(30 * time.Millisecond)
	batcher.Stop()

	if calls := sender.snapshot(); len(calls) != thinkingMaxFailures {
		t.Errorf("sends = %d, want %d (relay disabled after the retries)", len(calls), thinkingMaxFailures)
	}
}

func TestThinkingBatcher_AddNeverBlocksOnSlowSender(t *testing.T) {
	sender := &fakeThinkingSender{delay: 200 * time.Millisecond}
	batcher := NewThinkingBatcher(sender, 7)
	batcher.flushInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batcher.Start(ctx)
	defer batcher.Stop()

	start := time.Now()
	for i := 0; i < 500; i++ {
		batcher.Add(ThinkingEvent{Kind: "thinking", Text: "chunk"})
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("Add blocked for %v", elapsed)
	}
}

func TestThinkingBatcher_NilSenderIsInert(t *testing.T) {
	batcher := NewThinkingBatcher(nil, 7)
	batcher.Add(ThinkingEvent{Kind: "thinking", Text: "x"})
	batcher.Stop()
}
