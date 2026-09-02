package agent

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

const thinkingTailBytes = 1024

const thinkingTailMaxAge = 6 * time.Hour

// thinkingTails keeps, per running task, the last bytes of thinking text and
// the highest batch seq seen. The tail is what survives when the full text is
// not persisted; the seq guards against the gateway resending a batch.
type thinkingTails struct {
	mu    sync.Mutex
	tails map[int64]thinkingTail
	now   func() time.Time
}

type thinkingTail struct {
	text      string
	lastSeq   int
	updatedAt time.Time
}

func newThinkingTails() *thinkingTails {
	return &thinkingTails{tails: make(map[int64]thinkingTail), now: time.Now}
}

func (t *thinkingTails) Append(idTask int64, text string) {
	if text == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	tail := t.tails[idTask]
	// Consecutive fragments are one sentence cut mid-word: concatenate, never
	// join with a newline.
	tail.text = trimToTail(tail.text+text, thinkingTailBytes)
	tail.updatedAt = t.now()
	t.tails[idTask] = tail
}

// HasSeenSeq reports whether the task already recorded this batch seq, which
// marks the batch as a gateway resend.
func (t *thinkingTails) HasSeenSeq(idTask int64, seq int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return seq <= t.tails[idTask].lastSeq
}

func (t *thinkingTails) RecordSeq(idTask int64, seq int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tail := t.tails[idTask]
	if seq > tail.lastSeq {
		tail.lastSeq = seq
		tail.updatedAt = t.now()
		t.tails[idTask] = tail
	}
}

func (t *thinkingTails) Take(idTask int64) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	tail := t.tails[idTask]
	delete(t.tails, idTask)
	return tail.text
}

func (t *thinkingTails) Sweep(maxAge time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := t.now().Add(-maxAge)
	swept := 0
	for idTask, tail := range t.tails {
		if tail.updatedAt.Before(cutoff) {
			delete(t.tails, idTask)
			swept++
		}
	}
	return swept
}

func trimToTail(s string, maxBytes int) string {
	if len(s) > maxBytes {
		s = s[len(s)-maxBytes:]
	}
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

// tailFromEvents builds the tail out of the thinking text of the events. Tool
// output is left out: it is the part most likely to carry repository content,
// and the tail survives when the full text is not kept.
func tailFromEvents(events model.AgentThinkingEvents) string {
	var thinking strings.Builder
	for _, event := range events {
		if event.Kind == model.ThinkingKindThinking {
			thinking.WriteString(event.Text)
		}
	}
	return trimToTail(thinking.String(), thinkingTailBytes)
}
