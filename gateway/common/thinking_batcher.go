package common

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	thinkingFlushBytes = 2 * 1024
	thinkingMaxPending = 256
	// Must stay above the tracker client's own timeout, or a slow tracker is cut
	// off here instead of failing there with its status.
	thinkingSendTimeout = 20 * time.Second
	thinkingMaxFailures = 5
)

// Kinds of a ThinkingEvent, matching the tracker's model.ThinkingKind* values.
const (
	ThinkingKindThinking = "thinking"
	ThinkingKindTool     = "tool"
)

type ThinkingEvent struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	Tool string `json:"tool,omitempty"`
	At   int64  `json:"at"`
}

type ThinkingSender interface {
	SendThinking(ctx context.Context, idTask int64, seq int, events []ThinkingEvent) error
}

type ThinkingBatcher struct {
	sender ThinkingSender
	idTask int64

	flushInterval time.Duration

	mu           sync.Mutex
	pending      []ThinkingEvent
	pendingBytes int
	dropped      int
	seq          int
	isDisabled   bool
	retry        []ThinkingEvent
	retrySeq     int
	failures     int

	isStarted bool
	wake      chan struct{}
	stopOnce  sync.Once
	stopped   chan struct{}
	done      chan struct{}
}

func NewThinkingBatcher(sender ThinkingSender, idTask int64) *ThinkingBatcher {
	return &ThinkingBatcher{
		sender:        sender,
		idTask:        idTask,
		flushInterval: time.Second,
		wake:          make(chan struct{}, 1),
		stopped:       make(chan struct{}),
		done:          make(chan struct{}),
	}
}

func (b *ThinkingBatcher) Start(ctx context.Context) {
	if b.sender == nil {
		return
	}
	b.mu.Lock()
	b.isStarted = true
	b.mu.Unlock()
	go b.loop(ctx)
}

// Runs on the stream-scanning goroutine and must never block.
func (b *ThinkingBatcher) Add(event ThinkingEvent) {
	if b.sender == nil {
		return
	}
	if event.At == 0 {
		event.At = time.Now().UnixMilli()
	}

	b.mu.Lock()
	if b.isDisabled {
		b.mu.Unlock()
		return
	}
	if len(b.pending) == thinkingMaxPending {
		b.pendingBytes -= len(b.pending[0].Text)
		b.pending = b.pending[1:]
		b.dropped++
	}
	b.pending = append(b.pending, event)
	b.pendingBytes += len(event.Text)
	isFull := b.pendingBytes >= thinkingFlushBytes
	b.mu.Unlock()

	if isFull {
		select {
		case b.wake <- struct{}{}:
		default:
		}
	}
}

func (b *ThinkingBatcher) Stop() {
	b.stopOnce.Do(func() { close(b.stopped) })
	b.mu.Lock()
	isStarted := b.isStarted
	b.mu.Unlock()
	if !isStarted {
		return
	}
	<-b.done
}

func (b *ThinkingBatcher) DroppedCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}

func (b *ThinkingBatcher) loop(ctx context.Context) {
	defer close(b.done)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			b.flushAll()
			return
		case <-b.stopped:
			b.flushAll()
			return
		case <-ticker.C:
			b.flush()
		case <-b.wake:
			b.flush()
		}
	}
}

func (b *ThinkingBatcher) flushAll() {
	b.flush()
	b.flush()
}

func (b *ThinkingBatcher) flush() {
	b.mu.Lock()
	if b.isDisabled || (len(b.pending) == 0 && len(b.retry) == 0) {
		b.mu.Unlock()
		return
	}
	// A failed batch keeps its seq so the tracker can recognise the resend; every
	// effect on that side is guarded by it.
	batch, seq := b.retry, b.retrySeq
	if len(batch) == 0 {
		batch = b.pending
		b.pending = nil
		b.pendingBytes = 0
		b.seq++
		seq = b.seq
	}
	b.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), thinkingSendTimeout)
	defer cancel()
	err := b.sender.SendThinking(ctx, b.idTask, seq, batch)

	b.mu.Lock()
	defer b.mu.Unlock()
	if err == nil {
		b.retry = nil
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= thinkingMaxFailures {
		b.isDisabled = true
		b.pending = nil
		b.pendingBytes = 0
		b.retry = nil
		log.Warn().Int64("idTask", b.idTask).Err(err).Msg("thinking relay disabled for this task")
		return
	}
	b.retry = batch
	b.retrySeq = seq
}
