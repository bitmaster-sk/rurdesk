package githost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubHost_GetMergeRequestChanges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"head":  map[string]any{"sha": "abc123"},
				"state": "open",
			})
		case "/api/v3/repos/owner/repo/pulls/1/files":
			json.NewEncoder(w).Encode([]map[string]any{
				{"filename": "src/main.go", "previous_filename": "src/main.go", "patch": "some patch"},
				{"filename": "src/new.go", "previous_filename": "", "patch": "new file patch"},
			})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "test-token")
	diff, err := host.GetMergeRequestChanges(t.Context(), "1")
	require.NoError(t, err)
	require.NotNil(t, diff)
	assert.Equal(t, "abc123", diff.HeadSHA)
	assert.Len(t, diff.Files, 2)
	assert.Equal(t, "src/main.go", diff.Files[0].OldPath)
	assert.Equal(t, "src/main.go", diff.Files[0].NewPath)
	assert.Equal(t, "src/new.go", diff.Files[1].OldPath)
}

func TestGitHubHost_GetMergeRequestChanges_Pagination(t *testing.T) {
	callCount := 0
	page1Files := make([]map[string]any, 100)
	for i := range page1Files {
		page1Files[i] = map[string]any{"filename": "file1.go", "patch": "p"}
	}
	page2Files := []map[string]any{
		{"filename": "extra.go", "patch": "extra"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"head":  map[string]any{"sha": "deadbeef"},
				"state": "open",
			})
		case "/api/v3/repos/owner/repo/pulls/1/files":
			page := r.URL.Query().Get("page")
			callCount++
			if page == "2" {
				json.NewEncoder(w).Encode(page2Files)
			} else {
				w.Header().Set("Link", `<http://example.com/pulls/1/files?page=2>; rel="next"`)
				json.NewEncoder(w).Encode(page1Files)
			}
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	diff, err := host.GetMergeRequestChanges(t.Context(), "1")
	require.NoError(t, err)
	assert.Len(t, diff.Files, 101)
	assert.Equal(t, 2, callCount)
}

func githubCheckRunsPayload(runs ...map[string]any) map[string]any {
	return map[string]any{"total_count": len(runs), "check_runs": runs}
}

func TestGitHubHost_GetMergeRequestStatus_Open(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
			json.NewEncoder(w).Encode(githubCheckRunsPayload(
				map[string]any{"name": "build", "status": "completed", "conclusion": "success"},
				map[string]any{"name": "test", "status": "completed", "conclusion": "success"},
			))
		case "/api/v3/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateOpen, status.State)
	assert.Equal(t, constants.CiStatusSuccess, status.CiStatus)
	assert.False(t, status.Approved)
	assert.Equal(t, srv.URL+"/owner/repo/pull/1", status.WebUrl)
}

func TestGitHubHost_GetMergeRequestStatus_Merged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "closed", "merged": true,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
			json.NewEncoder(w).Encode(githubCheckRunsPayload(
				map[string]any{"name": "build", "status": "completed", "conclusion": "success"},
			))
		case "/api/v3/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{{"state": "APPROVED"}})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateMerged, status.State)
	assert.True(t, status.Approved)
	assert.Equal(t, srv.URL+"/owner/repo/pull/1", status.WebUrl)
}

func TestGitHubHost_CiStatus_FailureOnSecondCheckRunsPage(t *testing.T) {
	var pagesServed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
			assert.Equal(t, "latest", r.URL.Query().Get("filter"))
			page := r.URL.Query().Get("page")
			pagesServed = append(pagesServed, page)
			runs := make([]map[string]any, 0, githubCheckRunsPageSize)
			if page == "2" {
				runs = append(runs, map[string]any{"name": "flaky", "status": "completed", "conclusion": "failure"})
			} else {
				for i := 0; i < githubCheckRunsPageSize; i++ {
					runs = append(runs, map[string]any{"name": "job", "status": "completed", "conclusion": "success"})
				}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"total_count": githubCheckRunsPageSize + 1,
				"check_runs":  runs,
			})
		case "/api/v3/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.CiStatusFailed, status.CiStatus)
	assert.Equal(t, []string{"1", "2"}, pagesServed)
}

