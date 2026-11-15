package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// completeRecorder is a tracker stub that answers every call with 200 and
// remembers the complete_stage payload of each task.
type completeRecorder struct {
	mu        sync.Mutex
	completes map[int64]CompleteStagePayload
}

func newCompleteRecorder(t *testing.T) (*completeRecorder, *httptest.Server) {
	t.Helper()
	recorder := &completeRecorder{completes: map[int64]CompleteStagePayload{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var idTask int64
		if _, err := fmt.Sscanf(request.URL.Path, "/agent/task/%d/complete", &idTask); err == nil {
			var payload CompleteStagePayload
			_ = json.NewDecoder(request.Body).Decode(&payload)
			recorder.mu.Lock()
			recorder.completes[idTask] = payload
			recorder.mu.Unlock()
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return recorder, server
}

func (r *completeRecorder) complete(idTask int64) (CompleteStagePayload, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	payload, ok := r.completes[idTask]
	return payload, ok
}

// attemptRecord is what one agent.Run saw in its worktree.
type attemptRecord struct {
	task        Task
	hasWorkFile bool
	log         string
}

// committingFailAgent commits work.txt locally and fails on its first Run, then
// records what every later Run finds in the worktree.
type committingFailAgent struct {
	t        *testing.T
	firstErr error
	mu       sync.Mutex
	attempts []attemptRecord
}

func (a *committingFailAgent) Run(_ context.Context, task Task) (RunStats, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, statErr := os.Stat(filepath.Join(task.WorktreePath, "work.txt"))
	logOut, _ := runGitOutput(task.WorktreePath, "log", "--format=%s")
	a.attempts = append(a.attempts, attemptRecord{task: task, hasWorkFile: statErr == nil, log: logOut})
	if len(a.attempts) > 1 {
		return RunStats{}, nil
	}
	if err := os.WriteFile(filepath.Join(task.WorktreePath, "work.txt"), []byte("half done\n"), 0o644); err != nil {
		a.t.Errorf("writing work.txt: %v", err)
	}
	git(a.t, task.WorktreePath, "add", "work.txt")
	git(a.t, task.WorktreePath, "commit", "-m", "local work")
	return RunStats{}, a.firstErr
}

func (a *committingFailAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func (a *committingFailAgent) attempt(t *testing.T, index int) attemptRecord {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.attempts) <= index {
		t.Fatalf("agent ran %d times, want attempt %d", len(a.attempts), index+1)
	}
	return a.attempts[index]
}

func resumeTestOrchestrator(t *testing.T, agent Agent) (*Orchestrator, *completeRecorder, AgentRunGitRepo) {
	t.Helper()
	workspaceBase, repo := stageTestWorkspace(t)
	recorder, tracker := newCompleteRecorder(t)
	cfg := &Config{
		TrackerAPIUrl: tracker.URL,
		WebhookSecret: testWebhookSecret,
		WorkspaceBase: workspaceBase,
	}
	return NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState()), recorder, repo
}

func resumeTestTask(repo AgentRunGitRepo, idTask int64, attemptNo int) Task {
	return Task{Repo: repo, IdTask: idTask, IdRun: 200, IdUserAgent: 1, IdIssue: 300, Stage: StageImplementation, AttemptNo: attemptNo}
}

func runWorktreePath(t *testing.T, orchestrator *Orchestrator, repo AgentRunGitRepo, idRun int64) string {
	t.Helper()
	repoPath, err := repo.LocalPath(orchestrator.cfg.WorkspaceBase)
	if err != nil {
		t.Fatalf("LocalPath: %v", err)
	}
	return WorktreePath(repoPath, idRun)
}

func TestRunStage_KeepsWorktreeAfterAgentFailure(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantReason string
	}{
		{"rate limited", &AgentError{Code: ErrCodeProviderRateLimited}, ErrCodeProviderRateLimited},
		{"stage not submitted", &AgentError{Code: ErrCodeStageNotSubmitted}, ErrCodeStageNotSubmitted},
		{"untyped error", errors.New("boom"), "agent_error"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			agent := &committingFailAgent{t: t, firstErr: testCase.err}
			orchestrator, recorder, repo := resumeTestOrchestrator(t, agent)

			orchestrator.runStage(resumeTestTask(repo, 100, 1))
			orchestrator.runStage(resumeTestTask(repo, 101, 2))

			first, second := agent.attempt(t, 0), agent.attempt(t, 1)
			if second.task.Branch != first.task.Branch {
				t.Errorf("second attempt ran on %q, want the first attempt's branch %q", second.task.Branch, first.task.Branch)
			}
			if !second.hasWorkFile {
				t.Error("second attempt does not see work.txt from the failed attempt")
			}
			if !strings.Contains(second.log, "local work") {
				t.Errorf("second attempt does not see the local commit; log:\n%s", second.log)
			}
			payload, ok := recorder.complete(100)
			if !ok {
				t.Fatal("the failed attempt was not reported to the tracker")
			}
			if payload.Outcome != OutcomeErrored || payload.ErrorReason != testCase.wantReason {
				t.Errorf("complete_stage = %s/%s, want %s/%s", payload.Outcome, payload.ErrorReason, OutcomeErrored, testCase.wantReason)
			}
		})
	}
}

