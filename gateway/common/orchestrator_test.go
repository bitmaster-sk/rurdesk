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
	"reflect"
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

func unmarshalEvent(t *testing.T, body string) WebhookEvent {
	t.Helper()
	var event WebhookEvent
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	event.Type = "stage_execute"
	return event
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

	if err := orchestrator.HandleEvent(WebhookEvent{Type: "cancelled", IdRun: 201}); err != nil {
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
	agent := &submittingAgent{entered: make(chan struct{}), release: make(chan struct{}), cancelled: make(chan struct{})}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/agent/task/100/heartbeat":
			select {
			case <-agent.entered:
				writer.WriteHeader(http.StatusConflict)
				_, _ = writer.Write([]byte(`{"code":"TASK_NOT_RUNNING_BY_AGENT"}`))
				return
			default:
			}
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
	event := unmarshalEvent(t, `{
		"idRun": 42,
		"idIssue": 7,
		"idUserAgent": 84,
		"idProject": 1,
		"payload": {
			"idTask": 123,
			"stage": "design",
			"attemptNo": 1,
			"contextBundle": {
				"issue": {
					"idIssuePublic": 11,
					"title": "Fix login",
					"description": "It crashes."
				}
			}
		}
	}`)
	task, err := parseStageExecutePayload(event)
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
	event := unmarshalEvent(t, `{
		"idRun": 42,
		"payload": {
			"idTask": 123,
			"stage": "implementation",
			"repo": {
				"cloneUrl": "https://gitlab.com/group/sub/repo.git",
				"host": "gitlab.com",
				"repoPath": "group/sub/repo",
				"baseBranch": "develop",
				"encryptedToken": "c2VhbGVk"
			}
		}
	}`)
	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := AgentRunGitRepo{CloneUrl: "https://gitlab.com/group/sub/repo.git", Host: "gitlab.com", RepoPath: "group/sub/repo", BaseBranch: "develop", EncryptedToken: "c2VhbGVk"}
	if task.Repo != want {
		t.Errorf("Repo = %+v, want %+v", task.Repo, want)
	}
}

func TestParseStageExecutePayload_CarriesPrBranch(t *testing.T) {
	event := unmarshalEvent(t, `{
		"idRun": 42,
		"payload": {
			"idTask": 123,
			"stage": "implementation",
			"prBranch": "agent/a3/i48/1790053695"
		}
	}`)
	task, err := parseStageExecutePayload(event)
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

func threadPayload(thread string) string {
	return `{
		"idRun": 1,
		"idIssue": 2,
		"idUserAgent": 3,
		"idProject": 4,
		"payload": {
			"idTask": 5,
			"stage": "implementation",
			"attemptNo": 2,
			"contextBundle": {"reviewThread": ` + thread + `}
		}
	}`
}

func TestParseStageExecutePayload_WithReviewThread(t *testing.T) {
	event := unmarshalEvent(t, threadPayload(`[
		{
			"message": "Looks good but check edge case X.",
			"messageKind": "comment",
			"createdAt": "2026-05-26T10:00:00Z",
			"creator": {"name": "Alice"}
		},
		{
			"message": "Pushed the fix.",
			"messageKind": "pull_request_pushed",
			"createdAt": "2026-05-26T11:00:00Z",
			"creator": {"name": "Bot"}
		}
	]`))
	task, err := parseStageExecutePayload(event)
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
	event := unmarshalEvent(t, threadPayload(`[
		{
			"message": "## Design\nLong document…",
			"messageKind": "design",
			"createdAt": "2026-05-26T10:00:00Z",
			"creator": {"name": "Bot"}
		},
		{
			"message": "## Plan\nLong document…",
			"messageKind": "implementation_plan",
			"createdAt": "2026-05-26T10:30:00Z",
			"creator": {"name": "Bot"}
		}
	]`))
	task, err := parseStageExecutePayload(event)
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
	event := unmarshalEvent(t, `{
		"idRun": 1,
		"idIssue": 2,
		"idUserAgent": 3,
		"idProject": 4,
		"payload": {
			"idTask": 5,
			"stage": "implementation",
			"attemptNo": 1,
			"contextBundle": {
				"approvedDesign": {"message": "DESIGN-MD", "messageKind": "design"},
				"approvedImplPlan": {"message": "PLAN-MD", "messageKind": "implementation_plan"}
			}
		}
	}`)
	task, err := parseStageExecutePayload(event)
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
	event := unmarshalEvent(t, `{
		"idRun": 1,
		"idIssue": 2,
		"idUserAgent": 3,
		"idProject": 4,
		"payload": {
			"idTask": 5,
			"stage": "design",
			"attemptNo": 2,
			"contextBundle": {
				"rejectedOutput": {"message": "OLD-DESIGN-MD", "messageKind": "design"}
			}
		}
	}`)
	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.RejectedOutput != "OLD-DESIGN-MD" {
		t.Errorf("RejectedOutput = %q, want %q", task.RejectedOutput, "OLD-DESIGN-MD")
	}
}

func TestParseStageExecutePayload_ApprovedArtifactsAbsentOrNull(t *testing.T) {
	// A null/absent approvedDesign/approvedImplPlan must leave the fields empty.
	// The API ships null when no such artifact exists yet (e.g. the design
	// stage has no prior design).
	event := unmarshalEvent(t, `{
		"idRun": 1,
		"idIssue": 2,
		"idUserAgent": 3,
		"idProject": 4,
		"payload": {
			"idTask": 5,
			"stage": "design",
			"attemptNo": 1,
			"contextBundle": {
				"approvedDesign": null,
				"approvedImplPlan": null
			}
		}
	}`)
	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ApprovedDesign != "" || task.ApprovedImplPlan != "" {
		t.Errorf("expected empty artifacts, got design=%q plan=%q", task.ApprovedDesign, task.ApprovedImplPlan)
	}
}

func TestParseStageExecutePayload_RejectsMissingIdTask(t *testing.T) {
	event := unmarshalEvent(t, `{
		"idRun": 1,
		"payload": {"stage": "design"}
	}`)
	if _, err := parseStageExecutePayload(event); err == nil {
		t.Errorf("expected error for missing idTask")
	}
}

func TestParseStageExecutePayload_RejectsMissingStage(t *testing.T) {
	event := unmarshalEvent(t, `{
		"idRun": 1,
		"payload": {"idTask": 5}
	}`)
	if _, err := parseStageExecutePayload(event); err == nil {
		t.Errorf("expected error for missing stage")
	}
}

func TestParseStageExecutePayload_RealTrackerPayload(t *testing.T) {
	body := `{
		"idRun": 42,
		"idProject": 1,
		"idIssue": 7,
		"idUserAgent": 3,
		"event": "stage_execute",
		"sequence": 5,
		"payload": {
			"idTask": 123,
			"stage": "implementation",
			"attemptNo": 1,
			"stagePlan": "{\"stages\":[{\"stage\":\"design\"}]}",
			"prBranch": "agent/a3/i7/1792731948",
			"repo": {
				"cloneUrl": "https://github.com/acme/project.git",
				"host": "github.com",
				"repoPath": "acme/project",
				"baseBranch": "main",
				"encryptedToken": "c2VhbGVkLXNlY3JldA=="
			},
			"model": "openai/gpt-4o",
			"maxTurns": 300,
			"contextBundle": {
				"issue": {
					"idIssue": 7,
					"idIssuePublic": 42,
					"title": "Add typed webhook parsing",
					"description": "Replace map[string]any with typed structs."
				},
				"project": {
					"idProject": 1,
					"name": "Rurdesk",
					"color": "#3366cc"
				},
				"agent": {
					"idUser": 3,
					"name": "goose",
					"email": "goose@example.com",
					"isAgent": true
				},
				"reviewThread": [
					{
						"idMessage": 100,
						"message": "Please implement this.",
						"messageKind": "comment",
						"createdAt": "2026-10-22T09:00:00Z",
						"creator": {"name": "Alice"}
					},
					{
						"idMessage": 101,
						"message": "## Design\nApproved design body.",
						"messageKind": "design",
						"createdAt": "2026-10-22T10:00:00Z",
						"creator": {"name": "goose"}
					}
				],
				"approvedDesign": {"message": "## Design\nApproved design body.", "messageKind": "design"},
				"approvedImplPlan": {"message": "## Plan\nApproved plan body.", "messageKind": "implementation_plan"},
				"rejectedOutput": null,
				"approvedMockupRef": "mockup-A",
				"skills": [
					{"idSkill": 1, "name": "pr-rules", "content": "Pull request rules."},
					{"idSkill": 2, "name": "repo-rules", "content": "Repository rules."}
				],
				"wiki": {
					"pages": [
						{"slug": "overview", "title": "Overview", "version": 3, "reason": "linked", "body": "# Overview\nProject overview."}
					],
					"index": [
						{"slug": "architecture", "title": "Architecture", "summary": "Architecture notes.", "overLimit": false}
					],
					"indexMore": 0
				},
				"wikiProposals": [
					{"id": 9, "kind": "update", "slug": "gateway-webhook-protocol", "title": "Gateway Webhook Protocol", "parent": null, "reason": "payload types", "baseVersion": 1, "state": "open", "updated": false}
				],
				"isShortWorkflow": false,
				"previousFailedAttempt": null
			}
		}
	}`
	event := unmarshalEvent(t, body)
	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.IdRun != 42 || task.IdProject != 1 || task.IdIssue != 7 || task.IdUserAgent != 3 {
		t.Errorf("envelope fields not parsed: %+v", task)
	}
	if task.IdTask != 123 || task.Stage != StageImplementation || task.AttemptNo != 1 {
		t.Errorf("payload fields not parsed: %+v", task)
	}
	if task.PrBranch != "agent/a3/i7/1792731948" {
		t.Errorf("PrBranch = %q", task.PrBranch)
	}
	wantRepo := AgentRunGitRepo{
		CloneUrl:       "https://github.com/acme/project.git",
		Host:           "github.com",
		RepoPath:       "acme/project",
		BaseBranch:     "main",
		EncryptedToken: "c2VhbGVkLXNlY3JldA==",
	}
	if task.Repo != wantRepo {
		t.Errorf("Repo = %+v, want %+v", task.Repo, wantRepo)
	}
	if task.RequestedModel != "openai/gpt-4o" || task.MaxTurns != 300 {
		t.Errorf("model/turns = %q/%d, want openai/gpt-4o/300", task.RequestedModel, task.MaxTurns)
	}
	if task.IdIssuePublic != 42 || task.IssueTitle != "Add typed webhook parsing" {
		t.Errorf("issue context not parsed: %+v", task)
	}
	if task.ApprovedDesign == "" || task.ApprovedImplPlan == "" {
		t.Errorf("approved artifacts missing: design=%q plan=%q", task.ApprovedDesign, task.ApprovedImplPlan)
	}
	if task.RejectedOutput != "" {
		t.Errorf("rejected output should be empty when null, got %q", task.RejectedOutput)
	}
	if task.ApprovedMockupRef != "mockup-A" {
		t.Errorf("ApprovedMockupRef = %q", task.ApprovedMockupRef)
	}
	if len(task.Skills) != 2 {
		t.Errorf("skills count = %d", len(task.Skills))
	}
	if task.Wiki == nil || len(task.Wiki.Pages) != 1 || len(task.Wiki.Index) != 1 {
		t.Errorf("wiki context not parsed: %+v", task.Wiki)
	}
	if len(task.WikiProposals) != 1 || task.WikiProposals[0].Slug != "gateway-webhook-protocol" {
		t.Errorf("wiki proposals not parsed: %+v", task.WikiProposals)
	}
	if len(task.Conversation) != 2 {
		t.Errorf("conversation length = %d", len(task.Conversation))
	}
	if task.IsShortWorkflow {
		t.Errorf("IsShortWorkflow should be false")
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
		{StageReleaseNotes, "release_notes"},
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

type workspaceRecordingAgent struct {
	workspace chan string
	entries   chan int
}

func (a *workspaceRecordingAgent) Run(_ context.Context, task Task) (RunStats, error) {
	entries, _ := os.ReadDir(task.WorktreePath)
	a.entries <- len(entries)
	a.workspace <- task.WorktreePath
	return RunStats{}, nil
}

func (a *workspaceRecordingAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func TestRunStage_ReleaseNotesRunsInAScratchDirectoryWithoutARepo(t *testing.T) {
	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tracker.Close()
	cfg := &Config{WebhookSecret: testWebhookSecret, TrackerAPIUrl: tracker.URL, WorkspaceBase: filepath.Join(t.TempDir(), "never-created")}
	agent := &workspaceRecordingAgent{workspace: make(chan string, 1), entries: make(chan int, 1)}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	orchestrator.runStage(Task{IdTask: 1, IdRun: 2, Stage: StageReleaseNotes, AttemptNo: 1})

	select {
	case workspace := <-agent.workspace:
		if workspace == "" {
			t.Fatal("the agent got no working directory")
		}
		if entries := <-agent.entries; entries != 0 {
			t.Errorf("the scratch directory held %d entries, want none", entries)
		}
		if _, err := os.Stat(workspace); !os.IsNotExist(err) {
			t.Errorf("the scratch directory %s is left behind", workspace)
		}
	default:
		t.Fatal("the agent never ran: a release notes stage must not be rejected for its missing repo")
	}
	if _, err := os.Stat(cfg.WorkspaceBase); !os.IsNotExist(err) {
		t.Error("a release notes stage must not touch the repo workspace")
	}
}

func TestParseStageExecutePayload_CarriesTheRelease(t *testing.T) {
	event := unmarshalEvent(t, `{
		"idRun": 42,
		"payload": {
			"idTask": 123,
			"stage": "release_notes",
			"repo": null,
			"contextBundle": {
				"release": {
					"name": "v1.0.4",
					"tag": "next",
					"openIssues": [{"idIssue": 11, "idIssuePublic": 106, "title": "Half done"}],
					"sections": [
						{"idIssueType": 2, "name": "Feature", "issues": [{"idIssue": 9, "idIssuePublic": 104, "title": "Release notes"}]},
						{"idIssueType": null, "name": null, "issues": [{"idIssue": 10, "idIssuePublic": 105, "title": "Bump deps"}]}
					]
				}
			}
		}
	}`)
	task, err := parseStageExecutePayload(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Repo.IsSet() {
		t.Errorf("Repo = %+v, want none", task.Repo)
	}
	want := &ReleaseContext{
		Name:       "v1.0.4",
		Tag:        "next",
		OpenIssues: []ReleaseIssue{{IdIssuePublic: 106, Title: "Half done"}},
		Sections: []ReleaseSection{
			{Name: "Feature", Issues: []ReleaseIssue{{IdIssuePublic: 104, Title: "Release notes"}}},
			{Name: "", Issues: []ReleaseIssue{{IdIssuePublic: 105, Title: "Bump deps"}}},
		},
	}
	if !reflect.DeepEqual(task.Release, want) {
		t.Errorf("Release = %+v, want %+v", task.Release, want)
	}
}
