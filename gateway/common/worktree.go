package common

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const agentRunsDir = ".agent-runs"

// Concurrent git ref updates on one repo fail with "cannot lock ref".
var worktreeMu sync.Mutex

// hookSourcePath is the pre-push hook baked into the image and installed into
// every cloned repo. A var, not const, so tests can point it at a fixture.
var hookSourcePath = "/etc/git-hooks/pre-push"

// Never put the token in the clone URL: git writes the URL to .git/config.
func EnsureRepo(workspaceBase string, repo AgentRunGitRepo) (string, error) {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

	repoPath, err := repo.LocalPath(workspaceBase)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
			return "", fmt.Errorf("creating workspace dir: %w", err)
		}
		log.Info().Str("url", repo.CloneUrl).Str("path", repoPath).Msg("cloning repo")
		if err := runRemoteGit(".", repo.Token, "clone", "-c", "credential.helper="+credentialHelper, repo.CloneUrl, repoPath); err != nil {
			return "", fmt.Errorf("cloning %s: %w", repo.CloneUrl, err)
		}
	} else {
		if err := runGit(repoPath, "remote", "set-url", "origin", repo.CloneUrl); err != nil {
			return "", fmt.Errorf("refreshing remote URL in %s: %w", repoPath, err)
		}
		if err := runGit(repoPath, "config", "--replace-all", "credential.helper", credentialHelper); err != nil {
			return "", fmt.Errorf("setting the credential helper in %s: %w", repoPath, err)
		}
	}

	// A brand-new (empty) repo has no branches: `fetch origin main` would
	// fatal with "couldn't find remote ref main".
	hasBase, err := remoteHasBranch(repoPath, repo.Token, repo.BaseBranch)
	if err != nil {
		return "", fmt.Errorf("checking base branch on origin in %s: %w", repoPath, err)
	}
	if !hasBase {
		empty, err := remoteIsEmpty(repoPath, repo.Token)
		if err != nil {
			return "", fmt.Errorf("checking whether origin is empty in %s: %w", repoPath, err)
		}
		if !empty {
			return "", fmt.Errorf(
				"base branch %q not found on origin in %s (set Base branch on the project's git integration)",
				repo.BaseBranch, repoPath,
			)
		}
		if err := seedEmptyRemote(repoPath, repo.Token, repo.BaseBranch); err != nil {
			return "", fmt.Errorf("seeding empty remote in %s: %w", repoPath, err)
		}
	}

	if err := InstallHooks(repoPath); err != nil {
		return "", fmt.Errorf("installing hooks in %s: %w", repoPath, err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, agentRunsDir), 0o755); err != nil {
		return "", fmt.Errorf("creating agent-runs dir: %w", err)
	}
	return repoPath, nil
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
func seedEmptyRemote(repoPath, token, branch string) error {
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
	if err := runRemoteGit(repoPath, token, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("pushing initial %s: %w", branch, err)
	}
	return nil
}

// remoteHasBranch reports whether origin has the given branch. `ls-remote
// --heads` exits 0 either way; a match prints a line, a miss prints nothing,
// so emptiness of stdout is the signal.
func remoteHasBranch(repoPath, token, branch string) (bool, error) {
	out, err := runRemoteGitOutput(repoPath, token, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// remoteIsEmpty reports whether origin has no branches at all (a brand-new repo).
func remoteIsEmpty(repoPath, token string) (bool, error) {
	out, err := runRemoteGitOutput(repoPath, token, "ls-remote", "--heads", "origin")
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
// The start point must stay an explicitly fetched origin/<baseBranch>: the base
// checkout's HEAD is only as new as the clone.
func CreateWorktree(repoPath, token, baseBranch, branch string, idRun int64) (string, error) {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

	worktreePath := WorktreePath(repoPath, idRun)
	if err := runRemoteGit(repoPath, token, "fetch", "origin", baseBranch); err != nil {
		return "", fmt.Errorf("fetching %s: %w", baseBranch, err)
	}
	if err := runGit(repoPath, "worktree", "add", worktreePath, "-b", branch, "origin/"+baseBranch); err != nil {
		return "", fmt.Errorf("creating worktree: %w", err)
	}
	return worktreePath, nil
}

func ResumeWorktree(repoPath, token, branch string, idRun int64) (string, error) {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

	worktreePath := WorktreePath(repoPath, idRun)
	if err := runRemoteGit(repoPath, token, "fetch", "origin", branch); err != nil {
		return "", fmt.Errorf("fetching %s: %w", branch, err)
	}
	if err := releaseBranch(repoPath, branch); err != nil {
		return "", err
	}
	if err := runGit(repoPath, "worktree", "add", "-B", branch, worktreePath, "origin/"+branch); err != nil {
		return "", fmt.Errorf("creating worktree on %s: %w", branch, err)
	}
	return worktreePath, nil
}

func releaseBranch(repoPath, branch string) error {
	if err := runGit(repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("pruning worktrees: %w", err)
	}
	out, err := runGitOutput(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}
	var currentPath string
	for _, line := range strings.Split(out, "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			currentPath = path
			continue
		}
		if line != "branch refs/heads/"+branch {
			continue
		}
		if err := runGit(currentPath, "checkout", "--detach"); err != nil {
			return fmt.Errorf("detaching %s from %s: %w", currentPath, branch, err)
		}
	}
	return nil
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
	cmd := newGitCmd(dir, nil, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// GenerateBranchName returns the branch name for a run. It must keep matching
// the pattern in git-hooks/pre-push, or the agent cannot push.
func GenerateBranchName(idUserAgent, idIssue int64) string {
	return fmt.Sprintf("agent/a%d/i%d/%d", idUserAgent, idIssue, time.Now().Unix())
}

// Must stay identical to the pattern in git-hooks/pre-push.
var agentBranchPattern = regexp.MustCompile(`^agent/a[0-9]+/i[0-9]+/[0-9]+$`)

func IsAgentBranch(branch string) bool {
	return agentBranchPattern.MatchString(branch)
}

func newGitCmd(dir string, env []string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}

func runGitCmd(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitCmdOutput returns stdout; stderr still streams to the gateway log so
// failures stay visible.
func gitCmdOutput(cmd *exec.Cmd) (string, error) {
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

func runGit(dir string, args ...string) error {
	return runGitCmd(newGitCmd(dir, nil, args...))
}

func runGitOutput(dir string, args ...string) (string, error) {
	return gitCmdOutput(newGitCmd(dir, nil, args...))
}

func runRemoteGit(dir, token string, args ...string) error {
	return runGitCmd(newGitCmd(dir, []string{GitTokenEnv(token)}, args...))
}

func runRemoteGitOutput(dir, token string, args ...string) (string, error) {
	return gitCmdOutput(newGitCmd(dir, []string{GitTokenEnv(token)}, args...))
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
