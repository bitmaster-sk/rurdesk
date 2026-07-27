package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func (a *blockingAgent) Cancel(ctx context.Context, runID RunID) error { return nil }

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestRunStage_SerializesToMaxConcurrent verifies the gateway runs at most
// MaxConcurrent stage attempts at once, queueing the rest — the property
// that keeps concurrent CLI processes off the shared OAuth credential.
func TestRunStage_SerializesToMaxConcurrent(t *testing.T) {
	workspaceBase := t.TempDir()
	repoURL := "https://example.com/acme/myrepo.git"
	initGitRepo(t, RepoPathFromURL(workspaceBase, repoURL))

	cfg := &Config{
		WorkspaceBase: workspaceBase,
		RepoUrl:       repoURL,
		MaxConcurrent: 1,
	}
	agent := &blockingAgent{entered: make(chan int64, 3), release: make(chan struct{})}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	const taskCount = 3
	for i := 0; i < taskCount; i++ {
		task := Task{
			IdTask:    int64(100 + i),
			IdRun:     int64(200 + i),
			IdUserBot: 1,
			IdIssue:   int64(300 + i), // distinct issue → distinct branch
			Stage:     StageDesign,
			AttemptNo: 1,
		}
		go orchestrator.runStage(task)
	}

	// Exactly one Run is allowed in flight with MaxConcurrent=1.
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
		"idRun":     float64(42),
		"idIssue":   float64(7),
		"idUserBot": float64(84),
		"idProject": float64(1),
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

func TestParseStageExecutePayload_WithComments(t *testing.T) {
	payload := map[string]any{
		"idRun":     float64(1),
		"idIssue":   float64(2),
		"idUserBot": float64(3),
		"idProject": float64(4),
		"payload": map[string]any{
			"idTask":    float64(5),
			"stage":     "brainstorming",
			"attemptNo": float64(2),
			"contextBundle": map[string]any{
				"pendingComments": []any{
					map[string]any{
						"message":   "Looks good but check edge case X.",
						"createdAt": "2026-05-26T10:00:00Z",
						"creator":   map[string]any{"name": "Alice"},
					},
				},
			},
		},
	}
	task, err := parseStageExecutePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(task.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(task.Comments))
	}
	if task.Comments[0].CreatorName != "Alice" || task.Comments[0].Message == "" {
		t.Errorf("comment fields wrong: %+v", task.Comments[0])
	}
}

func TestParseStageExecutePayload_WithApprovedArtifacts(t *testing.T) {
	payload := map[string]any{
		"idRun":     float64(1),
		"idIssue":   float64(2),
		"idUserBot": float64(3),
		"idProject": float64(4),
		"payload": map[string]any{
			"idTask":    float64(5),
			"stage":     "implementation",
			"attemptNo": float64(1),
			"contextBundle": map[string]any{
				"approvedDesign":   map[string]any{"message": "DESIGN-MD", "messageKind": "design"},
				"approvedImplPlan": map[string]any{"message": "PLAN-MD", "messageKind": "implementation_plan"},
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
		"idRun":     float64(1),
		"idIssue":   float64(2),
		"idUserBot": float64(3),
		"idProject": float64(4),
		"payload": map[string]any{
			"idTask":    float64(5),
			"stage":     "design",
			"attemptNo": float64(2),
			"contextBundle": map[string]any{
				"rejectedOutput": map[string]any{"message": "OLD-DESIGN-MD", "messageKind": "design"},
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
		"idRun":     float64(1),
		"idIssue":   float64(2),
		"idUserBot": float64(3),
		"idProject": float64(4),
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
