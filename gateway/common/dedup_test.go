package common

import (
	"testing"
	"time"
)

const dedupTestTTL = 20 * time.Millisecond

func storedEntryCount(dc *DedupCache) int {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return len(dc.entries)
}

func TestDedupCacheEvictsExpiredEntries(t *testing.T) {
	dc := NewDedupCache(dedupTestTTL)
	defer dc.Close()
	dc.MarkProcessed("event-1")

	time.Sleep(5 * dedupTestTTL)

	if count := storedEntryCount(dc); count != 0 {
		t.Fatalf("expected the eviction loop to drop the expired entry, %d left", count)
	}
}

func TestDedupCacheCloseStopsEviction(t *testing.T) {
	dc := NewDedupCache(dedupTestTTL)
	dc.MarkProcessed("event-1")
	dc.Close()
	dc.Close()

	time.Sleep(5 * dedupTestTTL)

	if count := storedEntryCount(dc); count != 1 {
		t.Fatalf("expected no eviction after Close, %d entries left", count)
	}
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
