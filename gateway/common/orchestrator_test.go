package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingAgent signals on entered when Run starts and blocks until release is
// closed, so a test can observe exactly how many Run calls are in flight.
type blockingAgent struct {
	entered chan int64
	release chan struct{}
}

func (a *blockingAgent) Run(ctx context.Context, task Task) (RunStats, error) {
	a.entered <- task.IdTask
	select {
	case <-a.release:
	case <-ctx.Done():
	}
	return RunStats{}, nil
}

func (a *blockingAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func stageTestWorkspace(t *testing.T) (string, AgentRunGitRepo) {
	t.Helper()
	tmp := newGitEnv(t)
	originPath, _ := seedOrigin(t, tmp, "main")
	return filepath.Join(tmp, "ws"), testRepo(originPath, "example.com", "main")
}

// TestRunStage_RunsOneStageAtATime verifies the gateway runs one stage
// attempt at a time, queueing the rest — the property
// that keeps concurrent CLI processes off the shared OAuth credential.
func TestRunStage_RunsOneStageAtATime(t *testing.T) {
	workspaceBase, repo := stageTestWorkspace(t)

	cfg := &Config{
		WebhookSecret: testWebhookSecret,
		WorkspaceBase: workspaceBase,
	}
	agent := &blockingAgent{entered: make(chan int64, 3), release: make(chan struct{})}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	const taskCount = 3
	for i := 0; i < taskCount; i++ {
		task := Task{
			Repo:        repo,
			IdTask:      int64(100 + i),
			IdRun:       int64(200 + i),
			IdUserAgent: 1,
			IdIssue:     int64(300 + i), // distinct issue → distinct branch
			Stage:       StageDesign,
			AttemptNo:   1,
		}
		go orchestrator.runStage(task)
	}

	// Exactly one Run is allowed in flight.
	var first int64
	select {
	case first = <-agent.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("no agent.Run started within timeout")
	}
	select {
	case id := <-agent.entered:
		t.Fatalf("a second agent.Run started while one held the only slot (id=%d)", id)
	case <-time.After(200 * time.Millisecond):
	}

	// Releasing the in-flight task lets the queued ones proceed; all must run.
	close(agent.release)
	seen := map[int64]bool{first: true}
	for len(seen) < taskCount {
		select {
		case id := <-agent.entered:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("expected all %d stages to run, saw %d", taskCount, len(seen))
		}
	}
}

func TestRunStage_CancelledQueuedStageNeverRunsTheAgent(t *testing.T) {
	workspaceBase, repo := stageTestWorkspace(t)

	cfg := &Config{
		WebhookSecret: testWebhookSecret,
		WorkspaceBase: workspaceBase,
	}
	agent := &blockingAgent{entered: make(chan int64, 2), release: make(chan struct{})}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	go orchestrator.runStage(Task{Repo: repo, IdTask: 100, IdRun: 200, IdUserAgent: 1, IdIssue: 300, Stage: StageDesign, AttemptNo: 1})
	select {
	case <-agent.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first stage did not start")
	}

	queuedDone := make(chan struct{})
	go func() {
		orchestrator.runStage(Task{Repo: repo, IdTask: 101, IdRun: 201, IdUserAgent: 1, IdIssue: 301, Stage: StageDesign, AttemptNo: 1})
		close(queuedDone)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		orchestrator.mu.Lock()
		_, queued := orchestrator.activeTasks[101]
		orchestrator.mu.Unlock()
		if queued {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second stage was never queued")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "cancelled", Payload: map[string]any{"idRun": float64(201)}}); err != nil {
		t.Fatalf("HandleEvent(cancelled): %v", err)
	}
	select {
	case <-queuedDone:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled stage stayed in the queue")
	}

	close(agent.release)
	select {
	case id := <-agent.entered:
		t.Fatalf("agent ran the cancelled stage (id=%d)", id)
	case <-time.After(200 * time.Millisecond):
	}
}

type cancellableAgent struct {
	entered chan int64
	release chan struct{}
}

func (a *cancellableAgent) Run(ctx context.Context, task Task) (RunStats, error) {
	a.entered <- task.IdTask
	select {
	case <-a.release:
		return RunStats{}, nil
	case <-ctx.Done():
		return RunStats{}, errors.New("agent cancelled")
	}
}

func (a *cancellableAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func TestRunStage_ReleasesTheSlotWhenTheTrackerNoLongerRunsTheTask(t *testing.T) {
	const goneTask, queuedTask = int64(100), int64(101)
	var completedTasks sync.Map
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case fmt.Sprintf("/agent/task/%d/heartbeat", goneTask):
			writer.WriteHeader(http.StatusConflict)
			_, _ = writer.Write([]byte(`{"code":"TASK_NOT_RUNNING_BY_AGENT"}`))
			return
		case fmt.Sprintf("/agent/task/%d/complete", goneTask):
			completedTasks.Store(goneTask, true)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	workspaceBase, repo := stageTestWorkspace(t)
	cfg := &Config{
		TrackerAPIUrl: server.URL,
		WebhookSecret: testWebhookSecret,
		WorkspaceBase: workspaceBase,
	}
	agent := &cancellableAgent{entered: make(chan int64, 2), release: make(chan struct{})}
	t.Cleanup(func() { close(agent.release) })
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())
	orchestrator.heartbeatLoop.interval = 20 * time.Millisecond

	go orchestrator.runStage(Task{Repo: repo, IdTask: goneTask, IdRun: 200, IdUserAgent: 1, IdIssue: 300, Stage: StageDesign, AttemptNo: 1})
	select {
	case id := <-agent.entered:
		if id != goneTask {
			t.Fatalf("first Run was task %d, want %d", id, goneTask)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no agent.Run started within timeout")
	}
	go orchestrator.runStage(Task{Repo: repo, IdTask: queuedTask, IdRun: 201, IdUserAgent: 1, IdIssue: 301, Stage: StageDesign, AttemptNo: 1})

	select {
	case id := <-agent.entered:
		if id != queuedTask {
			t.Fatalf("second Run was task %d, want %d", id, queuedTask)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the queued task never got the slot after the tracker dropped the running one")
	}
	if _, ok := completedTasks.Load(goneTask); ok {
		t.Error("gateway posted complete_stage for a task the tracker already dropped")
	}
}

type submittingAgent struct {
	entered   chan struct{}
	release   chan struct{}
	cancelled chan struct{}
}

func (a *submittingAgent) Run(ctx context.Context, task Task) (RunStats, error) {
	task.ReportStageSubmitted()
	close(a.entered)
	select {
	case <-a.release:
		return RunStats{TokensUsed: 10}, nil
	case <-ctx.Done():
		close(a.cancelled)
		return RunStats{}, errors.New("agent cancelled")
	}
}

func (a *submittingAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func TestRunStage_KeepsAStageThatAlreadyCalledCompleteStage(t *testing.T) {
	var isStatsSent atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/agent/task/100/heartbeat":
			writer.WriteHeader(http.StatusConflict)
			_, _ = writer.Write([]byte(`{"code":"TASK_NOT_RUNNING_BY_AGENT"}`))
			return
		case "/agent/task/100/stats":
			isStatsSent.Store(true)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	workspaceBase, repo := stageTestWorkspace(t)
	repoPath, err := repo.LocalPath(workspaceBase)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		TrackerAPIUrl: server.URL,
		WebhookSecret: testWebhookSecret,
		WorkspaceBase: workspaceBase,
	}
	agent := &submittingAgent{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())
	orchestrator.heartbeatLoop.interval = 20 * time.Millisecond

	stageDone := make(chan struct{})
	go func() {
		orchestrator.runStage(Task{Repo: repo, IdTask: 100, IdRun: 200, IdUserAgent: 1, IdIssue: 300, Stage: StageDesign, AttemptNo: 1})
		close(stageDone)
	}()
	select {
	case <-agent.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("no agent.Run started within timeout")
	}

	select {
	case <-agent.cancelled:
		t.Fatal("agent was stopped although it had already called complete_stage")
	case <-time.After(200 * time.Millisecond):
	}
	close(agent.release)
	select {
	case <-stageDone:
	case <-time.After(2 * time.Second):
		t.Fatal("stage did not finish after the agent returned")
	}
	if !WorktreeExists(repoPath, 200) {
		t.Error("worktree of a submitted stage was removed")
	}
	if !isStatsSent.Load() {
		t.Error("stats of a submitted stage were not sent")
	}
}

func TestAgentErrorReason(t *testing.T) {
	// A typed *AgentError forwards its stable code + detail verbatim so the API
	// stores the code the frontend translates.
	provErr := &AgentError{Code: ErrCodeProviderCreditExhausted, Detail: "credit balance is too low"}
	reason, detail := agentErrorReason(provErr)
	if reason != ErrCodeProviderCreditExhausted {
		t.Errorf("reason = %q, want %q", reason, ErrCodeProviderCreditExhausted)
	}
	if detail != "credit balance is too low" {
		t.Errorf("detail = %q, want the AgentError detail", detail)
	}

	// A wrapped *AgentError is still unwrapped via errors.As.
	wrapped := fmt.Errorf("running goose: %w", &AgentError{Code: ErrCodeProviderError, Detail: "boom"})
	if r, _ := agentErrorReason(wrapped); r != ErrCodeProviderError {
		t.Errorf("wrapped reason = %q, want %q", r, ErrCodeProviderError)
	}

	// A plain error falls back to the generic code with the message as detail.
	reason, detail = agentErrorReason(errors.New("exited with code 2"))
	if reason != "agent_error" || detail != "exited with code 2" {
		t.Errorf("plain error = (%q,%q), want (agent_error, exited with code 2)", reason, detail)
	}
}

func TestParseStageExecutePayload_Minimal(t *testing.T) {
	payload := map[string]any{
		"idRun":       float64(42),
		"idIssue":     float64(7),
		"idUserAgent": float64(84),
		"idProject":   float64(1),
		"payload": map[string]any{
			"idTask":    float64(123),
			"stage":     "design",
			"attemptNo": float64(1),
			"contextBundle": map[string]any{
				"issue": map[string]any{
					"idIssuePublic": float64(11),
					"title":         "Fix login",
					"description":   "It crashes.",
				},
			},
		},
	}
	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.IdTask != 123 || task.IdRun != 42 || task.Stage != "design" || task.AttemptNo != 1 {
		t.Errorf("bad parse: %+v", task)
	}
	if task.IssueTitle != "Fix login" || task.IdIssuePublic != 11 {
		t.Errorf("context bundle not parsed: %+v", task)
	}
}

func TestParseStageExecutePayload_CarriesTheRepo(t *testing.T) {
	task, err := parseStageExecutePayload(map[string]any{
		"idRun": float64(42),
		"payload": map[string]any{
			"idTask": float64(123),
			"stage":  "implementation",
			"repo": map[string]any{
				"cloneUrl":       "https://gitlab.com/group/sub/repo.git",
				"host":           "gitlab.com",
				"repoPath":       "group/sub/repo",
				"baseBranch":     "develop",
				"encryptedToken": "c2VhbGVk",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := AgentRunGitRepo{CloneUrl: "https://gitlab.com/group/sub/repo.git", Host: "gitlab.com", RepoPath: "group/sub/repo", BaseBranch: "develop", EncryptedToken: "c2VhbGVk"}
	if task.Repo != want {
		t.Errorf("Repo = %+v, want %+v", task.Repo, want)
	}
}

func TestParseStageExecutePayload_CarriesPrBranch(t *testing.T) {
	payload := map[string]any{
		"idRun": float64(42),
		"payload": map[string]any{
			"idTask":   float64(123),
			"stage":    "implementation",
			"prBranch": "agent/a3/i48/1790053695",
		},
	}
	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.PrBranch != "agent/a3/i48/1790053695" {
		t.Errorf("PrBranch = %q, want the branch from the payload", task.PrBranch)
	}
}

func TestCreateRunWorktree_PicksBranch(t *testing.T) {
	const prBranch = "agent/a3/i48/1790053695"
	tests := []struct {
		name         string
		prBranch     string
		wantPrBranch bool
	}{
		{"resumes the open PR branch that exists on origin", prBranch, true},
		{"starts a new branch without a PR branch", "", false},
		{"starts a new branch when the PR branch is gone from origin", "agent/a3/i48/1", false},
		{"starts a new branch when the PR branch is not an agent branch", "main", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := newGitEnv(t)
			originPath, seed := seedOrigin(t, tmp, "main")
			pushAgentBranch(t, seed, prBranch)
			cfg := &Config{WorkspaceBase: filepath.Join(tmp, "ws")}
			repo := testRepo(originPath, "example.com", "main")
			repoPath := ensureRepo(t, cfg.WorkspaceBase, repo)
			orchestrator := NewOrchestrator(cfg, nil, NewTrackerClient(cfg), NewState())

			task := Task{Repo: repo, IdRun: 9, IdUserAgent: 3, IdIssue: 48, PrBranch: tc.prBranch}
			path, branch, err := orchestrator.createRunWorktree(repoPath, task)
			if err != nil {
				t.Fatalf("createRunWorktree: %v", err)
			}

			if tc.wantPrBranch {
				if branch != prBranch {
					t.Errorf("branch = %q, want the PR branch %q", branch, prBranch)
				}
				assertFile(t, filepath.Join(path, "first-attempt.txt"), "first attempt\n")
				return
			}
			if branch == prBranch || branch == tc.prBranch || !IsAgentBranch(branch) {
				t.Errorf("branch = %q, want a freshly generated agent branch", branch)
			}
			if got, want := headSha(t, path), headSha(t, seed); got != want {
				t.Errorf("new branch starts at %s, want origin/main %s", got, want)
			}
		})
	}
}

func threadPayload(thread []any) map[string]any {
	return map[string]any{
		"idRun":       float64(1),
		"idIssue":     float64(2),
		"idUserAgent": float64(3),
		"idProject":   float64(4),
		"payload": map[string]any{
			"idTask":        float64(5),
			"stage":         "implementation",
			"attemptNo":     float64(2),
			"contextBundle": map[string]any{"reviewThread": thread},
		},
	}
}

func TestParseStageExecutePayload_WithReviewThread(t *testing.T) {
	task, err := parseStageExecutePayload(threadPayload([]any{
		map[string]any{
			"message":     "Looks good but check edge case X.",
			"messageKind": MessageKindComment,
			"createdAt":   "2026-05-26T10:00:00Z",
			"creator":     map[string]any{"name": "Alice"},
		},
		map[string]any{
			"message":     "Pushed the fix.",
			"messageKind": MessageKindPullRequestPushed,
			"createdAt":   "2026-05-26T11:00:00Z",
			"creator":     map[string]any{"name": "Bot"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(task.Conversation) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(task.Conversation))
	}
	if task.Conversation[0].CreatorName != "Alice" || task.Conversation[0].Kind != MessageKindComment {
		t.Errorf("first message wrong: %+v", task.Conversation[0])
	}
	if task.Conversation[1].Kind != MessageKindPullRequestPushed || task.Conversation[1].Message != "Pushed the fix." {
		t.Errorf("agent output must stay in the thread: %+v", task.Conversation[1])
	}
}

func TestParseStageExecutePayload_ElidesArtifactBodiesInThread(t *testing.T) {
	task, err := parseStageExecutePayload(threadPayload([]any{
		map[string]any{
			"message":     "## Design\nLong document…",
			"messageKind": MessageKindDesign,
			"createdAt":   "2026-05-26T10:00:00Z",
			"creator":     map[string]any{"name": "Bot"},
		},
		map[string]any{
			"message":     "## Plan\nLong document…",
			"messageKind": MessageKindImplementationPlan,
			"createdAt":   "2026-05-26T10:30:00Z",
			"creator":     map[string]any{"name": "Bot"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, message := range task.Conversation {
		if strings.Contains(message.Message, "Long document") {
			t.Errorf("artifact body must not be repeated in the thread: %+v", message)
		}
		if message.Message == "" {
			t.Errorf("elided artifact still needs a placeholder so the order reads: %+v", message)
		}
	}
}

func TestParseStageExecutePayload_WithApprovedArtifacts(t *testing.T) {
	payload := map[string]any{
		"idRun":       float64(1),
		"idIssue":     float64(2),
		"idUserAgent": float64(3),
		"idProject":   float64(4),
		"payload": map[string]any{
			"idTask":    float64(5),
			"stage":     "implementation",
			"attemptNo": float64(1),
			"contextBundle": map[string]any{
				"approvedDesign":   map[string]any{"message": "DESIGN-MD", "messageKind": MessageKindDesign},
				"approvedImplPlan": map[string]any{"message": "PLAN-MD", "messageKind": MessageKindImplementationPlan},
			},
		},
	}
	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ApprovedDesign != "DESIGN-MD" {
		t.Errorf("ApprovedDesign = %q, want %q", task.ApprovedDesign, "DESIGN-MD")
	}
	if task.ApprovedImplPlan != "PLAN-MD" {
		t.Errorf("ApprovedImplPlan = %q, want %q", task.ApprovedImplPlan, "PLAN-MD")
	}
}

func TestParseStageExecutePayload_WithRejectedOutput(t *testing.T) {
	payload := map[string]any{
		"idRun":       float64(1),
		"idIssue":     float64(2),
		"idUserAgent": float64(3),
		"idProject":   float64(4),
		"payload": map[string]any{
			"idTask":    float64(5),
			"stage":     "design",
			"attemptNo": float64(2),
			"contextBundle": map[string]any{
				"rejectedOutput": map[string]any{"message": "OLD-DESIGN-MD", "messageKind": MessageKindDesign},
			},
		},
	}
	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.RejectedOutput != "OLD-DESIGN-MD" {
		t.Errorf("RejectedOutput = %q, want %q", task.RejectedOutput, "OLD-DESIGN-MD")
	}
}

func TestParseStageExecutePayload_ApprovedArtifactsAbsentOrNull(t *testing.T) {
	// A null/absent approvedDesign/approvedImplPlan must leave the fields empty,
	// not panic on the type assertion. The API ships null when no such artifact
	// exists yet (e.g. the design stage has no prior design).
	payload := map[string]any{
		"idRun":       float64(1),
		"idIssue":     float64(2),
		"idUserAgent": float64(3),
		"idProject":   float64(4),
		"payload": map[string]any{
			"idTask":    float64(5),
			"stage":     "design",
			"attemptNo": float64(1),
			"contextBundle": map[string]any{
				"approvedDesign":   nil,
				"approvedImplPlan": nil,
			},
		},
	}
	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ApprovedDesign != "" || task.ApprovedImplPlan != "" {
		t.Errorf("expected empty artifacts, got design=%q plan=%q", task.ApprovedDesign, task.ApprovedImplPlan)
	}
}

func TestParseStageExecutePayload_RejectsMissingIdTask(t *testing.T) {
	payload := map[string]any{
		"idRun":   float64(1),
		"payload": map[string]any{"stage": "design"},
	}
	if _, err := parseStageExecutePayload(payload); err == nil {
		t.Errorf("expected error for missing idTask")
	}
}

func TestParseStageExecutePayload_RejectsMissingStage(t *testing.T) {
	payload := map[string]any{
		"idRun":   float64(1),
		"payload": map[string]any{"idTask": float64(5)},
	}
	if _, err := parseStageExecutePayload(payload); err == nil {
		t.Errorf("expected error for missing stage")
	}
}

func TestStageConstantsMirrorAPI(t *testing.T) {
	// Hard-coded mirror check so a typo in a stage name across the API/gateway
	// boundary surfaces in tests instead of at runtime as an unhandled outcome.
	cases := []struct{ got, want string }{
		{StagePickup, "pickup"},
		{StageBrainstorming, "brainstorming"},
		{StageDesign, "design"},
		{StageImplementationPlan, "implementation_plan"},
		{StageImplementation, "implementation"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("stage constant drift: got %q want %q", c.got, c.want)
		}
	}
}

func TestMessageKindConstantsMirrorAPI(t *testing.T) {
	cases := []struct{ got, want string }{
		{MessageKindComment, "comment"},
		{MessageKindBrainstormingQuestion, "brainstorming_question"},
		{MessageKindBrainstormingComplete, "brainstorming_complete"},
		{MessageKindDesign, "design"},
		{MessageKindImplementationPlan, "implementation_plan"},
		{MessageKindPullRequestPushed, "pull_request_pushed"},
		{MessageKindImplementationDone, "implementation_done"},
		{MessageKindReviewReply, "review_reply"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("message kind constant drift: got %q want %q", c.got, c.want)
		}
	}
}

func TestOutcomeConstantsMirrorAPI(t *testing.T) {
	cases := []struct{ got, want string }{
		{OutcomeOutputSubmitted, "output_submitted"},
		{OutcomeQuestionAsked, "question_asked"},
		{OutcomeNoActionNeeded, "no_action_needed"},
		{OutcomeErrored, "errored"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("outcome constant drift: got %q want %q", c.got, c.want)
		}
	}
}