func TestRunStage_KeepsWorktreeWhenTrackerDropsTheTask(t *testing.T) {
	const goneTask = int64(100)
	tracker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == fmt.Sprintf("/agent/task/%d/heartbeat", goneTask) {
			writer.WriteHeader(http.StatusConflict)
			_, _ = writer.Write([]byte(`{"code":"TASK_NOT_RUNNING_BY_AGENT"}`))
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(tracker.Close)
	workspaceBase, repo := stageTestWorkspace(t)
	cfg := &Config{TrackerAPIUrl: tracker.URL, WebhookSecret: testWebhookSecret, WorkspaceBase: workspaceBase}
	agent := &cancellableAgent{entered: make(chan int64, 1), release: make(chan struct{})}
	t.Cleanup(func() { close(agent.release) })
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())
	orchestrator.heartbeatLoop.interval = 20 * time.Millisecond

	done := make(chan struct{})
	go func() {
		orchestrator.runStage(resumeTestTask(repo, goneTask, 1))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stage did not stop after the tracker dropped it")
	}

	if _, err := os.Stat(runWorktreePath(t, orchestrator, repo, 200)); err != nil {
		t.Errorf("worktree of the dropped stage is gone: %v", err)
	}
}

type failingWorkspaceAgent struct {
	workspace chan string
}

func (a *failingWorkspaceAgent) Run(_ context.Context, task Task) (RunStats, error) {
	a.workspace <- task.WorktreePath
	return RunStats{}, errors.New("boom")
}

func (a *failingWorkspaceAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func TestRunStage_ReleaseNotesRemovesScratchDirOnFailure(t *testing.T) {
	recorder, tracker := newCompleteRecorder(t)
	cfg := &Config{WebhookSecret: testWebhookSecret, TrackerAPIUrl: tracker.URL, WorkspaceBase: filepath.Join(t.TempDir(), "never-created")}
	agent := &failingWorkspaceAgent{workspace: make(chan string, 1)}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	orchestrator.runStage(Task{IdTask: 1, IdRun: 2, Stage: StageReleaseNotes, AttemptNo: 1})

	workspace := <-agent.workspace
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Errorf("the scratch directory %s is left behind after a failure", workspace)
	}
	if payload, _ := recorder.complete(1); payload.Outcome != OutcomeErrored {
		t.Errorf("complete_stage outcome = %q, want %q", payload.Outcome, OutcomeErrored)
	}
}

