package common

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// git runs a git command in dir, failing the test with combined output on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func newGitEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	tmp := t.TempDir()
	hookFixture := filepath.Join(tmp, "pre-push")
	if err := os.WriteFile(hookFixture, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := hookSourcePath
	hookSourcePath = hookFixture
	t.Cleanup(func() { hookSourcePath = orig })
	return tmp
}

func seedOrigin(t *testing.T, tmp, defaultBranch string) (originPath, seedPath string) {
	t.Helper()
	return seedNamedOrigin(t, tmp, "origin", defaultBranch)
}

func seedNamedOrigin(t *testing.T, tmp, name, defaultBranch string) (originPath, seedPath string) {
	t.Helper()
	originPath = filepath.Join(tmp, name+".git")
	git(t, tmp, "init", "--bare", "-b", defaultBranch, originPath)
	seedPath = filepath.Join(tmp, name+"-seed")
	git(t, tmp, "init", "-b", defaultBranch, seedPath)
	if err := os.WriteFile(filepath.Join(seedPath, "README.md"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seedPath, "add", ".")
	git(t, seedPath, "commit", "-m", "init")
	git(t, seedPath, "remote", "add", "origin", originPath)
	git(t, seedPath, "push", "origin", defaultBranch)
	return originPath, seedPath
}

const testGitToken = "test-git-token-1234567890"

func testRepo(originPath, host, baseBranch string) AgentRunGitRepo {
	return AgentRunGitRepo{
		CloneUrl:       originPath,
		Host:           host,
		RepoPath:       "acme/myrepo",
		BaseBranch:     baseBranch,
		EncryptedToken: encryptGitTokenForTest(testWebhookSecret, testGitToken),
		Token:          testGitToken,
	}
}

func ensureRepo(t *testing.T, workspaceBase string, repo AgentRunGitRepo) string {
	t.Helper()
	repoPath, err := EnsureRepo(workspaceBase, repo)
	if err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	return repoPath
}

func headSha(t *testing.T, dir string) string {
	t.Helper()
	out, err := runGitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD in %s: %v", dir, err)
	}
	return strings.TrimSpace(out)
}

func TestEnsureRepo_SeedsEmptyRemote(t *testing.T) {
	tmp := newGitEnv(t)
	originPath := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--bare", "-b", "main", originPath)
	workspaceBase := filepath.Join(tmp, "ws")

	ensureRepo(t, workspaceBase, testRepo(originPath, "example.com", "main"))
	ensureRepo(t, workspaceBase, testRepo(originPath, "example.com", "main"))

	out, err := runGitOutput(originPath, "rev-list", "--count", "main")
	if err != nil {
		t.Fatalf("origin has no main after seeding: %v", err)
	}
	if strings.TrimSpace(out) != "1" {
		t.Errorf("origin main commit count = %q, want 1 (seeded once)", strings.TrimSpace(out))
	}
}

func TestEnsureRepo_FailsWhenOriginLacksTheBaseBranch(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "master")

	_, err := EnsureRepo(filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main"))
	if err == nil {
		t.Fatal("expected an error for a remote missing the base branch, got nil")
	}
	if !strings.Contains(err.Error(), "Base branch") {
		t.Errorf("error = %v, want it to point at the Base branch of the git integration", err)
	}
}

func TestEnsureRepo_ClonesEachRepoUnderItsHost(t *testing.T) {
	tmp := newGitEnv(t)
	giteaOrigin, _ := seedNamedOrigin(t, tmp, "gitea", "main")
	gitlabOrigin, _ := seedNamedOrigin(t, tmp, "gitlab", "trunk")
	workspaceBase := filepath.Join(tmp, "ws")

	giteaPath := ensureRepo(t, workspaceBase, testRepo(giteaOrigin, "gitea.example.com", "main"))
	gitlabPath := ensureRepo(t, workspaceBase, testRepo(gitlabOrigin, "gitlab.example.com:8443", "trunk"))

	if want := filepath.Join(workspaceBase, "gitea.example.com", "acme", "myrepo"); giteaPath != want {
		t.Errorf("gitea clone at %s, want %s", giteaPath, want)
	}
	if want := filepath.Join(workspaceBase, "gitlab.example.com:8443", "acme", "myrepo"); gitlabPath != want {
		t.Errorf("gitlab clone at %s, want %s", gitlabPath, want)
	}
	giteaWorktree, err := CreateWorktree(giteaPath, testGitToken, "main", "agent/a1/i1/1", 1)
	if err != nil {
		t.Fatalf("CreateWorktree in the gitea clone: %v", err)
	}
	gitlabWorktree, err := CreateWorktree(gitlabPath, testGitToken, "trunk", "agent/a1/i2/2", 2)
	if err != nil {
		t.Fatalf("CreateWorktree in the gitlab clone: %v", err)
	}
	assertFile(t, filepath.Join(giteaWorktree, "README.md"), "gitea\n")
	assertFile(t, filepath.Join(gitlabWorktree, "README.md"), "gitlab\n")
}

