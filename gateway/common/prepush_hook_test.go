package common

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const hookTestBranch = "agent/a1/i2/1788794379"

// newHookTestRepo builds a main clone with a run worktree inside .agent-runs,
// the layout the gateway creates, and returns both paths.
func newHookTestRepo(t *testing.T) (mainPath, worktreePath string) {
	t.Helper()
	tmp := newGitEnv(t)
	mainPath = filepath.Join(tmp, "main")
	git(t, tmp, "init", "-b", "main", mainPath)
	if err := os.WriteFile(filepath.Join(mainPath, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, mainPath, "add", ".")
	git(t, mainPath, "commit", "-m", "init")
	worktreePath = filepath.Join(mainPath, agentRunsDir, "7")
	git(t, mainPath, "worktree", "add", "-b", hookTestBranch, worktreePath)
	return mainPath, worktreePath
}

// runPrePushHook runs the real hook script in dir the way git does, with one
// ref line on stdin, and returns its output and whether it allowed the push.
func runPrePushHook(t *testing.T, dir, branch, worktreeEnv string) (output string, isAllowed bool) {
	t.Helper()
	hookPath, err := filepath.Abs(filepath.Join("git-hooks", "pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", hookPath, "origin", "https://example.com/acme/repo.git")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if worktreeEnv != "" {
		cmd.Env = append(cmd.Env, WorktreeEnvName+"="+worktreeEnv)
	}
	ref := "refs/heads/" + branch
	cmd.Stdin = strings.NewReader(ref + " 1111111111111111111111111111111111111111 " + ref + " 0000000000000000000000000000000000000000\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, isExitErr := err.(*exec.ExitError); !isExitErr {
			t.Fatalf("running the hook: %v", err)
		}
	}
	return string(out), err == nil
}

func TestPrePushHook_RunWorktreeGuard(t *testing.T) {
	mainPath, worktreePath := newHookTestRepo(t)
	linkPath := filepath.Join(t.TempDir(), "worktree-link")
	if err := os.Symlink(worktreePath, linkPath); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		dir         string
		branch      string
		worktreeEnv string
		isAllowed   bool
		wantOutput  string
	}{
		{"push from the run worktree", worktreePath, hookTestBranch, worktreePath, true, ""},
		{"push from a subdirectory of the run worktree", filepath.Join(worktreePath, agentRunsDir), hookTestBranch, worktreePath, true, ""},
		{"push from the main clone", mainPath, hookTestBranch, worktreePath, false, worktreePath},
		{"worktree named through a symlink", worktreePath, hookTestBranch, linkPath, true, ""},
		{"no worktree in the env keeps today's behaviour", mainPath, hookTestBranch, "", true, ""},
		{"a non-agent branch is still rejected", worktreePath, "main", worktreePath, false, "Agent branches must match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(tt.dir, 0o755); err != nil {
				t.Fatal(err)
			}
			output, isAllowed := runPrePushHook(t, tt.dir, tt.branch, tt.worktreeEnv)
			if isAllowed != tt.isAllowed {
				t.Fatalf("allowed = %v, want %v\n%s", isAllowed, tt.isAllowed, output)
			}
			if tt.wantOutput != "" && !strings.Contains(output, tt.wantOutput) {
				t.Errorf("output does not mention %q:\n%s", tt.wantOutput, output)
			}
		})
	}
}
