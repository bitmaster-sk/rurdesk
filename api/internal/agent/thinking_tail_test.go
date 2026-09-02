package agent

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThinkingTails_KeepsCappedTail(t *testing.T) {
	tails := newThinkingTails()

	tails.Append(1, strings.Repeat("a", 2000))
	tails.Append(1, "the last thought")

	tail := tails.Take(1)
	assert.LessOrEqual(t, len(tail), thinkingTailBytes)
	assert.Contains(t, tail, "the last thought")
}

func TestThinkingTails_ConcatenatesFragments(t *testing.T) {
	tails := newThinkingTails()

	tails.Append(1, "does it return an object")
	tails.Append(1, "? Let us investigate.")

	assert.Equal(t, "does it return an object? Let us investigate.", tails.Take(1))
}

func TestThinkingTails_TakeForgets(t *testing.T) {
	tails := newThinkingTails()
	tails.Append(1, "something")

	require.NotEmpty(t, tails.Take(1))
	assert.Empty(t, tails.Take(1))
}

func TestThinkingTails_SeparatePerTask(t *testing.T) {
	tails := newThinkingTails()
	tails.Append(1, "task one")
	tails.Append(2, "task two")

	assert.Equal(t, "task one", tails.Take(1))
	assert.Equal(t, "task two", tails.Take(2))
}

func TestThinkingTails_CutsOnRuneBoundary(t *testing.T) {
	tails := newThinkingTails()
	tails.Append(1, strings.Repeat("ú", 2000))

	assert.True(t, strings.HasPrefix(tails.Take(1), "ú"))
}

func TestThinkingTails_KeepsTailAroundAnInvalidByte(t *testing.T) {
	tails := newThinkingTails()

	tails.Append(1, strings.Repeat("a", 2000)+"the last thought\xff")

	tail := tails.Take(1)
	assert.Contains(t, tail, "the last thought")
	assert.True(t, utf8.ValidString(tail), "the tail is stored as text and must be valid UTF-8")
}

func TestThinkingTails_SweepsAbandonedTails(t *testing.T) {
	tails := newThinkingTails()
	now := time.Now()
	tails.now = func() time.Time { return now }

	tails.Append(1, "the run that was cancelled")
	now = now.Add(thinkingTailMaxAge + time.Minute)
	tails.Append(2, "the run still working")

	assert.Equal(t, 1, tails.Sweep(thinkingTailMaxAge))
	assert.Empty(t, tails.Take(1))
	assert.Equal(t, "the run still working", tails.Take(2))
}

// The gateway resends a failed batch under its original seq. The rows dedup on
// their key, but the tail would otherwise be appended twice.
func TestThinkingTails_SeqIsSeenOnlyOnce(t *testing.T) {
	tails := newThinkingTails()

	assert.False(t, tails.HasSeenSeq(1, 1))
	tails.RecordSeq(1, 1)

	assert.True(t, tails.HasSeenSeq(1, 1))
	assert.False(t, tails.HasSeenSeq(1, 2))
}

func TestThinkingTails_SeqIsTrackedPerTask(t *testing.T) {
	tails := newThinkingTails()
	tails.RecordSeq(1, 4)

	assert.True(t, tails.HasSeenSeq(1, 4))
	assert.False(t, tails.HasSeenSeq(2, 4))
}

func TestTailFromEvents_KeepsThinkingOnly(t *testing.T) {
	tail := tailFromEvents(model.AgentThinkingEvents{
		{Kind: model.ThinkingKindThinking, Text: "does it return an object"},
		{Kind: model.ThinkingKindTool, Tool: "secret__dump", Text: "the private key"},
		{Kind: model.ThinkingKindThinking, Text: "? Let us investigate."},
	})

	assert.Equal(t, "does it return an object? Let us investigate.", tail)
}
