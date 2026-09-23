package common

import (
	"testing"
	"time"
)

func TestDedupCacheCloseStopsEviction(t *testing.T) {
	dc := NewDedupCache(50 * time.Millisecond)
	dc.MarkProcessed("event-1")
	dc.Close()

	// After Close, the stop channel is closed. Verify it doesn't panic
	// and that calling Close again is safe.
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
