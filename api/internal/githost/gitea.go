package githost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

const MaxDiffBytes = 20 << 20

var ErrDiffTooLarge = errors.New("diff exceeds maximum allowed size")

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

func (h *GiteaHost) GetMergeRequestChanges(ctx context.Context, idMr string) (*Diff, error) {
	prURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s", h.baseUrl, h.repoPath, idMr)
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

	diffURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s.diff", h.baseUrl, h.repoPath, idMr)
	req, err := h.newRequest(ctx, diffURL)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(ctx, h.client, req)
	if err != nil {
		return nil, fmt.Errorf("fetching PR diff: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PR diff: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxDiffBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading PR diff: %w", err)
	}
	if len(raw) > MaxDiffBytes {
		return nil, ErrDiffTooLarge
	}
	files := parseGiteaDiff(string(raw))

	return &Diff{HeadSHA: prData.Head.SHA, Files: files}, nil
}

const giteaDiffHeader = "diff --git "

func parseGiteaDiff(raw string) []DiffFile {
	var files []DiffFile
	var section []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, giteaDiffHeader) {
			if section != nil {
				files = append(files, parseGiteaDiffFile(section))
			}
			section = []string{}
		}
		if section != nil {
			section = append(section, line)
		}
	}
	if section != nil {
		files = append(files, parseGiteaDiffFile(section))
	}
	return files
}

func parseGiteaDiffFile(lines []string) DiffFile {
	var oldPath, newPath, renameFrom, renameTo string
	hunkStart := len(lines)
	for i, line := range lines[1:] {
		if strings.HasPrefix(line, "@@") {
			hunkStart = i + 1
			break
		}
		switch {
		case strings.HasPrefix(line, "--- "):
			oldPath = giteaMarkerPath(strings.TrimPrefix(line, "--- "), "a/")
		case strings.HasPrefix(line, "+++ "):
			newPath = giteaMarkerPath(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "rename from "), strings.HasPrefix(line, "copy from "):
			renameFrom = unquoteGitPath(line[strings.Index(line, " from ")+len(" from "):])
		case strings.HasPrefix(line, "rename to "), strings.HasPrefix(line, "copy to "):
			renameTo = unquoteGitPath(line[strings.Index(line, " to ")+len(" to "):])
		}
	}

	if renameFrom != "" && renameTo != "" {
		oldPath, newPath = renameFrom, renameTo
	}
	if oldPath == "" {
		oldPath = newPath
	}
	if newPath == "" {
		newPath = oldPath
	}
	if oldPath == "" {
		oldPath = giteaHeaderPath(strings.TrimPrefix(lines[0], giteaDiffHeader))
		newPath = oldPath
	}

	patch := strings.TrimRight(strings.Join(lines[hunkStart:], "\n"), "\n")
	return DiffFile{OldPath: oldPath, NewPath: newPath, Patch: patch}
}

func giteaMarkerPath(s, prefix string) string {
	s = strings.TrimSuffix(s, "\t")
	if s == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(unquoteGitPath(s), prefix)
}

// Only reached when the header names the same file twice, so either half is the path.
func giteaHeaderPath(header string) string {
	if quoted, err := strconv.QuotedPrefix(header); err == nil && strings.HasPrefix(header, `"`) {
		return strings.TrimPrefix(unquoteGitPath(quoted), "a/")
	}
	return strings.TrimPrefix(header[:(len(header)-1)/2], "a/")
}

func unquoteGitPath(s string) string {
	if !strings.HasPrefix(s, `"`) {
		return s
	}
	if unquoted, err := strconv.Unquote(s); err == nil {
		return unquoted
	}
	return s
}

func (h *GiteaHost) GetMergeRequestStatus(ctx context.Context, idMr string) (*Status, error) {
	prURL := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%s", h.baseUrl, h.repoPath, idMr)
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

	return &Status{State: state, Approved: approved, CiStatus: ciStatus, WebUrl: h.GetMergeRequestUrl(idMr), HeadSHA: prData.Head.SHA}, nil
}

func (h *GiteaHost) GetFileContent(ctx context.Context, path, ref string) ([]byte, error) {
	rawURL := fmt.Sprintf("%s/api/v1/repos/%s/raw/%s?ref=%s", h.baseUrl, h.repoPath, escapePathSegments(path), url.QueryEscape(ref))
	req, err := h.newRequest(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return fetchFileContent(ctx, h.client, req, path, ref)
}

func (h *GiteaHost) GetMergeRequestUrl(idMr string) string {
	return fmt.Sprintf("%s/%s/pulls/%s", h.baseUrl, h.repoPath, idMr)
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
		return "", fmt.Errorf("repo %s: %w", h.repoPath, ErrNoDefaultBranch)
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