func TestEnsureRepo_ConcurrentFirstUseClonesOnce(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	workspaceBase := filepath.Join(tmp, "ws")
	repo := testRepo(originPath, "example.com", "main")

	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := EnsureRepo(workspaceBase, repo)
			errs <- err
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent EnsureRepo: %v", err)
		}
	}
	clones, err := ListClones(workspaceBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(clones) != 1 {
		t.Errorf("clones = %v, want exactly one", clones)
	}
}

func TestEnsureRepo_LeavesNoTokenOnDisk(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	workspaceBase := filepath.Join(tmp, "ws")
	repo := testRepo(originPath, "example.com", "main")

	repoPath := ensureRepo(t, workspaceBase, repo)
	if _, err := CreateWorktree(repoPath, repo.Token, "main", "agent/a1/i1/1", 1); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	ensureRepo(t, workspaceBase, repo)

	err := filepath.WalkDir(workspaceBase, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), repo.Token) {
			t.Errorf("the git token is written to %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRepo_CredentialHelperAnswersWithTheTokenFromEnv(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main"))
	worktreePath, err := CreateWorktree(repoPath, testGitToken, "main", "agent/a1/i1/1", 1)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	cmd := exec.Command("git", "credential", "fill")
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), GitTokenEnv("token-from-the-agent-env"), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=git.example.com\n\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git credential fill: %v", err)
	}
	if !strings.Contains(string(out), "username=x-token\n") || !strings.Contains(string(out), "password=token-from-the-agent-env\n") {
		t.Errorf("credential fill = %q, want the token from the env of the git process", out)
	}
}

func TestRepoLocalPath_RejectsPathsOutsideTheWorkspace(t *testing.T) {
	cases := []AgentRunGitRepo{
		{Host: "..", RepoPath: "acme/repo"},
		{Host: "example.com", RepoPath: "acme/../../etc"},
		{Host: "example.com", RepoPath: ""},
		{Host: "", RepoPath: "acme/repo"},
		{Host: "a/b", RepoPath: "acme/repo"},
	}
	for _, repo := range cases {
		if path, err := repo.LocalPath("/worktrees"); err == nil {
			t.Errorf("LocalPath(%+v) = %s, want an error", repo, path)
		}
	}
	path, err := AgentRunGitRepo{Host: "gitlab.com", RepoPath: "group/sub/repo"}.LocalPath("/worktrees")
	if err != nil || path != filepath.Join("/worktrees", "gitlab.com", "group", "sub", "repo") {
		t.Errorf("LocalPath of a subgroup repo = %s, %v", path, err)
	}
}

func TestRemoveLegacyClones_KeepsTheHostLayout(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	workspaceBase := filepath.Join(tmp, "ws")
	repoPath := ensureRepo(t, workspaceBase, testRepo(originPath, "example.com", "main"))
	legacyClone := filepath.Join(workspaceBase, "myrepo")
	git(t, tmp, "init", "-b", "main", legacyClone)
	git(t, legacyClone, "remote", "add", "origin", "https://x-token:old-pat@example.invalid/acme/myrepo.git")

	if err := RemoveLegacyClones(workspaceBase); err != nil {
		t.Fatalf("RemoveLegacyClones: %v", err)
	}

	if _, err := os.Stat(legacyClone); !os.IsNotExist(err) {
		t.Errorf("the old single-repo clone %s is still there", legacyClone)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Errorf("the clone in the host layout was removed: %v", err)
	}
}

