package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

func gitlabIntegration(t *testing.T, status int, body string) *model.GitIntegration {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &model.GitIntegration{HostType: model.HostTypeGitLab, BaseUrl: server.URL, RepoPath: "group/repo"}
}

func TestResolveBaseBranch(t *testing.T) {
	configured := "develop"
	tests := []struct {
		name        string
		integration *model.GitIntegration
		want        string
		wantErr     bool
	}{
		{"uses the base branch of the integration", &model.GitIntegration{HostType: model.HostTypeGitLab, BaseUrl: "http://unused.invalid", RepoPath: "group/repo", BaseBranch: &configured}, "develop", false},
		{"uses the default branch of the host", gitlabIntegration(t, http.StatusOK, `{"default_branch":"trunk"}`), "trunk", false},
		{"falls back to main for an empty repository", gitlabIntegration(t, http.StatusOK, `{"default_branch":null}`), "main", false},
		{"fails when the host cannot be read", gitlabIntegration(t, http.StatusUnauthorized, `{"message":"401 Unauthorized"}`), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := integrationBaseBranch(context.Background(), tt.integration, "token")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("integrationBaseBranch = %q, want an error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("integrationBaseBranch = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
