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

// GitHubHost implements GitHost for GitHub (github.com and GitHub Enterprise).
type GitHubHost struct {
	apiBase  string
	webBase  string
	repoPath string
	token    string
	client   *http.Client
}

func NewGitHubHost(baseUrl, repoPath, token string) *GitHubHost {
	normalized := strings.TrimRight(baseUrl, "/")
	var apiBase, webBase string
	if normalized == "https://github.com" {
		apiBase = "https://api.github.com"
		webBase = "https://github.com"
	} else {
		apiBase = normalized + "/api/v3"
		webBase = normalized
	}
	return &GitHubHost{
		apiBase:  apiBase,
		webBase:  webBase,
		repoPath: repoPath,
		token:    token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *GitHubHost) GetMergeRequestChanges(ctx context.Context, idMr string) (*Diff, error) {
	prURL := fmt.Sprintf("%s/repos/%s/pulls/%s", h.apiBase, h.repoPath, idMr)
	prReq, err := h.newRequest(ctx, prURL)
	if err != nil {
		return nil, err
	}
	prResp, err := doWithRetry(ctx, h.client, prReq)
	if err != nil {
		return nil, fmt.Errorf("fetching PR metadata: %w", err)
	}
	defer prResp.Body.Close()
	if prResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PR metadata: unexpected status %d", prResp.StatusCode)
	}
	var prData struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.NewDecoder(prResp.Body).Decode(&prData); err != nil {
		return nil, fmt.Errorf("decoding PR metadata: %w", err)
	}

	var files []DiffFile
	page := 1
	for page <= 10 {
		filesURL := fmt.Sprintf("%s/repos/%s/pulls/%s/files?per_page=100&page=%d", h.apiBase, h.repoPath, idMr, page)
		req, err := h.newRequest(ctx, filesURL)
		if err != nil {
			return nil, err
		}
		resp, err := doWithRetry(ctx, h.client, req)
		if err != nil {
			return nil, fmt.Errorf("fetching PR files page %d: %w", page, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("PR files page %d: unexpected status %d", page, resp.StatusCode)
		}

		var pageFiles []struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
			Patch            string `json:"patch"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&pageFiles); err != nil {
			return nil, fmt.Errorf("decoding PR files page %d: %w", page, err)
		}
		for _, f := range pageFiles {
			oldPath := f.PreviousFilename
			if oldPath == "" {
				oldPath = f.Filename
			}
			files = append(files, DiffFile{
				OldPath: oldPath,
				NewPath: f.Filename,
				Patch:   f.Patch,
			})
		}
		if len(pageFiles) < 100 || resp.Header.Get("Link") == "" || !strings.Contains(resp.Header.Get("Link"), `rel="next"`) {
			break
		}
		page++
	}

	return &Diff{HeadSHA: prData.Head.SHA, Files: files}, nil
}

func (h *GitHubHost) GetMergeRequestStatus(ctx context.Context, idMr string) (*Status, error) {
	prURL := fmt.Sprintf("%s/repos/%s/pulls/%s", h.apiBase, h.repoPath, idMr)
	req, err := h.newRequest(ctx, prURL)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return nil, fmt.Errorf("fetching PR status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PR status: unexpected status %d", resp.StatusCode)
	}

	var prData struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prData); err != nil {
		return nil, fmt.Errorf("decoding PR status: %w", err)
	}

	state := constants.MrStateClosed
	if prData.Merged {
		state = constants.MrStateMerged
	} else if prData.State == "open" {
		state = constants.MrStateOpen
	}

	ciStatus := constants.CiStatusUnknown
	if prData.Head.SHA != "" {
		ciStatus = h.getCiStatus(ctx, idMr, prData.Head.SHA)
	}

	approved := h.hasApproval(ctx, idMr)

	return &Status{State: state, Approved: approved, CiStatus: ciStatus, HeadSHA: prData.Head.SHA}, nil
}

func (h *GitHubHost) GetMergeRequestUrl(idMr string) string {
	return fmt.Sprintf("%s/%s/pull/%s", h.webBase, h.repoPath, idMr)
}

func (h *GitHubHost) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	h.setAuth(req)
	return req, nil
}

func (h *GitHubHost) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "token "+h.token)
	req.Header.Set("Accept", "application/vnd.github+json")
}

func (h *GitHubHost) DefaultBranch(ctx context.Context) (string, error) {
	repoURL := fmt.Sprintf("%s/repos/%s", h.apiBase, h.repoPath)
	req, err := h.newRequest(ctx, repoURL)
	if err != nil {
		return "", err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return "", fmt.Errorf("fetching repo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("repo: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	var data struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decoding repo: %w", err)
	}
	if data.DefaultBranch == "" {
		return "", fmt.Errorf("repo %s has no default_branch", h.repoPath)
	}
	return data.DefaultBranch, nil
}

func (h *GitHubHost) FindOpenPullRequest(ctx context.Context, headBranch string) (string, string, bool, error) {
	owner := h.repoPath
	if i := strings.Index(owner, "/"); i >= 0 {
		owner = owner[:i]
	}
	listURL := fmt.Sprintf("%s/repos/%s/pulls?head=%s:%s&state=open",
		h.apiBase, h.repoPath, url.QueryEscape(owner), url.QueryEscape(headBranch))
	req, err := h.newRequest(ctx, listURL)
	if err != nil {
		return "", "", false, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return "", "", false, fmt.Errorf("listing PRs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("listing PRs: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	var prs []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return "", "", false, fmt.Errorf("decoding PR list: %w", err)
	}
	if len(prs) == 0 {
		return "", "", false, nil
	}
	return strconv.Itoa(prs[0].Number), prs[0].HTMLURL, true, nil
}

func (h *GitHubHost) CreatePullRequest(ctx context.Context, head, base, title, body string) (string, string, error) {
	createURL := fmt.Sprintf("%s/repos/%s/pulls", h.apiBase, h.repoPath)
	payload := map[string]string{"title": title, "head": head, "base": base, "body": body}
	resp, err := postJSON(ctx, h.client, createURL, payload, h.setAuth)
	if err != nil {
		return "", "", fmt.Errorf("creating PR: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("creating PR: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	var data struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", fmt.Errorf("decoding created PR: %w", err)
	}
	return strconv.Itoa(data.Number), data.HTMLURL, nil
}
