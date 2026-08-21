package githost

import "context"

// GitHost provides access to merge/pull request data on a git hosting platform.
type GitHost interface {
	GetMergeRequestChanges(ctx context.Context, idMr string) (*Diff, error)
	GetMergeRequestStatus(ctx context.Context, idMr string) (*Status, error)
	GetMergeRequestUrl(idMr string) string

	// DefaultBranch returns the repository's default branch (PR base).
	DefaultBranch(ctx context.Context) (string, error)
	// FindOpenPullRequest looks up an open PR/MR whose source/head is headBranch.
	// Used for idempotency so a re-invoked run reuses its existing PR.
	FindOpenPullRequest(ctx context.Context, headBranch string) (prId, prUrl string, found bool, err error)
	// CreatePullRequest opens a PR/MR from head into base and returns the host's
	// authoritative id + web URL.
	CreatePullRequest(ctx context.Context, head, base, title, body string) (prId, prUrl string, err error)
}

type Diff struct {
	HeadSHA string     `json:"headSha"`
	Files   []DiffFile `json:"files"`
}

type DiffFile struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
	Patch   string `json:"patch"`
}

type Status struct {
	State    string `json:"state"`
	Approved bool   `json:"approved"`
	CiStatus string `json:"ciStatus"`
	// HeadSHA is the current head commit of the MR/PR. It lets the diff cache
	// (keyed by head SHA) be read without an extra host call, and it always
	// matches the SHA GetMergeRequestChanges keys its diff under.
	HeadSHA string `json:"headSha"`
}