func TestRunStage_FailsOnUnusableWorktreeWithoutRemovingIt(t *testing.T) {
	cases := []struct {
		name          string
		breakWorktree func(t *testing.T, worktreePath string)
	}{
		{"detached with uncommitted work", func(t *testing.T, worktreePath string) {
			git(t, worktreePath, "checkout", "--detach")
			if err := os.WriteFile(filepath.Join(worktreePath, "draft.txt"), []byte("draft\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"not a git worktree", func(t *testing.T, worktreePath string) {
			if err := os.Remove(filepath.Join(worktreePath, ".git")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			agent := &recordingAgent{}
			orchestrator, recorder, repo := resumeTestOrchestrator(t, agent)
			orchestrator.runStage(resumeTestTask(repo, 100, 1))
			worktreePath := runWorktreePath(t, orchestrator, repo, 200)
			testCase.breakWorktree(t, worktreePath)

			orchestrator.runStage(resumeTestTask(repo, 101, 2))

			if ran := agent.ran(); len(ran) != 1 {
				t.Errorf("agent ran %d times, want only the first attempt", len(ran))
			}
			if payload, _ := recorder.complete(101); payload.ErrorReason != ErrCodeWorktreeUnusable {
				t.Errorf("error_reason = %q, want %q", payload.ErrorReason, ErrCodeWorktreeUnusable)
			}
			if _, err := os.Stat(worktreePath); err != nil {
				t.Errorf("the unusable worktree was removed: %v", err)
			}
		})
	}
}

func TestRunStage_RecreatesCleanDetachedWorktree(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	worktreePath := runWorktreePath(t, orchestrator, repo, 200)
	// What releaseBranch leaves behind: the worktree detached, its branch now
	// belonging to another run. Renaming also keeps the next branch name, which
	// has one-second resolution, from colliding with it.
	git(t, worktreePath, "checkout", "--detach")
	git(t, worktreePath, "branch", "-m", agent.ran()[0].Branch, "agent/a9/i9/1")

	orchestrator.runStage(resumeTestTask(repo, 101, 2))

	ran := agent.ran()
	if len(ran) != 2 {
		t.Fatalf("agent ran %d times, want 2", len(ran))
	}
	if !IsAgentBranch(ran[1].Branch) {
		t.Errorf("second attempt ran on %q, want a fresh agent branch", ran[1].Branch)
	}
}

func waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func isMissing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func TestHandleEvent_RunDiscardedRemovesIdleWorktreeAndKeepsBranch(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	worktreePath := runWorktreePath(t, orchestrator, repo, 200)

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "run_discarded", IdRun: 200}); err != nil {
		t.Fatalf("HandleEvent(run_discarded): %v", err)
	}

	waitUntil(t, "the discarded worktree is removed", func() bool { return isMissing(worktreePath) })
	repoPath := filepath.Dir(filepath.Dir(worktreePath))
	if err := runGitQuiet(repoPath, "rev-parse", "--verify", "refs/heads/"+agent.ran()[0].Branch); err != nil {
		t.Errorf("the branch of the discarded run was deleted with its worktree: %v", err)
	}
}

func TestHandleEvent_RunDiscardedStopsActiveStageThenRemovesWorktree(t *testing.T) {
	agent := &cancellableAgent{entered: make(chan int64, 1), release: make(chan struct{})}
	t.Cleanup(func() { close(agent.release) })
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	done := make(chan struct{})
	go func() {
		orchestrator.runStage(resumeTestTask(repo, 100, 1))
		close(done)
	}()
	select {
	case <-agent.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the stage never started")
	}

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "run_discarded", IdRun: 200}); err != nil {
		t.Fatalf("HandleEvent(run_discarded): %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the discarded stage kept running")
	}
	if worktreePath := runWorktreePath(t, orchestrator, repo, 200); !isMissing(worktreePath) {
		t.Errorf("worktree %s of the discarded stage is left behind", worktreePath)
	}
}

