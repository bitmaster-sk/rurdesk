package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoPathFromPullsUrl(t *testing.T) {
	repoPath, ok := repoPathFromPullsUrl("/repos/org/my-repo/pulls/7")
	require.True(t, ok)
	assert.Equal(t, "org/my-repo", repoPath)

	repoPath, ok = repoPathFromPullsUrl("/api/v3/repos/org/my-repo/pulls/7")
	require.True(t, ok, "a self-hosted GitHub base URL adds the /api/v3 prefix")
	assert.Equal(t, "org/my-repo", repoPath)

	_, ok = repoPathFromPullsUrl("/repos/org/my-repo/issues/7")
	assert.False(t, ok, "a non-pulls path must not be treated as a PR lookup")

	_, ok = repoPathFromPullsUrl("/repos/org")
	assert.False(t, ok)
}

func TestPrStateStore_UnknownRepoReadsAsOpen(t *testing.T) {
	prStates.reset()

	state := prStates.get("org/repo")

	assert.Equal(t, "open", state.State)
	assert.False(t, state.Merged)
}

func TestPrStateStore_ReportsTheConfiguredState(t *testing.T) {
	prStates.reset()
	prStates.put("org/repo", PrState{State: "closed", Merged: true})

	state := prStates.get("org/repo")

	assert.Equal(t, "closed", state.State)
	assert.True(t, state.Merged)
}
