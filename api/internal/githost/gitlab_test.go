package githost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitLabHost_GetMergeRequestChanges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/changes") {
			json.NewEncoder(w).Encode(map[string]any{
				"diff_refs": map[string]any{"head_sha": "deadbeef"},
				"changes": []map[string]any{
					{"old_path": "src/a.go", "new_path": "src/a.go", "diff": "some diff"},
				},
			})
		}
	}))
	defer srv.Close()

	host := NewGitLabHost(srv.URL, "group/project", "token")
	diff, err := host.GetMergeRequestChanges(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, "deadbeef", diff.HeadSHA)
	assert.Len(t, diff.Files, 1)
}

func gitlabStatusServer(t *testing.T, mrState string, pipelines []map[string]any, approvals map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approvals"):
			json.NewEncoder(w).Encode(approvals)
		case strings.HasSuffix(r.URL.Path, "/pipelines"):
			json.NewEncoder(w).Encode(pipelines)
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"state":     mrState,
				"diff_refs": map[string]any{"head_sha": "deadbeef"},
			})
		}
	}))
}

func TestGitLabHost_GetMergeRequestStatus_Open(t *testing.T) {
	srv := gitlabStatusServer(t, "opened",
		[]map[string]any{{"id": 9, "status": "success"}},
		map[string]any{"approved": false, "approvals_required": 0, "approved_by": []any{}})
	defer srv.Close()

	host := NewGitLabHost(srv.URL, "group/project", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateOpen, status.State)
	assert.Equal(t, constants.CiStatusSuccess, status.CiStatus)
	assert.False(t, status.Approved)
}

func TestGitLabHost_GetMergeRequestStatus_Merged(t *testing.T) {
	var approvalsPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approvals"):
			approvalsPath = r.URL.Path
			json.NewEncoder(w).Encode(map[string]any{
				"approved": true,
				"approved_by": []any{
					map[string]any{"user": map[string]any{"id": 1, "username": "reviewer"}},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/pipelines"):
			json.NewEncoder(w).Encode([]map[string]any{{"status": "success"}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"state": "merged"})
		}
	}))
	defer srv.Close()

	host := NewGitLabHost(srv.URL, "group/project", "token")
	status, err := host.GetMergeRequestStatus(t.Context(), "1")
	require.NoError(t, err)
	assert.Equal(t, constants.MrStateMerged, status.State)
	assert.True(t, status.Approved)
	assert.Equal(t, "/api/v4/projects/group/project/merge_requests/1/approvals", approvalsPath)
}

func TestGitLabHost_GetMergeRequestStatus_PipelineStates(t *testing.T) {
	tests := []struct {
		name      string
		pipelines []map[string]any
		want      string
	}{
		{name: "success", pipelines: []map[string]any{{"status": "success"}}, want: constants.CiStatusSuccess},
		{name: "failed", pipelines: []map[string]any{{"status": "failed"}}, want: constants.CiStatusFailed},
		{name: "canceled", pipelines: []map[string]any{{"status": "canceled"}}, want: constants.CiStatusCanceled},
		{name: "canceling", pipelines: []map[string]any{{"status": "canceling"}}, want: constants.CiStatusCanceled},
		{name: "skipped", pipelines: []map[string]any{{"status": "skipped"}}, want: constants.CiStatusSkipped},
		{name: "running", pipelines: []map[string]any{{"status": "running"}}, want: constants.CiStatusPending},
		{name: "created is still in flight", pipelines: []map[string]any{{"status": "created"}}, want: constants.CiStatusPending},
		{name: "waiting for resource is still in flight", pipelines: []map[string]any{{"status": "waiting_for_resource"}}, want: constants.CiStatusPending},
		{name: "manual is still in flight", pipelines: []map[string]any{{"status": "manual"}}, want: constants.CiStatusPending},
		{name: "a state GitLab has not shipped yet is still in flight", pipelines: []map[string]any{{"status": "brand_new_thing"}}, want: constants.CiStatusPending},
		{name: "only the latest pipeline counts", pipelines: []map[string]any{{"status": "success"}, {"status": "failed"}}, want: constants.CiStatusSuccess},
		{name: "no pipeline at all", pipelines: []map[string]any{}, want: constants.CiStatusUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := gitlabStatusServer(t, "opened", tc.pipelines, map[string]any{"approved": false})
			defer srv.Close()

			host := NewGitLabHost(srv.URL, "group/project", "token")
			status, err := host.GetMergeRequestStatus(t.Context(), "1")
			require.NoError(t, err)
			assert.Equal(t, tc.want, status.CiStatus)
		})
	}
}

func TestGitLabHost_GetMergeRequestUrl(t *testing.T) {
	host := NewGitLabHost("https://gitlab.com", "group/project", "token")
	assert.Equal(t, "https://gitlab.com/group/project/-/merge_requests/5", host.GetMergeRequestUrl("5"))
}

func TestGitLabHost_EncodedRepoPath(t *testing.T) {
	host := NewGitLabHost("https://gitlab.com", "group/sub/project", "token")
	expected := url.PathEscape("group/sub/project")
	assert.Equal(t, expected, host.encodedPath)
}

func TestGitLabHost_DefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/api/v4/projects/group/project"))
		json.NewEncoder(w).Encode(map[string]any{"default_branch": "develop"})
	}))
	defer srv.Close()

	host := NewGitLabHost(srv.URL, "group/project", "token")
	branch, err := host.DefaultBranch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "develop", branch)
}

func TestGitLabHost_FindOpenPullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, "/merge_requests"))
		assert.Equal(t, "feature", r.URL.Query().Get("source_branch"))
		assert.Equal(t, "opened", r.URL.Query().Get("state"))
		json.NewEncoder(w).Encode([]map[string]any{
			{"iid": 9, "web_url": "https://gitlab.com/group/project/-/merge_requests/9"},
		})
	}))
	defer srv.Close()

	host := NewGitLabHost(srv.URL, "group/project", "token")
	id, mrURL, found, err := host.FindOpenPullRequest(t.Context(), "feature")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "9", id)
	assert.Equal(t, "https://gitlab.com/group/project/-/merge_requests/9", mrURL)
}

func TestGitLabHost_CreatePullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "feature", body["source_branch"])
		assert.Equal(t, "main", body["target_branch"])
		assert.Equal(t, "desc", body["description"])
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"iid": 3, "web_url": "https://gitlab.com/group/project/-/merge_requests/3"})
	}))
	defer srv.Close()

	host := NewGitLabHost(srv.URL, "group/project", "token")
	id, mrURL, err := host.CreatePullRequest(t.Context(), "feature", "main", "title", "desc")
	require.NoError(t, err)
	assert.Equal(t, "3", id)
	assert.Equal(t, "https://gitlab.com/group/project/-/merge_requests/3", mrURL)
}