func TestGitHubHost_CiStatus_FallsBackToCommitStatuses(t *testing.T) {
	statusesCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
			json.NewEncoder(w).Encode(map[string]any{"total_count": 0, "check_runs": []map[string]any{}})
		case "/api/v3/repos/owner/repo/commits/abc123/status":
			statusesCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"state": "failure",
				"statuses": []map[string]any{
					{"context": "ci/jenkins", "state": "failure"},
				},
			})
		case "/api/v3/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.True(t, statusesCalled)
	assert.Equal(t, constants.CiStatusFailed, status.CiStatus)
}

func TestGitHubHost_CiStatus_NoCiAtAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
			json.NewEncoder(w).Encode(map[string]any{"total_count": 0, "check_runs": []map[string]any{}})
		case "/api/v3/repos/owner/repo/commits/abc123/status":
			json.NewEncoder(w).Encode(map[string]any{
				"state":       "pending",
				"total_count": 0,
				"statuses":    []map[string]any{},
			})
		case "/api/v3/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.CiStatusUnknown, status.CiStatus)
}

func TestGitHubHost_CiStatus_CheckRunConclusions(t *testing.T) {
	tests := []struct {
		name string
		run  map[string]any
		want string
	}{
		{name: "queued has no conclusion yet", run: map[string]any{"status": "queued"}, want: constants.CiStatusPending},
		{name: "in progress", run: map[string]any{"status": "in_progress"}, want: constants.CiStatusPending},
		{name: "success", run: map[string]any{"status": "completed", "conclusion": "success"}, want: constants.CiStatusSuccess},
		{name: "neutral counts as passing", run: map[string]any{"status": "completed", "conclusion": "neutral"}, want: constants.CiStatusSuccess},
		{name: "failure", run: map[string]any{"status": "completed", "conclusion": "failure"}, want: constants.CiStatusFailed},
		{name: "timed out", run: map[string]any{"status": "completed", "conclusion": "timed_out"}, want: constants.CiStatusFailed},
		{name: "action required", run: map[string]any{"status": "completed", "conclusion": "action_required"}, want: constants.CiStatusFailed},
		{name: "cancelled", run: map[string]any{"status": "completed", "conclusion": "cancelled"}, want: constants.CiStatusCanceled},
		{name: "skipped", run: map[string]any{"status": "completed", "conclusion": "skipped"}, want: constants.CiStatusSkipped},
		{name: "a conclusion we have never seen", run: map[string]any{"status": "completed", "conclusion": "brand_new_thing"}, want: constants.CiStatusUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v3/repos/owner/repo/pulls/1":
					json.NewEncoder(w).Encode(map[string]any{
						"state": "open", "merged": false,
						"head": map[string]any{"sha": "abc123"},
					})
				case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
					json.NewEncoder(w).Encode(githubCheckRunsPayload(tc.run))
				case "/api/v3/repos/owner/repo/commits/abc123/status":
					json.NewEncoder(w).Encode(map[string]any{"state": "pending", "statuses": []map[string]any{}})
				case "/api/v3/repos/owner/repo/pulls/1/reviews":
					json.NewEncoder(w).Encode([]map[string]any{})
				}
			}))
			defer srv.Close()

			host := NewGitHubHost(srv.URL, "owner/repo", "token")
			status, err := host.GetMergeRequestStatus(t.Context(), "1")
			require.NoError(t, err)
			assert.Equal(t, tc.want, status.CiStatus)
		})
	}
}

