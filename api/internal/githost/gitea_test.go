package githost

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaHost_GetMergeRequestChanges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/files") {
			json.NewEncoder(w).Encode([]map[string]any{
				{"filename": "main.go", "previous_filename": "main.go", "patch": "some patch"},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"head":  map[string]any{"sha": "cafebabe"},
			"state": "open",
		})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	diff, err := host.GetMergeRequestChanges(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, "cafebabe", diff.HeadSHA)
	assert.Len(t, diff.Files, 1)
}

func TestGiteaHost_GetMergeRequestStatus_Open(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"state": "open", "merged": false,
		})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateOpen, status.State)
	assert.Equal(t, constants.CiStatusUnknown, status.CiStatus)
}

func TestGiteaHost_GetMergeRequestStatus_Merged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"state": "closed", "merged": true,
		})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateMerged, status.State)
}

func TestGiteaHost_GetMergeRequestUrl(t *testing.T) {
	host := NewGiteaHost("https://gitea.example.com", "owner/repo", "token")
	assert.Equal(t, "https://gitea.example.com/owner/repo/pulls/3", host.GetMergeRequestUrl("3"))
}

func TestGiteaHost_DefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/owner/repo", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{"default_branch": "trunk"})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	branch, err := host.DefaultBranch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "trunk", branch)
}

func TestGiteaHost_FindOpenPullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/owner/repo/pulls", r.URL.Path)
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "html_url": "x", "head": map[string]any{"ref": "other"}},
			{"number": 5, "html_url": "https://gitea/owner/repo/pulls/5", "head": map[string]any{"ref": "feature"}},
		})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	id, prURL, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "5", id)
	assert.Equal(t, "https://gitea/owner/repo/pulls/5", prURL)
}

func TestGiteaHost_FindOpenPullRequest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "html_url": "x", "head": map[string]any{"ref": "other"}},
		})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	_, _, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.False(t, found)
}

// A repo with more open PRs than one page must not report "no PR for this
// branch" — the caller answers that by opening a duplicate.
func TestGiteaHost_FindOpenPullRequest_WalksPages(t *testing.T) {
	var pagesServed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		pagesServed = append(pagesServed, page)
		switch page {
		case "1":
			// A full page, so the caller cannot tell this is the last one.
			full := make([]map[string]any, 0, giteaPullsPageSize)
			for i := 0; i < giteaPullsPageSize; i++ {
				full = append(full, map[string]any{
					"number": i + 1, "html_url": "x",
					"head": map[string]any{"ref": fmt.Sprintf("other-%d", i)},
				})
			}
			json.NewEncoder(w).Encode(full)
		case "2":
			json.NewEncoder(w).Encode([]map[string]any{
				{"number": 99, "html_url": "https://gitea/owner/repo/pulls/99",
					"head": map[string]any{"ref": "feature"}},
			})
		default:
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	id, prURL, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.True(t, found, "a PR beyond the first page must still be found")
	assert.Equal(t, "99", id)
	assert.Equal(t, "https://gitea/owner/repo/pulls/99", prURL)
	assert.Equal(t, []string{"1", "2"}, pagesServed, "must stop as soon as it matches")
}

// A short page means the end of the list — no point asking for another.
func TestGiteaHost_FindOpenPullRequest_StopsOnShortPage(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "html_url": "x", "head": map[string]any{"ref": "other"}},
		})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	_, _, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, 1, requests)
}

func TestGiteaHost_CreatePullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/repos/owner/repo/pulls", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "feature", body["head"])
		assert.Equal(t, "main", body["base"])
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"number": 8, "html_url": "https://gitea/owner/repo/pulls/8"})
	}))
	defer srv.Close()

	host := NewGiteaHost(srv.URL, "owner/repo", "token")
	id, prURL, err := host.CreatePullRequest(t.Context(), "feature", "main", "title", "body")
	require.NoError(t, err)
	assert.Equal(t, "8", id)
	assert.Equal(t, "https://gitea/owner/repo/pulls/8", prURL)
}
