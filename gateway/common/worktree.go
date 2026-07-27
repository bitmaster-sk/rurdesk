package common

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const agentRunsDir = ".agent-runs"

// hookSourcePath is the pre-push hook baked into the image and installed into
// every cloned repo. A var, not const, so tests can point it at a fixture.
var hookSourcePath = "/etc/git-hooks/pre-push"

// CloneRepo clones cfg.RepoUrl into <cfg.WorkspaceBase>/<repo-name>/.
// If already cloned, pulls latest from the base branch.
//
// One repo per gateway: a run's worktree and the repo it reports back to the
// tracker both come from this clone. A project may hold several git
// integrations, but nothing today says which repo a given issue belongs to, so
// serving more than one repo means running another gateway with another bot.
func CloneRepo(cfg *Config) error {
	if err := os.MkdirAll(cfg.WorkspaceBase, 0o755); err != nil {
		return fmt.Errorf("creating workspace base: %w", err)
	}

	{
		rawURL := cfg.RepoUrl
		repoName := repoNameFromURL(rawURL)
		repoPath := filepath.Join(cfg.WorkspaceBase, repoName)
		authenticatedURL := injectToken(rawURL, cfg.GitAccessToken)

		if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
			log.Info().Str("url", rawURL).Str("path", repoPath).Msg("cloning repo")
			if err := runGit(".", "clone", authenticatedURL, repoPath); err != nil {
				return fmt.Errorf("cloning %s: %w", rawURL, err)
			}
		} else {
			log.Info().Str("path", repoPath).Str("branch", cfg.RepoBranchBase).Msg("pulling repo")
			// Refresh the stored remote URL with the current PAT — the workspace
			// is a persistent volume, so a stale token would otherwise survive
			// restarts and break both the pull here and the agent's push.
			if err := runGit(repoPath, "remote", "set-url", "origin", authenticatedURL); err != nil {
				return fmt.Errorf("refreshing remote URL in %s: %w", repoPath, err)
			}
		}

		// A brand-new (empty) repo has no branches: `fetch origin main` would
		// fatal with "couldn't find remote ref main" and take the gateway down.
		hasBase, err := remoteHasBranch(repoPath, cfg.RepoBranchBase)
		if err != nil {
			return fmt.Errorf("checking base branch on origin in %s: %w", repoPath, err)
		}
		if hasBase {
			if err := syncBaseToOrigin(repoPath, cfg.RepoBranchBase); err != nil {
				return err
			}
		} else {
			empty, err := remoteIsEmpty(repoPath)
			if err != nil {
				return fmt.Errorf("checking whether origin is empty in %s: %w", repoPath, err)
			}
			if !empty {
				// Branches exist, just not the configured base one — a genuine
				// misconfiguration (e.g. the repo's default is `master`). Don't
				// guess; fail with a message pointing at the actual cause.
				return fmt.Errorf(
					"base branch %q not found on origin in %s (the repo has other branches — set REPO_BRANCH_BASE to the correct default)",
					cfg.RepoBranchBase, repoPath,
				)
			}
			if err := seedEmptyRemote(repoPath, cfg.RepoBranchBase); err != nil {
				return fmt.Errorf("seeding empty remote in %s: %w", repoPath, err)
			}
		}

		if err := InstallHooks(repoPath); err != nil {
			return fmt.Errorf("installing hooks in %s: %w", repoPath, err)
		}

		agentRunsPath := filepath.Join(repoPath, agentRunsDir)
		if err := os.MkdirAll(agentRunsPath, 0o755); err != nil {
			return fmt.Errorf("creating agent-runs dir: %w", err)
		}
	}
	return nil
}

// syncBaseToOrigin forces the local base checkout back to origin's base
// branch. The checkout is a disposable mirror — agents only touch per-run
// worktrees under .agent-runs/, never this tree — but a persistent volume can
// still carry leftover edits or untracked files (crashed run, manual poke,
// merged PR whose new files linger untracked), which would abort a plain
// `pull --ff-only`. Force instead.
func syncBaseToOrigin(repoPath, branch string) error {
	originRef := "origin/" + branch
	if err := runGit(repoPath, "fetch", "origin", branch); err != nil {
		return fmt.Errorf("fetching %s in %s: %w", branch, repoPath, err)
	}
	// Reset first (clobbers tracked edits and files origin now tracks that the
	// volume still holds untracked), then drop remaining untracked leftovers,
	// preserving the per-run worktree tree. Branch pointer moves only once
	// the tree is clean, so no checkout can collide.
	if err := runGit(repoPath, "reset", "--hard", originRef); err != nil {
		return fmt.Errorf("resetting %s in %s: %w", branch, repoPath, err)
	}
	if err := runGit(repoPath, "clean", "-fd", "-e", agentRunsDir); err != nil {
		return fmt.Errorf("cleaning %s: %w", repoPath, err)
	}
	if err := runGit(repoPath, "checkout", "-B", branch, originRef); err != nil {
		return fmt.Errorf("checkout %s in %s: %w", branch, repoPath, err)
	}
	return nil
}

// seedInitialCommitIdentity is the author/committer for the empty initial
// commit made when seeding a brand-new remote. The image has no global git
// identity (only agents commit, inside their worktrees), so it's passed
// inline rather than relying on git config.
const (
	seedCommitName  = "issue-tracker"
	seedCommitEmail = "gateway@issue-tracker.local"
)