func TestGitHubHost_CiStatus_CanceledAmongGreen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/repos/owner/repo/pulls/1":
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/api/v3/repos/owner/repo/commits/abc123/check-runs":
			json.NewEncoder(w).Encode(githubCheckRunsPayload(
				map[string]any{"name": "build", "status": "completed", "conclusion": "success"},
				map[string]any{"name": "e2e", "status": "completed", "conclusion": "cancelled"},
				map[string]any{"name": "docs", "status": "completed", "conclusion": "skipped"},
			))
		case "/api/v3/repos/owner/repo/pulls/1/reviews":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.CiStatusCanceled, status.CiStatus)
}

func TestGitHubHost_GetMergeRequestUrl(t *testing.T) {
	host := NewGitHubHost("https://github.com", "owner/repo", "token")
	assert.Equal(t, "https://github.com/owner/repo/pull/42", host.GetMergeRequestUrl("42"))

	gheHost := NewGitHubHost("https://github.example.com", "org/repo", "token")
	assert.Equal(t, "https://github.example.com/org/repo/pull/7", gheHost.GetMergeRequestUrl("7"))
}

func TestGitHubHost_BaseUrlDetection(t *testing.T) {
	tests := []struct {
		baseUrl     string
		wantApiBase string
	}{
		{"https://github.com", "https://api.github.com"},
		{"https://github.com/", "https://api.github.com"},
		{"https://github.example.com", "https://github.example.com/api/v3"},
		{"https://github-enterprise.acme.io", "https://github-enterprise.acme.io/api/v3"},
	}
	for _, tc := range tests {
		host := NewGitHubHost(tc.baseUrl, "owner/repo", "token")
		assert.Equal(t, tc.wantApiBase, host.apiBase, "base url: %s", tc.baseUrl)
	}
}

func TestGitHubHost_DefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/repos/owner/repo", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	branch, err := host.DefaultBranch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestGitHubHost_FindOpenPullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/repos/owner/repo/pulls", r.URL.Path)
		assert.Equal(t, "owner:feature", r.URL.Query().Get("head"))
		assert.Equal(t, "open", r.URL.Query().Get("state"))
		json.NewEncoder(w).Encode([]map[string]any{
			{"number": 42, "html_url": "https://github.com/owner/repo/pull/42"},
		})
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	id, prURL, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "42", id)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", prURL)
}

func TestGitHubHost_FindOpenPullRequest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	_, _, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestGitHubHost_CreatePullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v3/repos/owner/repo/pulls", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "feature", body["head"])
		assert.Equal(t, "main", body["base"])
		assert.Equal(t, "My PR", body["title"])
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"number": 7, "html_url": "https://github.com/owner/repo/pull/7"})
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	id, prURL, err := host.CreatePullRequest(t.Context(), "feature", "main", "My PR", "body")
	require.NoError(t, err)
	assert.Equal(t, "7", id)
	assert.Equal(t, "https://github.com/owner/repo/pull/7", prURL)
}

func TestGitHubHost_CreatePullRequest_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{"message": "No commits between main and feature"})
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	_, _, err := host.CreatePullRequest(t.Context(), "feature", "main", "t", "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "422")
	assert.Contains(t, err.Error(), "No commits")
}

func TestGitHubHost_RetryAfter429(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if r.URL.Path == "/api/v3/repos/owner/repo/pulls/1" {
			json.NewEncoder(w).Encode(map[string]any{
				"state": "open", "merged": false,
				"head": map[string]any{"sha": "abc"},
			})
			return
		}
		if r.URL.Path == "/api/v3/repos/owner/repo/commits/abc/check-runs" {
			json.NewEncoder(w).Encode(githubCheckRunsPayload(
				map[string]any{"name": "build", "status": "in_progress"},
			))
			return
		}
		if r.URL.Path == "/api/v3/repos/owner/repo/pulls/1/reviews" {
			json.NewEncoder(w).Encode([]map[string]any{})
			return
		}
	}))
	defer srv.Close()

	host := NewGitHubHost(srv.URL, "owner/repo", "token")
	_, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, attempts, 2)
}
