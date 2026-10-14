package githost_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaHost_GetMergeRequestChanges(t *testing.T) {
	fixture, err := os.ReadFile("testdata/gitea_pr.diff")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/pulls/1.diff":
			w.Header().Set("Content-Type", "text/plain")
			w.Write(fixture)
		case "/api/v1/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "cafebabe"}, "state": "open"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	diff, err := host.GetMergeRequestChanges(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, "cafebabe", diff.HeadSHA)

	type paths struct{ old, new string }
	var got []paths
	for _, f := range diff.Files {
		got = append(got, paths{f.OldPath, f.NewPath})
	}
	assert.Equal(t, []paths{
		{"OLD.md", "OLD.md"},
		{"moved.txt", "docs/moved.txt"},
		{"logo.png", "logo.png"},
		{"main.go", "main.go"},
		{"my notes.md", "my notes.md"},
		{"naïve.txt", "naïve.txt"},
		{"config.yml", "settings.yml"},
	}, got)

	byPath := map[string]githost.DiffFile{}
	for _, f := range diff.Files {
		byPath[f.NewPath] = f
	}
	for _, path := range []string{"OLD.md", "main.go", "my notes.md", "naïve.txt", "settings.yml"} {
		assert.True(t, strings.HasPrefix(byPath[path].Patch, "@@ "), "%s patch should start with a hunk header, got %q", path, byPath[path].Patch)
	}
	assert.Empty(t, byPath["logo.png"].Patch)
	assert.Empty(t, byPath["docs/moved.txt"].Patch)
	assert.Equal(t, "@@ -1 +0,0 @@\n-old readme", byPath["OLD.md"].Patch)
	assert.Equal(t, "@@ -3,4 +3,4 @@ line2\n line3\n line4\n line5\n-line6\n+line6 changed", byPath["settings.yml"].Patch)
}

func TestGiteaHost_GetMergeRequestChanges_PathsFromHeaderOnly(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want githost.DiffFile
	}{
		{
			name: "binary file with a space in its name",
			diff: "diff --git a/my logo.png b/my logo.png\nindex 1..2 100644\nBinary files a/my logo.png and b/my logo.png differ\n",
			want: githost.DiffFile{OldPath: "my logo.png", NewPath: "my logo.png"},
		},
		{
			name: "quoted binary path",
			diff: "diff --git \"a/l\\303\\266go.png\" \"b/l\\303\\266go.png\"\nnew file mode 100644\nBinary files /dev/null and \"b/l\\303\\266go.png\" differ\n",
			want: githost.DiffFile{OldPath: "lögo.png", NewPath: "lögo.png"},
		},
		{
			name: "mode change only",
			diff: "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n",
			want: githost.DiffFile{OldPath: "run.sh", NewPath: "run.sh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".diff") {
					w.Write([]byte(tt.diff))
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "cafebabe"}})
			}))
			defer srv.Close()

			host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
			diff, err := host.GetMergeRequestChanges(t.Context(), "1")
			require.NoError(t, err)
			assert.Equal(t, []githost.DiffFile{tt.want}, diff.Files)
		})
	}
}

func TestGiteaHost_GetMergeRequestChanges_EmptyDiff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".diff") {
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "cafebabe"}})
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	diff, err := host.GetMergeRequestChanges(t.Context(), "1")
	require.NoError(t, err)
	assert.Empty(t, diff.Files)
}

func TestGiteaHost_GetMergeRequestChanges_DiffError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".diff") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "cafebabe"}})
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	_, err := host.GetMergeRequestChanges(t.Context(), "1")
	assert.Error(t, err)
}

func TestGiteaHost_GetMergeRequestChanges_RejectsOversizedDiff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".diff") {
			w.Write(make([]byte, githost.MaxDiffBytes+1))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"head": map[string]any{"sha": "cafebabe"}})
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	_, err := host.GetMergeRequestChanges(t.Context(), "1")
	assert.ErrorIs(t, err, githost.ErrDiffTooLarge)
}

func TestGiteaHost_GetMergeRequestStatus_Open(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "cafebabe"},
			})
		case "/api/v1/repos/owner/repo/commits/cafebabe/status":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "success",
				"statuses": []map[string]any{
					{"context": "build", "status": "success", "state": "success"},
					{"context": "test", "status": "success", "state": "success"},
				},
			})
		case "/api/v1/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{{"state": "REQUEST_CHANGES"}})
		}
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateOpen, status.State)
	assert.Equal(t, constants.CiStatusSuccess, status.CiStatus)
	assert.False(t, status.Approved)
	assert.Equal(t, srv.URL+"/owner/repo/pulls/1", status.WebUrl)
}

func TestGiteaHost_GetMergeRequestStatus_Merged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "closed", "merged": true,
				"head": map[string]any{"sha": "cafebabe"},
			})
		case "/api/v1/repos/owner/repo/commits/cafebabe/status":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "success",
				"statuses": []map[string]any{
					{"context": "build", "state": "success"},
				},
			})
		case "/api/v1/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{{"state": "APPROVED"}})
		}
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateMerged, status.State)
	assert.True(t, status.Approved)
	assert.Equal(t, srv.URL+"/owner/repo/pulls/1", status.WebUrl)
}

func TestGiteaHost_GetMergeRequestStatus_NoCi(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "cafebabe"},
			})
		case "/api/v1/repos/owner/repo/commits/cafebabe/status":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "pending", "statuses": []map[string]any{},
			})
		case "/api/v1/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.CiStatusUnknown, status.CiStatus)
}

func TestGiteaHost_GetMergeRequestUrl(t *testing.T) {
	host := githost.NewGiteaHost("https://gitea.example.com", "owner/repo", "token")
	assert.Equal(t, "https://gitea.example.com/owner/repo/pulls/3", host.GetMergeRequestUrl("3"))
}

func TestGiteaHost_DefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/owner/repo", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{"default_branch": "trunk"})
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
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

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	id, prURL, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "5", id)
	assert.Equal(t, "https://gitea/owner/repo/pulls/5", prURL)
}

func TestGiteaHost_FindOpenPullRequest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "html_url": "x", "head": map[string]any{"ref": "other"}},
		})
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
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
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		pagesServed = append(pagesServed, page)
		switch page {
		case "1":
			// A full page, so the caller cannot tell this is the last one.
			full := make([]map[string]any, 0, limit)
			for i := 0; i < limit; i++ {
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

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "html_url": "x", "head": map[string]any{"ref": "other"}},
		})
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
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

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	id, prURL, err := host.CreatePullRequest(t.Context(), "feature", "main", "title", "body")
	require.NoError(t, err)
	assert.Equal(t, "8", id)
	assert.Equal(t, "https://gitea/owner/repo/pulls/8", prURL)
}

func TestGiteaHost_GetFileContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/owner/repo/raw/src/my%20file%23v2.go", r.URL.EscapedPath())
		assert.Equal(t, "abc123", r.URL.Query().Get("ref"))
		_, _ = w.Write([]byte("package main\n"))
	}))
	defer srv.Close()

	host := githost.NewGiteaHost(srv.URL, "owner/repo", "token")
	content, err := host.GetFileContent(t.Context(), "src/my file#v2.go", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "package main\n", string(content))
}
