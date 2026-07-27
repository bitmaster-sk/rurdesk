package githost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

// GitLabHost implements GitHost for GitLab (gitlab.com and self-hosted).
type GitLabHost struct {
	baseUrl     string
	encodedPath string
	repoPath    string
	token       string
	client      *http.Client
}

func NewGitLabHost(baseUrl, repoPath, token string) *GitLabHost {
	normalized := strings.TrimRight(baseUrl, "/")
	return &GitLabHost{
		baseUrl:     normalized,
		encodedPath: url.PathEscape(repoPath),
		repoPath:    repoPath,
		token:       token,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *GitLabHost) GetMergeRequestChanges(ctx context.Context, mrID string) (*Diff, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/changes", h.baseUrl, h.encodedPath, mrID)
	req, err := h.newRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return nil, fmt.Errorf("fetching MR changes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MR changes: unexpected status %d", resp.StatusCode)
	}

	var data struct {
		DiffRefs struct {
			HeadSHA string `json:"head_sha"`
		} `json:"diff_refs"`
		Changes []struct {
			OldPath string `json:"old_path"`
			NewPath string `json:"new_path"`
			Diff    string `json:"diff"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding MR changes: %w", err)
	}

	files := make([]DiffFile, 0, len(data.Changes))
	for _, c := range data.Changes {
		files = append(files, DiffFile{
			OldPath: c.OldPath,
			NewPath: c.NewPath,
			Patch:   c.Diff,
		})
	}
	return &Diff{HeadSHA: data.DiffRefs.HeadSHA, Files: files}, nil
}

func (h *GitLabHost) GetMergeRequestStatus(ctx context.Context, mrID string) (*Status, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s", h.baseUrl, h.encodedPath, mrID)
	req, err := h.newRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return nil, fmt.Errorf("fetching MR status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MR status: unexpected status %d", resp.StatusCode)
	}

	var mrData struct {
		State      string `json:"state"`
		ApprovedBy []any  `json:"approved_by"`
		DiffRefs   struct {
			HeadSHA string `json:"head_sha"`
		} `json:"diff_refs"`
		HeadPipeline *struct {
			Status string `json:"status"`
		} `json:"head_pipeline"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mrData); err != nil {
		return nil, fmt.Errorf("decoding MR status: %w", err)
	}

	state := constants.MrStateClosed
	switch mrData.State {
	case "opened":
		state = constants.MrStateOpen
	case "merged":
		state = constants.MrStateMerged
	}

	ciStatus := constants.CiStatusUnknown
	if mrData.HeadPipeline != nil {
		switch mrData.HeadPipeline.Status {
		case "success":
			ciStatus = constants.CiStatusSuccess
		case "failed":
			ciStatus = constants.CiStatusFailed
		case "running", "pending":
			ciStatus = constants.CiStatusPending
		}
	}

	approved := len(mrData.ApprovedBy) > 0

	return &Status{State: state, Approved: approved, CiStatus: ciStatus, HeadSHA: mrData.DiffRefs.HeadSHA}, nil
}

func (h *GitLabHost) GetMergeRequestUrl(mrID string) string {
	return fmt.Sprintf("%s/%s/-/merge_requests/%s", h.baseUrl, h.repoPath, mrID)
}

func (h *GitLabHost) newRequest(ctx context.Context, apiURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	h.setAuth(req)
	return req, nil
}

func (h *GitLabHost) setAuth(req *http.Request) {
	req.Header.Set("PRIVATE-TOKEN", h.token)
}

func (h *GitLabHost) DefaultBranch(ctx context.Context) (string, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s", h.baseUrl, h.encodedPath)
	req, err := h.newRequest(ctx, apiURL)
	if err != nil {
		return "", err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return "", fmt.Errorf("fetching project: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("project: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	var data struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decoding project: %w", err)
	}
	if data.DefaultBranch == "" {
		return "", fmt.Errorf("project %s has no default_branch", h.repoPath)
	}
	return data.DefaultBranch, nil
}

func (h *GitLabHost) FindOpenPullRequest(ctx context.Context, headBranch string) (string, string, bool, error) {
	listURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests?source_branch=%s&state=opened",
		h.baseUrl, h.encodedPath, url.QueryEscape(headBranch))
	req, err := h.newRequest(ctx, listURL)
	if err != nil {
		return "", "", false, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return "", "", false, fmt.Errorf("listing MRs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("listing MRs: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	var mrs []struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return "", "", false, fmt.Errorf("decoding MR list: %w", err)
	}
	if len(mrs) == 0 {
		return "", "", false, nil
	}
	return strconv.Itoa(mrs[0].IID), mrs[0].WebURL, true, nil
}

func (h *GitLabHost) CreatePullRequest(ctx context.Context, head, base, title, body string) (string, string, error) {
	createURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", h.baseUrl, h.encodedPath)
	payload := map[string]string{
		"source_branch": head,
		"target_branch": base,
		"title":         title,
		"description":   body,
	}
	resp, err := postJSON(ctx, h.client, createURL, payload, h.setAuth)
	if err != nil {
		return "", "", fmt.Errorf("creating MR: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("creating MR: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	var data struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("decoding created MR: %w", err)
	}
	return strconv.Itoa(data.IID), data.WebURL, nil
}
