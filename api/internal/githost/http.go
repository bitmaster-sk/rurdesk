package githost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const maxFileContentBytes = 1 << 20

// postJSON issues a single POST with a JSON body. The caller's setAuth applies
// host-specific auth headers (and any Accept header).
//
// Single attempt on purpose: creating a PR/MR is NOT idempotent, so a blind
// retry on a transient 5xx/network error could open duplicate PRs. Cross-run
// idempotency is handled by FindOpenPullRequest before any create.
func postJSON(ctx context.Context, client *http.Client, url string, body any, setAuth func(*http.Request)) (*http.Response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAuth(req)
	return client.Do(req)
}

func getJSON(ctx context.Context, client *http.Client, url string, setAuth func(*http.Request), out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	setAuth(req)
	resp, err := doWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: unexpected status %d: %s", url, resp.StatusCode, readBody(resp))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", url, err)
	}
	return nil
}

// readBody returns up to 2KB of the response body as a string for error context.
// Safe to call after a non-2xx; the caller still owns Body.Close().
func readBody(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return string(b)
}

var (
	ErrFileTooLarge = errors.New("file exceeds maximum allowed size")
	ErrBinaryFile   = errors.New("file appears to be binary")
)

func fetchFileContent(ctx context.Context, client *http.Client, req *http.Request, path, ref string) ([]byte, error) {
	resp, err := doWithRetry(ctx, client, req)
	if err != nil {
		return nil, fmt.Errorf("fetching file content %s@%s: %w", path, ref, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("file content %s@%s: unexpected status %d: %s", path, ref, resp.StatusCode, readBody(resp))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFileContentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading file content %s@%s: %w", path, ref, err)
	}
	if len(data) > maxFileContentBytes {
		return nil, fmt.Errorf("%s: %w", path, ErrFileTooLarge)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: %w", path, ErrBinaryFile)
	}
	return data, nil
}

func escapePathSegments(path string) string {
	segments := strings.Split(path, "/")
	for idx, segment := range segments {
		segments[idx] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
