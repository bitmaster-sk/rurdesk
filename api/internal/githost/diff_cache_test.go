package githost

import (
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
)

func TestDiffCache_Hit(t *testing.T) {
	cache := NewDiffCache(100, 50)
	diff := &Diff{HeadSHA: "abc", Files: []DiffFile{{OldPath: "a.go", NewPath: "a.go"}}}
	cache.SetDiff(1, "42", "abc123", diff)

	got, ok := cache.GetDiff(1, "42", "abc123")
	assert.True(t, ok)
	assert.Equal(t, "abc", got.HeadSHA)
}

func TestDiffCache_Miss_DifferentSha(t *testing.T) {
	cache := NewDiffCache(100, 50)
	diff := &Diff{HeadSHA: "sha1"}
	cache.SetDiff(1, "42", "sha1", diff)

	_, ok := cache.GetDiff(1, "42", "sha2")
	assert.False(t, ok)
}

func TestDiffCache_StatusHit(t *testing.T) {
	cache := NewDiffCache(100, 50)
	status := &Status{State: "open", Approved: false, CiStatus: "success"}
	cache.SetStatus(1, "42", status)

	got, ok := cache.GetStatus(1, "42")
	assert.True(t, ok)
	assert.Equal(t, "open", got.State)
}

func TestDiffCache_StatusExpiry(t *testing.T) {
	// Built directly (not via NewDiffCache) for a short status TTL.
	shortCache := &DiffCache{
		diffs:       lru.NewLRU[string, *Diff](100, nil, 24*time.Hour),
		statuses:    lru.NewLRU[string, *Status](50, nil, 10*time.Millisecond),
		diffIndex:   make(map[int64]map[string]struct{}),
		statusIndex: make(map[int64]map[string]struct{}),
	}
	shortCache.SetStatus(1, "42", &Status{State: "open"})

	got, ok := shortCache.GetStatus(1, "42")
	assert.True(t, ok)
	assert.Equal(t, "open", got.State)

	time.Sleep(20 * time.Millisecond)
	_, ok = shortCache.GetStatus(1, "42")
	assert.False(t, ok)
}

func TestDiffCache_PurgeIntegration(t *testing.T) {
	cache := NewDiffCache(100, 50)
	cache.SetDiff(7, "1", "sha1", &Diff{HeadSHA: "sha1"})
	cache.SetDiff(7, "2", "sha2", &Diff{HeadSHA: "sha2"})
	cache.SetStatus(7, "1", &Status{State: "open"})
	cache.SetDiff(8, "1", "sha1", &Diff{HeadSHA: "other"})

	cache.PurgeIntegration(7)

	_, ok1 := cache.GetDiff(7, "1", "sha1")
	_, ok2 := cache.GetDiff(7, "2", "sha2")
	_, ok3 := cache.GetStatus(7, "1")
	_, ok4 := cache.GetDiff(8, "1", "sha1")

	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.False(t, ok3)
	assert.True(t, ok4, "other integration must be unaffected")
}
