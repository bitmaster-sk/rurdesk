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

func TestNewRetention_DefaultsWhenUnset(t *testing.T) {
	retention := NewRetention(&Config{}, nil)
	if retention.maxAge != 7*24*time.Hour {
		t.Errorf("worktree retention = %v, want 7 days", retention.maxAge)
	}
	if retention.unpushedBranchMaxAge != 30*24*time.Hour {
		t.Errorf("unpushed branch retention = %v, want 30 days", retention.unpushedBranchMaxAge)
	}
}

func TestRetention_KeepsWorktreeUsedRecently(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	orchestrator.cfg.WorktreeRetention = 7 * 24 * time.Hour
	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	worktreePath := runWorktreePath(t, orchestrator, repo, 200)
	created := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(worktreePath, created, created); err != nil {
		t.Fatal(err)
	}

	orchestrator.runStage(resumeTestTask(repo, 101, 2))
	NewRetention(orchestrator.cfg, orchestrator).runOnce()

	if _, err := os.Stat(worktreePath); err != nil {
		t.Errorf("worktree used by the last stage was pruned: %v", err)
	}
}

func TestRetention_PrunesWorktreeIdleLongerThanConfigured(t *testing.T) {
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	cfg := &Config{WorkspaceBase: filepath.Join(tmp, "ws"), WorktreeRetention: 24 * time.Hour}
	repoPath := ensureRepo(t, cfg.WorkspaceBase, testRepo(originPath, "example.com", "main"))
	path, err := CreateWorktree(repoPath, testGitToken, "main", GenerateBranchName(1, 1), 1)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	idle := time.Now().Add(-2 * 24 * time.Hour)
	if err := os.Chtimes(path, idle, idle); err != nil {
		t.Fatal(err)
	}

	NewRetention(cfg, NewOrchestrator(cfg, nil, NewTrackerClient(cfg), NewState())).runOnce()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree idle for 2 days survived a 1-day retention")
	}
}
