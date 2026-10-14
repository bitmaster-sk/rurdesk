package githost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

const (
	AccessOk           = "ok"
	AccessUnreachable  = "unreachable"
	AccessInvalidToken = "invalid_token"
	AccessRepoNotFound = "repo_not_found"
	AccessReadOnly     = "read_only"
)

const gitlabDeveloperAccessLevel = 30

type AccessCheck struct {
	Result        string `json:"result"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
}

type accessChecker interface {
	CheckAccess(ctx context.Context) (*AccessCheck, error)
}

func CheckAccess(ctx context.Context, hostType, baseUrl, repoPath, token string) (*AccessCheck, error) {
	host, err := NewGitHost(hostType, baseUrl, repoPath, token)
	if err != nil {
		return nil, err
	}
	checker, isChecker := host.(accessChecker)
	if !isChecker {
		return nil, fmt.Errorf("host type %s does not support access checks", hostType)
	}
	return checker.CheckAccess(ctx)
}

func fetchRepoForCheck(client *http.Client, req *http.Request, out any) (string, http.Header) {
	resp, err := client.Do(req)
	if err != nil {
		return AccessUnreachable, nil
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return AccessInvalidToken, resp.Header
	case http.StatusForbidden, http.StatusNotFound:
		return AccessRepoNotFound, resp.Header
	default:
		return AccessUnreachable, resp.Header
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return AccessUnreachable, resp.Header
	}
	return AccessOk, resp.Header
}

func writeAccessResult(canWrite bool, defaultBranch string) *AccessCheck {
	if !canWrite {
		return &AccessCheck{Result: AccessReadOnly, DefaultBranch: defaultBranch}
	}
	return &AccessCheck{Result: AccessOk, DefaultBranch: defaultBranch}
}

func (h *GitHubHost) CheckAccess(ctx context.Context) (*AccessCheck, error) {
	req, err := h.newRequest(ctx, fmt.Sprintf("%s/repos/%s", h.apiBase, h.repoPath))
	if err != nil {
		return nil, err
	}
	var repo struct {
		DefaultBranch string `json:"default_branch"`
		Permissions   struct {
			Push bool `json:"push"`
		} `json:"permissions"`
	}
	result, header := fetchRepoForCheck(h.client, req, &repo)
	if result != AccessOk {
		return &AccessCheck{Result: result}, nil
	}
	return writeAccessResult(repo.Permissions.Push && hasGitHubRepoScope(header), repo.DefaultBranch), nil
}

func hasGitHubRepoScope(header http.Header) bool {
	scopes := strings.TrimSpace(header.Get("X-OAuth-Scopes"))
	if scopes == "" {
		return true
	}
	for scope := range strings.SplitSeq(scopes, ",") {
		switch strings.TrimSpace(scope) {
		case "repo", "public_repo":
			return true
		}
	}
	return false
}

type gitlabAccess struct {
	AccessLevel int `json:"access_level"`
}

func (h *GitLabHost) CheckAccess(ctx context.Context) (*AccessCheck, error) {
	req, err := h.newRequest(ctx, fmt.Sprintf("%s/api/v4/projects/%s", h.baseUrl, h.encodedPath))
	if err != nil {
		return nil, err
	}
	var project struct {
		DefaultBranch string `json:"default_branch"`
		Permissions   struct {
			ProjectAccess *gitlabAccess `json:"project_access"`
			GroupAccess   *gitlabAccess `json:"group_access"`
		} `json:"permissions"`
	}
	result, _ := fetchRepoForCheck(h.client, req, &project)
	if result != AccessOk {
		return &AccessCheck{Result: result}, nil
	}
	accessLevel := 0
	for _, access := range []*gitlabAccess{project.Permissions.ProjectAccess, project.Permissions.GroupAccess} {
		if access != nil {
			accessLevel = max(accessLevel, access.AccessLevel)
		}
	}
	canWrite := accessLevel >= gitlabDeveloperAccessLevel && h.hasApiScope(ctx)
	return writeAccessResult(canWrite, project.DefaultBranch), nil
}

func (h *GitLabHost) hasApiScope(ctx context.Context) bool {
	var token struct {
		Scopes []string `json:"scopes"`
	}
	// Older GitLab versions lack this endpoint, so an error means unknown scopes, not missing ones.
	if err := getJSON(ctx, h.client, h.baseUrl+"/api/v4/personal_access_tokens/self", h.setAuth, &token); err != nil {
		return true
	}
	return slices.Contains(token.Scopes, "api")
}

func (h *GiteaHost) CheckAccess(ctx context.Context) (*AccessCheck, error) {
	req, err := h.newRequest(ctx, fmt.Sprintf("%s/api/v1/repos/%s", h.baseUrl, h.repoPath))
	if err != nil {
		return nil, err
	}
	var repo struct {
		DefaultBranch string `json:"default_branch"`
		Permissions   struct {
			Push bool `json:"push"`
		} `json:"permissions"`
	}
	result, _ := fetchRepoForCheck(h.client, req, &repo)
	if result != AccessOk {
		return &AccessCheck{Result: result}, nil
	}
	return writeAccessResult(repo.Permissions.Push, repo.DefaultBranch), nil
}
