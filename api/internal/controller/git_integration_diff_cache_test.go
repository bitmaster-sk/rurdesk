package controller

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/stretchr/testify/require"
)

// countingGitHost counts diff-endpoint hits so tests can prove the cache is
// actually read, not just written.
type countingGitHost struct {
	statusSHA    string // HeadSHA reported by the status endpoint
	diffSHA      string // HeadSHA carried by the returned diff
	changesCalls int
	statusCalls  int
}

func (h *countingGitHost) GetMergeRequestChanges(context.Context, string) (*githost.Diff, error) {
	h.changesCalls++
	return &githost.Diff{
		HeadSHA: h.diffSHA,
		Files:   []githost.DiffFile{{OldPath: "a", NewPath: "a", Patch: "@@"}},
	}, nil
}

func (h *countingGitHost) GetMergeRequestStatus(context.Context, string) (*githost.Status, error) {
	h.statusCalls++
	return &githost.Status{State: "open", HeadSHA: h.statusSHA}, nil
}

func (h *countingGitHost) GetMergeRequestUrl(string) string              { return "" }
func (h *countingGitHost) DefaultBranch(context.Context) (string, error) { return "main", nil }
func (h *countingGitHost) FindOpenPullRequest(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}
func (h *countingGitHost) CreatePullRequest(context.Context, string, string, string, string) (string, string, error) {
	return "", "", nil
}

func newDiffController() *GitIntegrationController {
	return &GitIntegrationController{diffCache: githost.NewDiffCache(16, 16)}
}

// Second fetch for the same head SHA must be served from the cache — the changes
// endpoint is hit exactly once.
func Test_fetchDiff_SameHead_ServedFromCache(t *testing.T) {
	gc := newDiffController()
	host := &countingGitHost{statusSHA: "sha1", diffSHA: "sha1"}
	ctx := context.Background()

	first, err := gc.fetchDiff(ctx, host, 7, "1")
	require.NoError(t, err)
	require.Equal(t, "sha1", first.HeadSHA)

	second, err := gc.fetchDiff(ctx, host, 7, "1")
	require.NoError(t, err)
	require.Equal(t, "sha1", second.HeadSHA)

	require.Equal(t, 1, host.changesCalls, "diff endpoint must be hit once; second view is a cache hit")
}

// A cached diff must only be served for its own head SHA, never when stale.
func Test_fetchDiff_DifferentHead_NotServedStale(t *testing.T) {
	gc := newDiffController()
	// Seed a sentinel diff under an old SHA that must never be returned.
	gc.diffCache.SetDiff(7, "1", "old", &githost.Diff{HeadSHA: "old"})

	host := &countingGitHost{statusSHA: "new", diffSHA: "new"}
	got, err := gc.fetchDiff(context.Background(), host, 7, "1")
	require.NoError(t, err)

	require.Equal(t, "new", got.HeadSHA, "must fetch the diff for the current head, not the cached old one")
	require.Equal(t, 1, host.changesCalls)
}

// With no head SHA (e.g. merged/closed MR), fall back to hitting the host every
// time — no caching, but no crash either.
func Test_fetchDiff_EmptyHead_AlwaysFetches(t *testing.T) {
	gc := newDiffController()
	host := &countingGitHost{statusSHA: "", diffSHA: ""}
	ctx := context.Background()

	_, err := gc.fetchDiff(ctx, host, 7, "1")
	require.NoError(t, err)
	_, err = gc.fetchDiff(ctx, host, 7, "1")
	require.NoError(t, err)

	require.Equal(t, 2, host.changesCalls, "empty head SHA is uncacheable; every view hits the host")
}