func TestHandleEvent_ContinueAfterRunDiscardedKeepsNewWorktree(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	worktreePath := runWorktreePath(t, orchestrator, repo, 200)
	if err := orchestrator.HandleEvent(WebhookEvent{Type: "run_discarded", IdRun: 200}); err != nil {
		t.Fatalf("HandleEvent(run_discarded): %v", err)
	}
	waitUntil(t, "the discarded worktree is removed", func() bool { return isMissing(worktreePath) })
	// Branch names have one-second resolution; move the old one aside so the
	// next attempt in the same second can create its own.
	git(t, filepath.Dir(filepath.Dir(worktreePath)), "branch", "-m", agent.ran()[0].Branch, "agent/a9/i9/1")

	orchestrator.runStage(resumeTestTask(repo, 101, 2))

	if len(agent.ran()) != 2 {
		t.Fatalf("agent ran %d times, want 2", len(agent.ran()))
	}
	if isMissing(worktreePath) {
		t.Error("the worktree of an attempt started after run_discarded was removed")
	}
}

func TestHandleEvent_CancelledKeepsWorktree(t *testing.T) {
	agent := &cancellableAgent{entered: make(chan int64, 1), release: make(chan struct{})}
	t.Cleanup(func() { close(agent.release) })
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	done := make(chan struct{})
	go func() {
		orchestrator.runStage(resumeTestTask(repo, 100, 1))
		close(done)
	}()
	select {
	case <-agent.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the stage never started")
	}

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "cancelled", IdRun: 200}); err != nil {
		t.Fatalf("HandleEvent(cancelled): %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled stage kept running")
	}
	if worktreePath := runWorktreePath(t, orchestrator, repo, 200); isMissing(worktreePath) {
		t.Error("cancel removed the worktree; only Restart may")
	}
}

func TestRunStage_ResumedWorktreeOnlyWhenWorktreeExisted(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)

	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	orchestrator.runStage(resumeTestTask(repo, 101, 2))

	ran := agent.ran()
	if len(ran) != 2 {
		t.Fatalf("agent ran %d times, want 2", len(ran))
	}
	if ran[0].ResumedWorktree {
		t.Error("the attempt that created the worktree was told it resumed one")
	}
	if !ran[1].ResumedWorktree {
		t.Error("the attempt on the existing worktree was not told it resumed one")
	}
}

func TestHandleEvent_RunDiscardedRemovesWorktreeOfQueuedStage(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	worktreePath := runWorktreePath(t, orchestrator, repo, 200)
	orchestrator.sem <- struct{}{} // another run's stage holds the only slot
	done := make(chan struct{})
	go func() {
		orchestrator.runStage(resumeTestTask(repo, 101, 2))
		close(done)
	}()
	waitUntil(t, "the next stage is queued", func() bool { return orchestrator.ActiveTaskCount() == 1 })

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "run_discarded", IdRun: 200}); err != nil {
		t.Fatalf("HandleEvent(run_discarded): %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the discarded stage stayed queued")
	}
	<-orchestrator.sem

	waitUntil(t, "the discarded worktree is removed", func() bool { return isMissing(worktreePath) })
	if ran := agent.ran(); len(ran) != 1 {
		t.Errorf("agent ran %d times, want only the stage before Restart", len(ran))
	}
}

func TestRunStage_ResumesWorktreeStoppedMidRebase(t *testing.T) {
	agent := &recordingAgent{}
	orchestrator, _, repo := resumeTestOrchestrator(t, agent)
	orchestrator.runStage(resumeTestTask(repo, 100, 1))
	worktreePath := runWorktreePath(t, orchestrator, repo, 200)
	stopMidRebase(t, filepath.Dir(filepath.Dir(worktreePath)), worktreePath)

	orchestrator.runStage(resumeTestTask(repo, 101, 2))

	ran := agent.ran()
	if len(ran) != 2 {
		t.Fatalf("agent ran %d times, want 2", len(ran))
	}
	if ran[1].Branch != ran[0].Branch || !ran[1].ResumedWorktree {
		t.Errorf("second attempt ran on %q (resumed %v), want the rebased branch %q in the same worktree", ran[1].Branch, ran[1].ResumedWorktree, ran[0].Branch)
	}
}
