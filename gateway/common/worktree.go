package common

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const agentRunsDir = ".agent-runs"

// Concurrent git ref updates on one repo fail with "cannot lock ref".
// RemoveWorktree and PruneAgentBranches lock it themselves: never call them
// while holding it.
var worktreeMu sync.Mutex

// hookSourcePath is the pre-push hook baked into the image and installed into
// every cloned repo. A var, not const, so tests can point it at a fixture.
var hookSourcePath = "/etc/git-hooks/pre-push"

// ErrRemoteEmpty: origin has no branches yet and the caller may not seed it.
var ErrRemoteEmpty = errors.New("origin is empty")

// EnsureRepo clones or refreshes the repo, seeding an empty origin with its
// base branch.
func EnsureRepo(workspaceBase string, repo AgentRunGitRepo) (string, error) {
	return prepareRepo(workspaceBase, repo, true)
}

// EnsureRepoReadOnly is EnsureRepo for a chat, which never pushes: an empty
// origin fails with ErrRemoteEmpty instead of being seeded.
func EnsureRepoReadOnly(workspaceBase string, repo AgentRunGitRepo) (string, error) {
	return prepareRepo(workspaceBase, repo, false)
}

// Never put the token in the clone URL: git writes the URL to .git/config.
func prepareRepo(workspaceBase string, repo AgentRunGitRepo, canSeed bool) (string, error) {
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
		// The empty helper drops the helpers of the system and global config, so
		// none of them (a store, cache or keychain) is handed the token.
		if err := runRemoteGit(".", repo.Token, "clone", "-c", "credential.helper=", "-c", "credential.helper="+credentialHelper, repo.CloneUrl, repoPath); err != nil {
			return "", fmt.Errorf("cloning %s: %w", repo.CloneUrl, err)
		}
	} else if err := runGit(repoPath, "remote", "set-url", "origin", repo.CloneUrl); err != nil {
		return "", fmt.Errorf("refreshing remote URL in %s: %w", repoPath, err)
	}
	if err := setCredentialHelper(repoPath); err != nil {
		return "", err
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
		if !canSeed {
			return "", ErrRemoteEmpty
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

// setCredentialHelper makes the clone's own helper the only one: the empty
// entry resets the list, so helpers of the system or global config never see
// the token, neither to answer nor to store it.
func setCredentialHelper(repoPath string) error {
	if err := runGit(repoPath, "config", "--replace-all", "credential.helper", ""); err != nil {
		return fmt.Errorf("resetting the credential helpers in %s: %w", repoPath, err)
	}
	if err := runGit(repoPath, "config", "--add", "credential.helper", credentialHelper); err != nil {
		return fmt.Errorf("setting the credential helper in %s: %w", repoPath, err)
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
	// git refuses to add a worktree at a registered path an operator deleted.
	if err := runGit(repoPath, "worktree", "prune"); err != nil {
		return "", fmt.Errorf("pruning worktrees: %w", err)
	}
	if err := runGit(repoPath, "worktree", "add", worktreePath, "-b", branch, "origin/"+baseBranch); err != nil {
		return "", fmt.Errorf("creating worktree: %w", err)
	}
	return worktreePath, nil
}

// CreateDetachedWorktree creates the run's worktree on a detached HEAD at the
// freshly fetched origin/<baseBranch> and returns that commit. The token
// is used only by the fetch and never lands in the clone's config.
func CreateDetachedWorktree(repoPath, token, baseBranch string, idRun int64) (headSha string, err error) {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

	worktreePath := WorktreePath(repoPath, idRun)
	if err := runRemoteGit(repoPath, token, "fetch", "origin", baseBranch); err != nil {
		return "", fmt.Errorf("fetching %s: %w", baseBranch, err)
	}
	if err := runGit(repoPath, "worktree", "prune"); err != nil {
		return "", fmt.Errorf("pruning worktrees: %w", err)
	}
	if err := runGit(repoPath, "worktree", "add", "--detach", worktreePath, "origin/"+baseBranch); err != nil {
		return "", fmt.Errorf("creating detached worktree: %w", err)
	}
	return worktreeHead(worktreePath)
}

// untrustedWorktreeConfig keeps the gateway's own git calls in an agent-touched
// worktree from running hooks or an fsmonitor command the agent planted.
var untrustedWorktreeConfig = []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false"}

// ResetDetachedWorktree puts a reused worktree back on a detached HEAD at head, dropping every
// change, commit, branch checkout and untracked file the agent left. Nothing is fetched; it fails
// when head is empty or unknown or the worktree cannot be restored.
func ResetDetachedWorktree(worktreePath, head string) error {
	if head == "" {
		return errors.New("no recorded head commit")
	}
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

	// Run nothing in a directory that is not the worktree root: git would act on
	// the clone around it.
	if _, err := worktreeHead(worktreePath); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"cat-file", "-e", head + "^{commit}"},
		{"reset", "--hard"},
		{"checkout", "--force", "--detach", head},
		{"clean", "-ffdx"},
	} {
		if err := runGitQuiet(worktreePath, append(slices.Clone(untrustedWorktreeConfig), args...)...); err != nil {
			return fmt.Errorf("git %s in %s: %w", strings.Join(args, " "), worktreePath, err)
		}
	}
	if branch := rebasingBranch(worktreePath); branch != "" {
		return fmt.Errorf("%s is in the middle of a rebase of %s", worktreePath, branch)
	}
	reset, err := worktreeHead(worktreePath)
	if err != nil {
		return err
	}
	if reset != head {
		return fmt.Errorf("%s is at %s after the reset, want %s", worktreePath, reset, head)
	}
	return nil
}

// worktreeHead returns the commit checked out in the worktree rooted at
// worktreePath, failing when that directory is not a worktree root.
func worktreeHead(worktreePath string) (string, error) {
	topLevel, err := runGitQuietOutput(worktreePath, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("reading the worktree root: %w", err)
	}
	isRoot, err := isSamePath(strings.TrimSpace(topLevel), worktreePath)
	if err != nil {
		return "", err
	}
	if !isRoot {
		return "", fmt.Errorf("%s is not a git worktree", worktreePath)
	}
	head, err := runGitQuietOutput(worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("reading the worktree HEAD: %w", err)
	}
	return strings.TrimSpace(head), nil
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

// worktreeState fails when the directory is not a worktree root. branch is the
// checked-out branch or the one a paused rebase rewrites; isDisposable is set
// only for a detached HEAD with no uncommitted changes and no commits outside
// branches and remotes.
func worktreeState(worktreePath string) (branch string, isDisposable bool, err error) {
	topLevel, err := runGitQuietOutput(worktreePath, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false, fmt.Errorf("reading the worktree root: %w", err)
	}
	isRoot, err := isSamePath(strings.TrimSpace(topLevel), worktreePath)
	if err != nil {
		return "", false, err
	}
	if !isRoot {
		return "", false, fmt.Errorf("%s is not a git worktree", worktreePath)
	}
	branch, err = branchOfWorktree(worktreePath)
	if err != nil {
		return "", false, fmt.Errorf("reading the worktree branch: %w", err)
	}
	if branch == "" {
		branch = rebasingBranch(worktreePath)
	}
	if branch != "" {
		return branch, false, nil
	}
	status, err := runGitQuietOutput(worktreePath, "status", "--porcelain")
	if err != nil {
		return "", false, fmt.Errorf("reading the worktree status: %w", err)
	}
	uniqueCommits, err := runGitQuietOutput(worktreePath, "rev-list", "HEAD", "--not", "--branches", "--remotes")
	if err != nil {
		return "", false, fmt.Errorf("listing commits outside branches: %w", err)
	}
	return "", strings.TrimSpace(status) == "" && strings.TrimSpace(uniqueCommits) == "", nil
}

// rebasingBranch returns the branch a paused rebase in the worktree rewrites,
// or "" when no rebase is in progress.
func rebasingBranch(worktreePath string) string {
	for _, stateDir := range []string{"rebase-merge", "rebase-apply"} {
		headNamePath, err := runGitQuietOutput(worktreePath, "rev-parse", "--git-path", stateDir+"/head-name")
		if err != nil {
			continue
		}
		headNamePath = strings.TrimSpace(headNamePath)
		if !filepath.IsAbs(headNamePath) {
			headNamePath = filepath.Join(worktreePath, headNamePath)
		}
		headName, err := os.ReadFile(headNamePath)
		if err != nil {
			continue
		}
		if branch, ok := strings.CutPrefix(strings.TrimSpace(string(headName)), "refs/heads/"); ok {
			return branch
		}
	}
	return ""
}

func isSamePath(first, second string) (bool, error) {
	resolvedFirst, err := filepath.EvalSymlinks(first)
	if err != nil {
		return false, err
	}
	resolvedSecond, err := filepath.EvalSymlinks(second)
	if err != nil {
		return false, err
	}
	return resolvedFirst == resolvedSecond, nil
}

// RemoveWorktree removes a git worktree, falling back to a plain directory
// removal when git doesn't know about it (orphaned admin metadata is common —
// .git/worktrees pruned but agent-runs dir survived, or vice versa). Git's
// stderr is discarded so retention sweeps don't spam the log with "fatal:
// ... is not a working tree"; the directory removal is what matters.
func RemoveWorktree(repoPath string, idRun int64) error {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

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

// PruneAgentBranches deletes local agent branches that no worktree has checked
// out: right away when all their commits are on a remote (pushed, or never
// committed to), otherwise once their tip is older than unpushedCutoff.
func PruneAgentBranches(repoPath string, unpushedCutoff time.Time) error {
	worktreeMu.Lock()
	defer worktreeMu.Unlock()

	if err := runGitQuiet(repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("pruning worktrees: %w", err)
	}
	checkedOut, err := checkedOutBranches(repoPath)
	if err != nil {
		return err
	}
	refs, err := runGitOutput(repoPath, "for-each-ref", "--format=%(refname:short) %(committerdate:unix)", "refs/heads/agent/")
	if err != nil {
		return fmt.Errorf("listing agent branches: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(refs), "\n") {
		branch, tipUnix, found := strings.Cut(line, " ")
		if !found || !IsAgentBranch(branch) || checkedOut[branch] {
			continue
		}
		isDeletable, err := isAgentBranchDeletable(repoPath, branch, tipUnix, unpushedCutoff)
		if err != nil {
			log.Warn().Str("repo", repoPath).Str("branch", branch).Err(err).Msg("retention: cannot inspect agent branch")
			continue
		}
		if !isDeletable {
			continue
		}
		if err := runGitQuiet(repoPath, "branch", "-D", branch); err != nil {
			log.Warn().Str("repo", repoPath).Str("branch", branch).Err(err).Msg("retention: cannot delete agent branch")
			continue
		}
		log.Info().Str("repo", repoPath).Str("branch", branch).Msg("retention: deleted agent branch")
	}
	return nil
}

func isAgentBranchDeletable(repoPath, branch, tipUnix string, unpushedCutoff time.Time) (bool, error) {
	localOnly, err := runGitOutput(repoPath, "rev-list", "refs/heads/"+branch, "--not", "--remotes")
	if err != nil {
		return false, fmt.Errorf("listing commits on no remote: %w", err)
	}
	if strings.TrimSpace(localOnly) == "" {
		return true, nil
	}
	tipSeconds, err := strconv.ParseInt(tipUnix, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parsing tip date %q: %w", tipUnix, err)
	}
	return time.Unix(tipSeconds, 0).Before(unpushedCutoff), nil
}

func checkedOutBranches(repoPath string) (map[string]bool, error) {
	out, err := runGitOutput(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}
	branches := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if branch, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			branches[branch] = true
		}
	}
	return branches, nil
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

// runGitQuietOutput returns stdout and discards stderr, for probes whose
// failure is an expected answer rather than something to log.
func runGitQuietOutput(dir string, args ...string) (string, error) {
	cmd := newGitCmd(dir, nil, args...)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	return string(out), err
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
