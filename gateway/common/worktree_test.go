package common

import (
	"os"
	"os/exec"
	"path/filepath"
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

// TestCloneRepo_ForcesDirtyBaseBackToOrigin reproduces the startup crash
// where a persistent /worktrees volume carried leftover edits + untracked
// files colliding with a merged PR's new files, aborting `pull --ff-only`.
// The refresh must force the base checkout back to origin instead.
func TestCloneRepo_ForcesDirtyBaseBackToOrigin(t *testing.T) {
	// Isolate from the host git config and give commits a stable identity.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	tmp := t.TempDir()

	// Point the hook fixture somewhere real so InstallHooks doesn't hit the
	// container path.
	hookFixture := filepath.Join(tmp, "pre-push")
	if err := os.WriteFile(hookFixture, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := hookSourcePath
	hookSourcePath = hookFixture
	t.Cleanup(func() { hookSourcePath = orig })

	// Bare origin seeded with an initial commit on main.
	originPath := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--bare", "-b", "main", originPath)
	seed := filepath.Join(tmp, "seed")
	git(t, tmp, "init", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "init")
	git(t, seed, "remote", "add", "origin", originPath)
	git(t, seed, "push", "origin", "main")

	cfg := &Config{
		WorkspaceBase:  filepath.Join(tmp, "ws"),
		RepoUrl:        originPath,
		RepoBranchBase: "main",
	}
	repoPath := filepath.Join(cfg.WorkspaceBase, "origin")

	// First run clones fresh.
	if err := CloneRepo(cfg); err != nil {
		t.Fatalf("initial CloneRepo: %v", err)
	}

	// A merged PR advances origin: it adds url-codec.html and rewrites README.
	if err := os.WriteFile(filepath.Join(seed, "url-codec.html"), []byte("<html>origin</html>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "add url-codec")
	git(t, seed, "push", "origin", "main")

	// The persistent base checkout is dirty: README locally edited, and the
	// same url-codec.html present but UNTRACKED — the state that made
	// `pull --ff-only` abort. Also seed a per-run worktree dir that must survive.
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("local scribble\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "url-codec.html"), []byte("<html>local</html>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(repoPath, agentRunsDir, "123")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keep, "run.txt"), []byte("in-flight\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second run must NOT fail, and must force the base back to origin.
	if err := CloneRepo(cfg); err != nil {
		t.Fatalf("refresh CloneRepo on dirty base: %v", err)
	}

	assertFile(t, filepath.Join(repoPath, "README.md"), "v2\n")
	assertFile(t, filepath.Join(repoPath, "url-codec.html"), "<html>origin</html>\n")
	assertFile(t, filepath.Join(keep, "run.txt"), "in-flight\n") // worktree tree preserved
}

// TestCloneRepo_SeedsEmptyRemote reproduces the startup crash on a brand-new,
// empty repo: `fetch origin main` fataled with "couldn't find remote ref
// main". CloneRepo must instead seed the empty origin with an initial commit
// on the base branch.
func TestCloneRepo_SeedsEmptyRemote(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	tmp := t.TempDir()

	hookFixture := filepath.Join(tmp, "pre-push")
	if err := os.WriteFile(hookFixture, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := hookSourcePath
	hookSourcePath = hookFixture
	t.Cleanup(func() { hookSourcePath = orig })

	// A completely empty bare origin — no branches, no commits.
	originPath := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--bare", "-b", "main", originPath)

	cfg := &Config{
		WorkspaceBase:  filepath.Join(tmp, "ws"),
		RepoUrl:        originPath,
		RepoBranchBase: "main",
	}
	repoPath := filepath.Join(cfg.WorkspaceBase, "origin")

	// First run: clones the empty repo, then seeds main. Must not fatal.
	if err := CloneRepo(cfg); err != nil {
		t.Fatalf("initial CloneRepo on empty remote: %v", err)
	}

	// Origin now has main at exactly one commit.
	if out, err := runGitOutput(originPath, "rev-list", "--count", "main"); err != nil {
		t.Fatalf("origin has no main after seeding: %v", err)
	} else if strings.TrimSpace(out) != "1" {
		t.Errorf("origin main commit count = %q, want 1", strings.TrimSpace(out))
	}

	// Local base checkout is on main.
	branch, err := runGitOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("reading local HEAD: %v", err)
	}
	if strings.TrimSpace(branch) != "main" {
		t.Errorf("local branch = %q, want main", strings.TrimSpace(branch))
	}

	// Second run must be idempotent: main now exists on origin, so it takes the
	// normal sync path and does not re-seed or fatal.
	if err := CloneRepo(cfg); err != nil {
		t.Fatalf("second CloneRepo after seeding: %v", err)
	}
	if out, err := runGitOutput(originPath, "rev-list", "--count", "main"); err != nil {
		t.Fatalf("origin main missing on second run: %v", err)
	} else if strings.TrimSpace(out) != "1" {
		t.Errorf("origin main commit count after second run = %q, want 1 (no re-seed)", strings.TrimSpace(out))
	}
}

// TestCloneRepo_MismatchedBaseBranch: a non-empty remote lacking the
// configured base branch is a misconfiguration, not an empty repo —
// CloneRepo must fail with a clear error rather than seed or crash cryptically.
func TestCloneRepo_MismatchedBaseBranch(t *testing.T) {
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

	// Origin has a `master` branch but no `main`.
	originPath := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--bare", "-b", "master", originPath)
	seed := filepath.Join(tmp, "seed")
	git(t, tmp, "init", "-b", "master", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", ".")
	git(t, seed, "commit", "-m", "init")
	git(t, seed, "remote", "add", "origin", originPath)
	git(t, seed, "push", "origin", "master")

	cfg := &Config{
		WorkspaceBase:  filepath.Join(tmp, "ws"),
		RepoUrl:        originPath,
		RepoBranchBase: "main",
	}

	err := CloneRepo(cfg)
	if err == nil {
		t.Fatal("expected an error for a remote missing the base branch, got nil")
	}
	if !strings.Contains(err.Error(), "REPO_BRANCH_BASE") {
		t.Errorf("error = %v, want it to mention REPO_BRANCH_BASE", err)
	}
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

func TestRepoSlugFromURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"https with .git", "https://github.com/org/repo.git", "org/repo"},
		{"https without .git", "https://github.com/org/repo", "org/repo"},
		{"https trailing slash", "https://github.com/org/repo/", "org/repo"},
		{"gitlab subgroup", "https://gitlab.com/group/sub/repo.git", "group/sub/repo"},
		{"self-hosted gitea", "https://gitea.example.com/team/proj.git", "team/proj"},
		{"ssh form", "git@github.com:org/repo.git", "org/repo"},
		{"with credentials in url", "https://x-token:pat@github.com/org/repo.git", "org/repo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RepoSlugFromURL(tc.url); got != tc.want {
				t.Errorf("RepoSlugFromURL(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
