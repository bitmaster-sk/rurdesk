package common

import (
	"sync"
	"time"
)

type dedupEntry struct {
	expiresAt time.Time
}

// DedupCache is a thread-safe event-id deduplication cache with TTL eviction.
type DedupCache struct {
	mu      sync.Mutex
	entries map[string]dedupEntry
	ttl     time.Duration
}

func NewDedupCache(ttl time.Duration) *DedupCache {
	dc := &DedupCache{
		entries: make(map[string]dedupEntry),
		ttl:     ttl,
	}
	go dc.evictLoop()
	return dc
}

func (dc *DedupCache) IsProcessed(eventID string) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	entry, ok := dc.entries[eventID]
	if !ok {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		delete(dc.entries, eventID)
		return false
	}
	return true
}

func (dc *DedupCache) MarkProcessed(eventID string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.entries[eventID] = dedupEntry{expiresAt: time.Now().Add(dc.ttl)}
}

func (dc *DedupCache) evictLoop() {
	ticker := time.NewTicker(dc.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		dc.evict()
	}
}

func (dc *DedupCache) evict() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	now := time.Now()
	for id, entry := range dc.entries {
		if now.After(entry.expiresAt) {
			delete(dc.entries, id)
		}
	}
}
