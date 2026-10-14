package githost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHostResponse struct {
	status  int
	body    any
	headers map[string]string
}

func newFakeHost(t *testing.T, routes map[string]fakeHostResponse) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, found := routes[r.URL.Path]
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		for name, value := range route.headers {
			w.Header().Set(name, value)
		}
		w.WriteHeader(route.status)
		if route.body != nil {
			_ = json.NewEncoder(w).Encode(route.body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckAccess(t *testing.T) {
	githubRepo := "/api/v3/repos/owner/repo"
	gitlabProject := "/api/v4/projects/group/repo"
	gitlabTokenSelf := "/api/v4/personal_access_tokens/self"
	giteaRepo := "/api/v1/repos/owner/repo"

	tests := []struct {
		name           string
		hostType       string
		repoPath       string
		routes         map[string]fakeHostResponse
		expectedResult string
		expectedBranch string
	}{
		{
			name:     "github token with push access is ok",
			hostType: model.HostTypeGitHub,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				githubRepo: {status: http.StatusOK, body: map[string]any{
					"default_branch": "main",
					"permissions":    map[string]any{"push": true},
				}},
			},
			expectedResult: AccessOk,
			expectedBranch: "main",
		},
		{
			name:     "github user without push is read only",
			hostType: model.HostTypeGitHub,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				githubRepo: {status: http.StatusOK, body: map[string]any{
					"default_branch": "main",
					"permissions":    map[string]any{"push": false},
				}},
			},
			expectedResult: AccessReadOnly,
			expectedBranch: "main",
		},
		{
			name:     "github classic token without repo scope is read only",
			hostType: model.HostTypeGitHub,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				githubRepo: {
					status:  http.StatusOK,
					headers: map[string]string{"X-OAuth-Scopes": "read:org, gist"},
					body: map[string]any{
						"default_branch": "main",
						"permissions":    map[string]any{"push": true},
					},
				},
			},
			expectedResult: AccessReadOnly,
			expectedBranch: "main",
		},
		{
			name:     "github classic token with repo scope is ok",
			hostType: model.HostTypeGitHub,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				githubRepo: {
					status:  http.StatusOK,
					headers: map[string]string{"X-OAuth-Scopes": "read:org, repo"},
					body: map[string]any{
						"default_branch": "main",
						"permissions":    map[string]any{"push": true},
					},
				},
			},
			expectedResult: AccessOk,
			expectedBranch: "main",
		},
		{
			name:     "github rejected token is invalid",
			hostType: model.HostTypeGitHub,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				githubRepo: {status: http.StatusUnauthorized},
			},
			expectedResult: AccessInvalidToken,
		},
		{
			name:           "github unknown repo is not found",
			hostType:       model.HostTypeGitHub,
			repoPath:       "owner/repo",
			routes:         map[string]fakeHostResponse{},
			expectedResult: AccessRepoNotFound,
		},
		{
			name:     "github non json answer is unreachable",
			hostType: model.HostTypeGitHub,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				githubRepo: {status: http.StatusOK, body: "<html>"},
			},
			expectedResult: AccessUnreachable,
		},
		{
			name:     "gitlab developer with api scope is ok",
			hostType: model.HostTypeGitLab,
			repoPath: "group/repo",
			routes: map[string]fakeHostResponse{
				gitlabProject: {status: http.StatusOK, body: map[string]any{
					"default_branch": "develop",
					"permissions": map[string]any{
						"project_access": map[string]any{"access_level": 30},
						"group_access":   nil,
					},
				}},
				gitlabTokenSelf: {status: http.StatusOK, body: map[string]any{"scopes": []string{"api"}}},
			},
			expectedResult: AccessOk,
			expectedBranch: "develop",
		},
		{
			name:     "gitlab group maintainer counts as write access",
			hostType: model.HostTypeGitLab,
			repoPath: "group/repo",
			routes: map[string]fakeHostResponse{
				gitlabProject: {status: http.StatusOK, body: map[string]any{
					"default_branch": "main",
					"permissions": map[string]any{
						"project_access": nil,
						"group_access":   map[string]any{"access_level": 40},
					},
				}},
				gitlabTokenSelf: {status: http.StatusOK, body: map[string]any{"scopes": []string{"api"}}},
			},
			expectedResult: AccessOk,
			expectedBranch: "main",
		},
		{
			name:     "gitlab reporter is read only",
			hostType: model.HostTypeGitLab,
			repoPath: "group/repo",
			routes: map[string]fakeHostResponse{
				gitlabProject: {status: http.StatusOK, body: map[string]any{
					"default_branch": "main",
					"permissions": map[string]any{
						"project_access": map[string]any{"access_level": 20},
					},
				}},
				gitlabTokenSelf: {status: http.StatusOK, body: map[string]any{"scopes": []string{"api"}}},
			},
			expectedResult: AccessReadOnly,
			expectedBranch: "main",
		},
		{
			name:     "gitlab token with read_api scope is read only",
			hostType: model.HostTypeGitLab,
			repoPath: "group/repo",
			routes: map[string]fakeHostResponse{
				gitlabProject: {status: http.StatusOK, body: map[string]any{
					"default_branch": "main",
					"permissions": map[string]any{
						"project_access": map[string]any{"access_level": 40},
					},
				}},
				gitlabTokenSelf: {status: http.StatusOK, body: map[string]any{"scopes": []string{"read_api"}}},
			},
			expectedResult: AccessReadOnly,
			expectedBranch: "main",
		},
		{
			name:     "gitlab without token scope endpoint trusts the access level",
			hostType: model.HostTypeGitLab,
			repoPath: "group/repo",
			routes: map[string]fakeHostResponse{
				gitlabProject: {status: http.StatusOK, body: map[string]any{
					"default_branch": "main",
					"permissions": map[string]any{
						"project_access": map[string]any{"access_level": 30},
					},
				}},
			},
			expectedResult: AccessOk,
			expectedBranch: "main",
		},
		{
			name:     "gitlab rejected token is invalid",
			hostType: model.HostTypeGitLab,
			repoPath: "group/repo",
			routes: map[string]fakeHostResponse{
				gitlabProject: {status: http.StatusUnauthorized},
			},
			expectedResult: AccessInvalidToken,
		},
		{
			name:     "gitea token with push access is ok",
			hostType: model.HostTypeGitea,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				giteaRepo: {status: http.StatusOK, body: map[string]any{
					"default_branch": "master",
					"permissions":    map[string]any{"push": true},
				}},
			},
			expectedResult: AccessOk,
			expectedBranch: "master",
		},
		{
			name:     "gitea user without push is read only",
			hostType: model.HostTypeGitea,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				giteaRepo: {status: http.StatusOK, body: map[string]any{
					"default_branch": "master",
					"permissions":    map[string]any{"push": false},
				}},
			},
			expectedResult: AccessReadOnly,
			expectedBranch: "master",
		},
		{
			name:     "gitea token missing a scope is forbidden",
			hostType: model.HostTypeGitea,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				giteaRepo: {status: http.StatusForbidden},
			},
			expectedResult: AccessRepoNotFound,
		},
		{
			name:     "gitea server error is unreachable",
			hostType: model.HostTypeGitea,
			repoPath: "owner/repo",
			routes: map[string]fakeHostResponse{
				giteaRepo: {status: http.StatusBadGateway},
			},
			expectedResult: AccessUnreachable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newFakeHost(t, tc.routes)

			check, err := CheckAccess(t.Context(), tc.hostType, srv.URL, tc.repoPath, "token")

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, check.Result)
			assert.Equal(t, tc.expectedBranch, check.DefaultBranch)
		})
	}
}

func TestCheckAccess_ClosedHostIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	baseUrl := srv.URL
	srv.Close()

	check, err := CheckAccess(t.Context(), model.HostTypeGitea, baseUrl, "owner/repo", "token")

	require.NoError(t, err)
	assert.Equal(t, AccessUnreachable, check.Result)
}

func TestCheckAccess_SendsTheToken(t *testing.T) {
	var receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := CheckAccess(t.Context(), model.HostTypeGitLab, srv.URL, "group/repo", "glpat-secret")

	require.NoError(t, err)
	assert.Equal(t, "glpat-secret", receivedToken)
}

func TestCheckAccess_UnsupportedHostType(t *testing.T) {
	_, err := CheckAccess(t.Context(), "bitbucket", "https://bitbucket.org", "owner/repo", "token")

	require.Error(t, err)
}
