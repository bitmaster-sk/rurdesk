package common

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetention_PrunesIdleWorktreesInEveryClone(t *testing.T) {
	tmp := newGitEnv(t)
	giteaOrigin, _ := seedNamedOrigin(t, tmp, "gitea", "main")
	gitlabOrigin, _ := seedNamedOrigin(t, tmp, "gitlab", "main")
	cfg := &Config{WorkspaceBase: filepath.Join(tmp, "ws")}
	giteaPath := ensureRepo(t, cfg.WorkspaceBase, testRepo(giteaOrigin, "gitea.example.com", "main"))
	gitlabPath := ensureRepo(t, cfg.WorkspaceBase, testRepo(gitlabOrigin, "gitlab.example.com", "main"))

	idle := time.Now().Add(-30 * 24 * time.Hour)
	worktree := func(repoPath string, idRun int64) string {
		path, err := CreateWorktree(repoPath, testGitToken, "main", GenerateBranchName(1, idRun), idRun)
		if err != nil {
			t.Fatalf("CreateWorktree: %v", err)
		}
		if err := os.Chtimes(path, idle, idle); err != nil {
			t.Fatal(err)
		}
		return path
	}
	idleGitea := worktree(giteaPath, 1)
	idleGitlab := worktree(gitlabPath, 2)
	activeGitlab := worktree(gitlabPath, 3)

	orchestrator := NewOrchestrator(cfg, nil, NewTrackerClient(cfg), NewState())
	orchestrator.activeTasks[30] = &activeTask{task: Task{IdTask: 30, IdRun: 3}}
	NewRetention(cfg, orchestrator).runOnce()

	for _, pruned := range []string{idleGitea, idleGitlab} {
		if _, err := os.Stat(pruned); !os.IsNotExist(err) {
			t.Errorf("idle worktree %s was not pruned", pruned)
		}
	}
	if _, err := os.Stat(activeGitlab); err != nil {
		t.Errorf("worktree of an active run was pruned: %v", err)
	}
}