// seedEmptyRemote initialises a completely empty origin with a base branch so
// the gateway can start and agents have something to branch from: point the
// unborn HEAD at the base branch, commit empty, push.
func seedEmptyRemote(repoPath, branch string) error {
	log.Info().Str("path", repoPath).Str("branch", branch).Msg("origin is empty — seeding initial branch")
	if err := runGit(repoPath, "symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
		return fmt.Errorf("pointing HEAD at %s: %w", branch, err)
	}
	if err := runGit(repoPath,
		"-c", "user.name="+seedCommitName,
		"-c", "user.email="+seedCommitEmail,
		"commit", "--allow-empty", "-m", "Initial commit",
	); err != nil {
		return fmt.Errorf("creating initial commit: %w", err)
	}
	if err := runGit(repoPath, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("pushing initial %s: %w", branch, err)
	}
	return nil
}

// remoteHasBranch reports whether origin has the given branch. `ls-remote
// --heads` exits 0 either way; a match prints a line, a miss prints nothing,
// so emptiness of stdout is the signal.
func remoteHasBranch(repoPath, branch string) (bool, error) {
	out, err := runGitOutput(repoPath, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// remoteIsEmpty reports whether origin has no branches at all (a brand-new repo).
func remoteIsEmpty(repoPath string) (bool, error) {
	out, err := runGitOutput(repoPath, "ls-remote", "--heads", "origin")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// InstallHooks copies the pre-push hook into the repo's .git/hooks/ directory.
func InstallHooks(repoPath string) error {
	hooksDir := filepath.Join(repoPath, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks dir: %w", err)
	}

	dest := filepath.Join(hooksDir, "pre-push")
	if err := copyFile(hookSourcePath, dest); err != nil {
		return fmt.Errorf("copying pre-push hook: %w", err)
	}
	return os.Chmod(dest, 0o755)
}

// CreateWorktree creates a new git worktree for a run. Returns the worktree path.
func CreateWorktree(repoPath, branch string, idRun int64) (string, error) {
	worktreePath := WorktreePath(repoPath, idRun)
	if err := runGit(repoPath, "worktree", "add", worktreePath, "-b", branch); err != nil {
		return "", fmt.Errorf("creating worktree: %w", err)
	}
	return worktreePath, nil
}

// WorktreePath returns the deterministic on-disk path for a run's worktree.
func WorktreePath(repoPath string, idRun int64) string {
	return filepath.Join(repoPath, agentRunsDir, fmt.Sprintf("%d", idRun))
}

// WorktreeExists reports whether a worktree already exists for the run. Used
// by the orchestrator to detect a re-enqueue and skip worktree creation +
// the queued→pickup transition.
func WorktreeExists(repoPath string, idRun int64) bool {
	info, err := os.Stat(WorktreePath(repoPath, idRun))
	return err == nil && info.IsDir()
}

// RemoveWorktree removes a git worktree, falling back to a plain directory
// removal when git doesn't know about it (orphaned admin metadata is common —
// .git/worktrees pruned but agent-runs dir survived, or vice versa). Git's
// stderr is discarded so retention sweeps don't spam the log with "fatal:
// ... is not a working tree"; the directory removal is what matters.
func RemoveWorktree(repoPath string, idRun int64) error {
	worktreePath := filepath.Join(repoPath, agentRunsDir, fmt.Sprintf("%d", idRun))
	_ = runGitQuiet(repoPath, "worktree", "remove", "--force", worktreePath)
	// Prune dead admin metadata so the next `git worktree list` is clean.
	// Errors here are non-fatal — the directory removal is the goal.
	_ = runGitQuiet(repoPath, "worktree", "prune")
	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("removing worktree directory: %w", err)
	}
	return nil
}

// runGitQuiet runs git discarding stdout/stderr, for cleanup paths where
// git's complaints (e.g. "not a working tree" after metadata was already
// cleared) are expected and harmless.
func runGitQuiet(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// GenerateBranchName returns a branch name matching the agent pattern.
func GenerateBranchName(idUserBot, idIssue int64) string {
	return fmt.Sprintf("agent/b%d/i%d/%d", idUserBot, idIssue, time.Now().Unix())
}

// RepoPathFromURL returns the local filesystem path for a given repo URL.
func RepoPathFromURL(workspaceBase, rawURL string) string {
	return filepath.Join(workspaceBase, repoNameFromURL(rawURL))
}

// RepoSlugFromURL extracts the "owner/repo" slug from a clone URL, used to
// tell the tracker which repo a run pushed to so it can resolve the matching
// git_integration. Supports HTTPS and SSH (git@host:owner/repo); GitLab
// subgroups keep their full path.
func RepoSlugFromURL(rawURL string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(rawURL), ".git")
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" && parsed.Path != "" {
		return strings.Trim(parsed.Path, "/")
	}
	// SSH-style git@host:owner/repo
	if i := strings.LastIndex(trimmed, ":"); i >= 0 {
		return strings.Trim(trimmed[i+1:], "/")
	}
	return strings.Trim(trimmed, "/")
}

func repoNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		parts := strings.Split(rawURL, "/")
		name := parts[len(parts)-1]
		return strings.TrimSuffix(name, ".git")
	}
	base := filepath.Base(parsed.Path)
	return strings.TrimSuffix(base, ".git")
}

// injectToken embeds the PAT into a clone/fetch URL. Git persists the URL
// (with credentials) into .git/config on clone, so callers must refresh it
// on subsequent pulls to handle token rotation.
func injectToken(rawURL, token string) string {
	if token == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = url.UserPassword("x-token", token)
	return parsed.String()
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runGitOutput runs git and returns its stdout; stderr still streams to the
// gateway log so failures stay visible.
func runGitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
