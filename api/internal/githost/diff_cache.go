package githost

import (
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
)

// DiffCache holds two in-memory LRU caches: one for diffs (24h TTL) and one for statuses (30s TTL).
// Keys include id_git_integration so different integrations with overlapping MR IDs never collide.
type DiffCache struct {
	diffs    *lru.LRU[string, *Diff]
	statuses *lru.LRU[string, *Status]
	contents *lru.LRU[string, []byte]

	// integrationIndex maps id_git_integration → set of cache keys for bulk purge.
	mu           sync.Mutex
	diffIndex    map[int64]map[string]struct{}
	statusIndex  map[int64]map[string]struct{}
	contentIndex map[int64]map[string]struct{}
}

func NewDiffCache(diffMaxSize, statusMaxSize, contentMaxSize int) *DiffCache {
	return &DiffCache{
		diffs:        lru.NewLRU[string, *Diff](diffMaxSize, nil, 24*time.Hour),
		statuses:     lru.NewLRU[string, *Status](statusMaxSize, nil, 30*time.Second),
		contents:     lru.NewLRU[string, []byte](contentMaxSize, nil, 24*time.Hour),
		diffIndex:    make(map[int64]map[string]struct{}),
		statusIndex:  make(map[int64]map[string]struct{}),
		contentIndex: make(map[int64]map[string]struct{}),
	}
}

func diffKey(idGitIntegration int64, mrId, sha string) string {
	return fmt.Sprintf("%d:%s:%s", idGitIntegration, mrId, sha)
}

func contentKey(idGitIntegration int64, sha, path string) string {
	return fmt.Sprintf("%d:%s:%s", idGitIntegration, sha, path)
}

func statusKey(idGitIntegration int64, mrId string) string {
	return fmt.Sprintf("%d:%s", idGitIntegration, mrId)
}

func (c *DiffCache) GetDiff(idGitIntegration int64, mrId, sha string) (*Diff, bool) {
	return c.diffs.Get(diffKey(idGitIntegration, mrId, sha))
}

func (c *DiffCache) SetDiff(idGitIntegration int64, mrId, sha string, diff *Diff) {
	key := diffKey(idGitIntegration, mrId, sha)
	c.diffs.Add(key, diff)
	c.mu.Lock()
	if c.diffIndex[idGitIntegration] == nil {
		c.diffIndex[idGitIntegration] = make(map[string]struct{})
	}
	c.diffIndex[idGitIntegration][key] = struct{}{}
	c.mu.Unlock()
}

func (c *DiffCache) GetStatus(idGitIntegration int64, mrId string) (*Status, bool) {
	return c.statuses.Get(statusKey(idGitIntegration, mrId))
}

func (c *DiffCache) SetStatus(idGitIntegration int64, mrId string, status *Status) {
	key := statusKey(idGitIntegration, mrId)
	c.statuses.Add(key, status)
	c.mu.Lock()
	if c.statusIndex[idGitIntegration] == nil {
		c.statusIndex[idGitIntegration] = make(map[string]struct{})
	}
	c.statusIndex[idGitIntegration][key] = struct{}{}
	c.mu.Unlock()
}

func (c *DiffCache) GetFileContent(idGitIntegration int64, sha, path string) ([]byte, bool) {
	return c.contents.Get(contentKey(idGitIntegration, sha, path))
}

func (c *DiffCache) SetFileContent(idGitIntegration int64, sha, path string, content []byte) {
	key := contentKey(idGitIntegration, sha, path)
	c.contents.Add(key, content)
	c.mu.Lock()
	if c.contentIndex[idGitIntegration] == nil {
		c.contentIndex[idGitIntegration] = make(map[string]struct{})
	}
	c.contentIndex[idGitIntegration][key] = struct{}{}
	c.mu.Unlock()
}

// PurgeIntegration drops all cached diffs and statuses for the given integration.
func (c *DiffCache) PurgeIntegration(idGitIntegration int64) {
	c.mu.Lock()
	diffKeys := c.diffIndex[idGitIntegration]
	statusKeys := c.statusIndex[idGitIntegration]
	contentKeys := c.contentIndex[idGitIntegration]
	delete(c.diffIndex, idGitIntegration)
	delete(c.statusIndex, idGitIntegration)
	delete(c.contentIndex, idGitIntegration)
	c.mu.Unlock()

	for key := range diffKeys {
		c.diffs.Remove(key)
	}
	for key := range statusKeys {
		c.statuses.Remove(key)
	}
	for key := range contentKeys {
		c.contents.Remove(key)
	}
}
