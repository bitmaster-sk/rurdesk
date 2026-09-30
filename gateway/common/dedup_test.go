package common

import (
	"testing"
	"time"
)

func TestDedupCacheCloseStopsEviction(t *testing.T) {
	dc := NewDedupCache(50 * time.Millisecond)
	dc.MarkProcessed("event-1")
	dc.Close()

	// After Close, the cache must still be readable — Close only stops the
	// eviction goroutine, it does not clear entries.
	if !dc.IsProcessed("event-1") {
		t.Fatal("expected event-1 to still be marked processed after Close")
	}

	// Calling Close again must be safe (no panic on double-close).
	dc.Close()
}

func TestDedupCacheIsProcessed(t *testing.T) {
	dc := NewDedupCache(time.Second)
	defer dc.Close()

	if dc.IsProcessed("event-1") {
		t.Fatal("expected event-1 to not be processed")
	}

	dc.MarkProcessed("event-1")

	if !dc.IsProcessed("event-1") {
		t.Fatal("expected event-1 to be processed")
	}
}