func TestCreateWorktree_BranchesFromCurrentOriginNotTheCloneSnapshot(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, seed := seedOrigin(t, tmp, "main")
	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main"))

	if err := os.WriteFile(filepath.Join(seed, "merged.txt"), []byte("merged PR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "advance main after the clone")
	git(t, seed, "push", "origin", "main")
	want := headSha(t, seed)

	worktreePath, err := CreateWorktree(repoPath, testGitToken, "main", "agent/a1/i1/1", 1)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if got := headSha(t, worktreePath); got != want {
		t.Errorf("worktree HEAD = %s, want %s (origin/main as it is now, not at clone time)", got, want)
	}
	assertFile(t, filepath.Join(worktreePath, "merged.txt"), "merged PR\n")
}

func TestCreateWorktree_StartsFromTheBaseBranchOfTheRun(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, seed := seedOrigin(t, tmp, "main")
	git(t, seed, "checkout", "-b", "develop")
	if err := os.WriteFile(filepath.Join(seed, "only-on-develop.txt"), []byte("dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "develop only")
	git(t, seed, "push", "origin", "develop")

	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "develop"))
	worktreePath, err := CreateWorktree(repoPath, testGitToken, "develop", "agent/a1/i2/2", 2)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	assertFile(t, filepath.Join(worktreePath, "only-on-develop.txt"), "dev\n")
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

// prePushBranchPattern reads the branch regex from the hook file itself.
func prePushBranchPattern(t *testing.T) string {
	t.Helper()
	hook, err := os.ReadFile(filepath.Join("git-hooks", "pre-push"))
	if err != nil {
		t.Fatalf("reading pre-push hook: %v", err)
	}
	for _, line := range strings.Split(string(hook), "\n") {
		_, rest, found := strings.Cut(line, "=~ ")
		if !found {
			continue
		}
		pattern, _, ok := strings.Cut(rest, " ]]")
		if !ok {
			continue
		}
		return pattern
	}
	t.Fatal("no branch pattern (=~ ...) found in git-hooks/pre-push")
	return ""
}

func TestGenerateBranchName_MatchesPrePushHook(t *testing.T) {
	pattern := prePushBranchPattern(t)
	matcher := regexp.MustCompile(pattern)

	branch := GenerateBranchName(2, 30)
	if !matcher.MatchString(branch) {
		t.Errorf("GenerateBranchName(2, 30) = %q, which the pre-push hook pattern %q rejects", branch, pattern)
	}

	if matcher.MatchString("agent/b2/i30/1788794379") {
		t.Errorf("pre-push pattern %q still accepts the legacy agent/b prefix", pattern)
	}
}

func TestIsAgentBranch_MatchesPrePushHook(t *testing.T) {
	if got, want := agentBranchPattern.String(), prePushBranchPattern(t); got != want {
		t.Errorf("agentBranchPattern = %q, pre-push hook pattern = %q", got, want)
	}
	if !IsAgentBranch("agent/a2/i30/1788794379") {
		t.Error("a generated agent branch is rejected")
	}
	for _, branch := range []string{"main", "--upload-pack=x", "agent/a2/i30/1788794379/x"} {
		if IsAgentBranch(branch) {
			t.Errorf("IsAgentBranch(%q) = true, want false", branch)
		}
	}
}

func pushAgentBranch(t *testing.T, seed, branch string) string {
	t.Helper()
	git(t, seed, "checkout", "-b", branch)
	if err := os.WriteFile(filepath.Join(seed, "first-attempt.txt"), []byte("first attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "first attempt")
	git(t, seed, "push", "origin", branch)
	sha := headSha(t, seed)
	git(t, seed, "checkout", "main")
	return sha
}

func TestResumeWorktree_ContinuesOnExistingBranch(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, seed := seedOrigin(t, tmp, "main")
	const branch = "agent/a1/i48/1790053695"
	want := pushAgentBranch(t, seed, branch)

	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main"))

	worktreePath, err := ResumeWorktree(repoPath, testGitToken, branch, 5)
	if err != nil {
		t.Fatalf("ResumeWorktree: %v", err)
	}

	if got := headSha(t, worktreePath); got != want {
		t.Errorf("worktree HEAD = %s, want the branch head %s", got, want)
	}
	if got, _ := branchOfWorktree(worktreePath); got != branch {
		t.Errorf("worktree branch = %q, want %q", got, branch)
	}
	assertFile(t, filepath.Join(worktreePath, "first-attempt.txt"), "first attempt\n")

	if err := os.WriteFile(filepath.Join(worktreePath, "second-attempt.txt"), []byte("fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, worktreePath, "add", ".")
	git(t, worktreePath, "commit", "-m", "second attempt")
	git(t, worktreePath, "push", "origin", branch)
	remoteHead, err := runGitOutput(repoPath, "ls-remote", "origin", "refs/heads/"+branch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(remoteHead, headSha(t, worktreePath)) {
		t.Errorf("origin %s = %q, want it fast-forwarded to the new commit", branch, remoteHead)
	}
}

func TestResumeWorktree_TakesBranchFromStaleWorktree(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, seed := seedOrigin(t, tmp, "main")
	const branch = "agent/a1/i48/1790053695"
	want := pushAgentBranch(t, seed, branch)

	repoPath := ensureRepo(t, filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main"))

	if _, err := ResumeWorktree(repoPath, testGitToken, branch, 1); err != nil {
		t.Fatalf("ResumeWorktree for the old run: %v", err)
	}
	worktreePath, err := ResumeWorktree(repoPath, testGitToken, branch, 2)
	if err != nil {
		t.Fatalf("ResumeWorktree for the new run while the old worktree holds the branch: %v", err)
	}
	if got, _ := branchOfWorktree(worktreePath); got != branch {
		t.Errorf("worktree branch = %q, want %q", got, branch)
	}
	if got := headSha(t, worktreePath); got != want {
		t.Errorf("worktree HEAD = %s, want %s", got, want)
	}
}
