package githost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

// GiteaHost implements GitHost for Gitea instances.
type GiteaHost struct {
	baseUrl  string
	repoPath string
	token    string
	client   *http.Client
}

func NewGiteaHost(baseUrl, repoPath, token string) *GiteaHost {
	return &GiteaHost{
		baseUrl:  strings.TrimRight(baseUrl, "/"),
		repoPath: repoPath,
		token:    token,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *GiteaHost) GetMergeRequestChanges(ctx context.Context, mrID string) (*Diff, error) {
	prURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s", h.baseUrl, h.repoPath, mrID)
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
	for page <= 20 {
		filesURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s/files?limit=50&page=%d", h.baseUrl, h.repoPath, mrID, page)
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
		if len(pageFiles) < 50 {
			break
		}
		page++
	}

	return &Diff{HeadSHA: prData.Head.SHA, Files: files}, nil
}

func (h *GiteaHost) GetMergeRequestStatus(ctx context.Context, mrID string) (*Status, error) {
	prURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s", h.baseUrl, h.repoPath, mrID)
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

	return &Status{State: state, Approved: false, CiStatus: constants.CiStatusUnknown, HeadSHA: prData.Head.SHA}, nil
}

func (h *GiteaHost) GetMergeRequestUrl(mrID string) string {
	return fmt.Sprintf("%s/%s/pulls/%s", h.baseUrl, h.repoPath, mrID)
}

func (h *GiteaHost) newRequest(ctx context.Context, apiURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	h.setAuth(req)
	return req, nil
}

func (h *GiteaHost) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "token "+h.token)
}

func (h *GiteaHost) DefaultBranch(ctx context.Context) (string, error) {
	repoURL := fmt.Sprintf("%s/api/v1/repos/%s", h.baseUrl, h.repoPath)
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

const (
	giteaPullsPageSize = 50
	giteaPullsMaxPages = 20
)

// FindOpenPullRequest walks the open-PR list for one whose head is headBranch.
//
// Gitea's pulls endpoint has no head filter (GitHub and GitLab do), so the match
// happens client-side over the whole list. Paging is not optional: missing a PR
// past the first page makes openOrReusePr create a duplicate.
func (h *GiteaHost) FindOpenPullRequest(ctx context.Context, headBranch string) (string, string, bool, error) {
	for page := 1; page <= giteaPullsMaxPages; page++ {
		prs, err := h.listOpenPullRequests(ctx, page)
		if err != nil {
			return "", "", false, err
		}
		for _, pr := range prs {
			if pr.Head.Ref == headBranch {
				return strconv.Itoa(pr.Number), pr.HTMLURL, true, nil
			}
		}
		// A short page is the last page; a full one may not be.
		if len(prs) < giteaPullsPageSize {
			return "", "", false, nil
		}
	}
	return "", "", false, nil
}

type giteaPullRef struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Head    struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

func (h *GiteaHost) listOpenPullRequests(ctx context.Context, page int) ([]giteaPullRef, error) {
	listURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls?state=open&limit=%d&page=%d",
		h.baseUrl, h.repoPath, giteaPullsPageSize, page)
	req, err := h.newRequest(ctx, listURL)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return nil, fmt.Errorf("listing PRs page %d: %w", page, err)
	}
	// Closed here rather than deferred: this runs in a loop, and deferring would
	// hold every page's body open until the whole walk finishes.
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing PRs page %d: unexpected status %d: %s", page, resp.StatusCode, readBody(resp))
	}
	var prs []giteaPullRef
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, fmt.Errorf("decoding PR list page %d: %w", page, err)
	}
	return prs, nil
}

func (h *GiteaHost) CreatePullRequest(ctx context.Context, head, base, title, body string) (string, string, error) {
	createURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls", h.baseUrl, h.repoPath)
	payload := map[string]string{"head": head, "base": base, "title": title, "body": body}
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
