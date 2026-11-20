package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitOutputOf(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGitOutput(dir, args...)
	if err != nil {
		t.Fatalf("git %s (in %s): %v", strings.Join(args, " "), dir, err)
	}
	return strings.TrimSpace(out)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s still exists (stat err %v)", path, err)
	}
}

func assertCleanDetachedMainCheckout(t *testing.T, repoPath string) {
	t.Helper()
	if branch, err := branchOfWorktree(repoPath); err != nil || branch != "" {
		t.Errorf("main clone is on branch %q (err %v), want a detached HEAD", branch, err)
	}
	var changes []string
	for _, line := range strings.Split(gitOutputOf(t, repoPath, "status", "--porcelain", "--ignored"), "\n") {
		if line != "" && line != "!! "+agentRunsDir+"/" {
			changes = append(changes, line)
		}
	}
	if len(changes) > 0 {
		t.Errorf("main clone is not clean:\n%s", strings.Join(changes, "\n"))
	}
}

// Run 67: the agent worked in the main clone instead of its worktree, switched
// it to the run branch, committed there and left changes and node_modules.
func TestEnsureRepo_RestoresAMainCloneTheAgentWorkedIn(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	workspace := filepath.Join(tmp, "ws")
	repo := testRepo(originPath, "example.com", "main")
	repoPath := ensureRepo(t, workspace, repo)
	worktreePath, err := CreateWorktree(repoPath, testGitToken, "main", hookTestBranch, 67)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	writeTestFile(t, filepath.Join(worktreePath, "wip.txt"), "uncommitted work of the run\n")

	git(t, repoPath, "checkout", "--ignore-other-worktrees", "-B", hookTestBranch, "origin/main")
	writeTestFile(t, filepath.Join(repoPath, "feature.ts"), "committed in the main clone\n")
	git(t, repoPath, "add", "feature.ts")
	git(t, repoPath, "commit", "-m", "work committed in the main clone")
	agentCommit := gitOutputOf(t, repoPath, "rev-parse", "HEAD")
	writeTestFile(t, filepath.Join(repoPath, "README.md"), "changed in the main clone\n")
	writeTestFile(t, filepath.Join(repoPath, "untracked.ts"), "untracked\n")
	writeTestFile(t, filepath.Join(repoPath, ".gitignore"), "node_modules/\n")
	writeTestFile(t, filepath.Join(repoPath, "node_modules", "vitest", "index.js"), "ignored\n")

	ensureRepo(t, workspace, repo)

	assertCleanDetachedMainCheckout(t, repoPath)
	assertFile(t, filepath.Join(repoPath, "README.md"), "origin\n")
	assertMissing(t, filepath.Join(repoPath, "untracked.ts"))
	assertMissing(t, filepath.Join(repoPath, "node_modules"))
	if got := gitOutputOf(t, repoPath, "rev-parse", hookTestBranch); got != agentCommit {
		t.Errorf("branch %s = %s, want the agent's commit %s to stay reachable", hookTestBranch, got, agentCommit)
	}
	assertFile(t, filepath.Join(worktreePath, "wip.txt"), "uncommitted work of the run\n")
}

func TestEnsureRepo_LeavesAFreshCloneDetachedAndUsable(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	workspace := filepath.Join(tmp, "ws")
	repo := testRepo(originPath, "example.com", "main")

	repoPath := ensureRepo(t, workspace, repo)
	assertCleanDetachedMainCheckout(t, repoPath)
	head := headSha(t, repoPath)

	ensureRepo(t, workspace, repo)
	assertCleanDetachedMainCheckout(t, repoPath)
	if got := headSha(t, repoPath); got != head {
		t.Errorf("a second EnsureRepo moved the clean main clone from %s to %s", head, got)
	}

	worktreePath, err := CreateWorktree(repoPath, testGitToken, "main", hookTestBranch, 1)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	writeTestFile(t, filepath.Join(worktreePath, "feature.ts"), "feature\n")
	git(t, worktreePath, "add", ".")
	git(t, worktreePath, "commit", "-m", "feature")
	git(t, worktreePath, "push", "origin", hookTestBranch)
	if err := RemoveWorktree(repoPath, 1); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	ensureRepo(t, workspace, repo)
	resumedPath, err := ResumeWorktree(repoPath, testGitToken, hookTestBranch, 2)
	if err != nil {
		t.Fatalf("ResumeWorktree: %v", err)
	}
	assertFile(t, filepath.Join(resumedPath, "feature.ts"), "feature\n")
	assertCleanDetachedMainCheckout(t, repoPath)
}
