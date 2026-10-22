package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type recordingAgent struct {
	mu    sync.Mutex
	tasks []Task
}

func (a *recordingAgent) Run(_ context.Context, task Task) (RunStats, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tasks = append(a.tasks, task)
	task.ReportStageSubmitted()
	return RunStats{}, nil
}

func (a *recordingAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func (a *recordingAgent) ran() []Task {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Task(nil), a.tasks...)
}

type trackerRecorder struct {
	mu        sync.Mutex
	completes map[string]CompleteStagePayload
}

func newTrackerRecorder(t *testing.T) (*trackerRecorder, string) {
	t.Helper()
	recorder := &trackerRecorder{completes: map[string]CompleteStagePayload{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload CompleteStagePayload
		if err := json.NewDecoder(request.Body).Decode(&payload); err == nil {
			recorder.mu.Lock()
			recorder.completes[request.URL.Path] = payload
			recorder.mu.Unlock()
		}
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return recorder, server.URL
}

func (r *trackerRecorder) complete(path string) (CompleteStagePayload, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	payload, ok := r.completes[path]
	return payload, ok
}

func modelTestOrchestrator(t *testing.T, trackerURL string, maxTurnsLimit int) (*Orchestrator, *recordingAgent, AgentRunGitRepo) {
	t.Helper()
	workspaceBase, repo := stageTestWorkspace(t)
	catalog, err := parseModelCatalog("goose", "openai/a,ollama/b:8b", "openai/a")
	if err != nil {
		t.Fatalf("parseModelCatalog: %v", err)
	}
	cfg := &Config{
		TrackerAPIUrl: trackerURL,
		WebhookSecret: testWebhookSecret,
		WorkspaceBase: workspaceBase,
		Models:        catalog,
		MaxTurnsLimit: maxTurnsLimit,
	}
	agent := &recordingAgent{}
	return NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState()), agent, repo
}

func TestParseStageExecutePayload_CarriesModelAndMaxTurns(t *testing.T) {
	task, err := parseStageExecutePayload(map[string]any{
		"idRun": float64(42),
		"payload": map[string]any{
			"idTask":   float64(123),
			"stage":    "design",
			"model":    "ollama/b:8b",
			"maxTurns": float64(80),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.RequestedModel != "ollama/b:8b" || task.MaxTurns != 80 {
		t.Errorf("RequestedModel = %q, MaxTurns = %d", task.RequestedModel, task.MaxTurns)
	}
}

func TestRunStage_RunsTheAgentWithTheRequestedModelAndTurns(t *testing.T) {
	_, trackerURL := newTrackerRecorder(t)
	orchestrator, agent, repo := modelTestOrchestrator(t, trackerURL, 0)

	orchestrator.runStage(Task{Repo: repo, IdTask: 1, IdRun: 2, IdUserAgent: 1, IdIssue: 3, Stage: StageDesign, RequestedModel: "ollama/b:8b", MaxTurns: 40})

	ran := agent.ran()
	if len(ran) != 1 {
		t.Fatalf("agent ran %d times, want 1", len(ran))
	}
	if ran[0].Model != (Model{Provider: "ollama", Name: "b:8b"}) || ran[0].MaxTurns != 40 {
		t.Errorf("agent got model %+v and %d turns", ran[0].Model, ran[0].MaxTurns)
	}
}

func TestRunStage_UsesTheGatewayDefaultsWhenTheTrackerSendsNone(t *testing.T) {
	tests := []struct {
		stage    string
		limit    int
		wantTurn int
	}{
		{StageDesign, 0, 250},
		{StageImplementation, 0, 500},
		{StageImplementation, 300, 300},
	}
	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			_, trackerURL := newTrackerRecorder(t)
			orchestrator, agent, repo := modelTestOrchestrator(t, trackerURL, tt.limit)

			orchestrator.runStage(Task{Repo: repo, IdTask: 1, IdRun: 2, IdUserAgent: 1, IdIssue: 3, Stage: tt.stage})

			ran := agent.ran()
			if len(ran) != 1 {
				t.Fatalf("agent ran %d times, want 1", len(ran))
			}
			if ran[0].Model.Id() != "openai/a" || ran[0].MaxTurns != tt.wantTurn {
				t.Errorf("agent got model %q and %d turns, want openai/a and %d", ran[0].Model.Id(), ran[0].MaxTurns, tt.wantTurn)
			}
		})
	}
}

func TestRunStage_RejectsWhatTheGatewayDoesNotAllow(t *testing.T) {
	tests := []struct {
		name       string
		task       Task
		wantReason string
	}{
		{"model outside MODELS", Task{RequestedModel: "anthropic/opus"}, ErrCodeModelNotAllowed},
		{"turns over MAX_TURNS_LIMIT", Task{MaxTurns: 301}, ErrCodeMaxTurnsOverLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder, trackerURL := newTrackerRecorder(t)
			orchestrator, agent, repo := modelTestOrchestrator(t, trackerURL, 300)
			task := tt.task
			task.Repo = repo
			task.IdTask, task.IdRun, task.IdUserAgent, task.IdIssue, task.Stage = 1, 2, 1, 3, StageDesign

			orchestrator.runStage(task)

			if ran := agent.ran(); len(ran) != 0 {
				t.Fatalf("agent ran %d times, want it never started", len(ran))
			}
			complete, ok := recorder.complete("/agent/task/1/complete")
			if !ok {
				t.Fatal("gateway did not report the rejected stage to the tracker")
			}
			if complete.Outcome != OutcomeErrored || complete.ErrorReason != tt.wantReason || complete.ErrorDetail == "" {
				t.Errorf("complete = %+v, want errored with %s and a detail", complete, tt.wantReason)
			}
		})
	}
}

func TestRunStage_FailsAStageWithoutARepo(t *testing.T) {
	recorder, trackerURL := newTrackerRecorder(t)
	orchestrator, agent, _ := modelTestOrchestrator(t, trackerURL, 0)

	orchestrator.runStage(Task{IdTask: 1, IdRun: 2, IdUserAgent: 1, IdIssue: 3, Stage: StageDesign})

	if ran := agent.ran(); len(ran) != 0 {
		t.Fatalf("agent ran %d times, want it never started", len(ran))
	}
	complete, ok := recorder.complete("/agent/task/1/complete")
	if !ok || complete.Outcome != OutcomeErrored || complete.ErrorReason != ErrCodeTrackerMissingRepo {
		t.Errorf("complete = %+v (sent %v), want errored with %s", complete, ok, ErrCodeTrackerMissingRepo)
	}
}

func TestRunStage_FailsAStageWhoseGitTokenTheGatewayCannotRead(t *testing.T) {
	recorder, trackerURL := newTrackerRecorder(t)
	orchestrator, agent, repo := modelTestOrchestrator(t, trackerURL, 0)
	repo.EncryptedToken = encryptGitTokenForTest([]byte("another-gateway-secret"), testGitToken)

	orchestrator.runStage(Task{Repo: repo, IdTask: 1, IdRun: 2, IdUserAgent: 1, IdIssue: 3, Stage: StageDesign})

	if ran := agent.ran(); len(ran) != 0 {
		t.Fatalf("agent ran %d times, want it never started", len(ran))
	}
	complete, ok := recorder.complete("/agent/task/1/complete")
	if !ok || complete.Outcome != OutcomeErrored || complete.ErrorReason != ErrCodeGitTokenUnreadable {
		t.Errorf("complete = %+v (sent %v), want errored with %s", complete, ok, ErrCodeGitTokenUnreadable)
	}
}

type failingAgent struct {
	detail string
}

func (a *failingAgent) Run(_ context.Context, _ Task) (RunStats, error) {
	return RunStats{}, &AgentError{Code: ErrCodeProviderError, Detail: a.detail}
}

func (a *failingAgent) Cancel(_ context.Context, _ RunID) error { return nil }

func TestRunStage_MasksTheGitTokenInWhatItReportsToTheTracker(t *testing.T) {
	recorder, trackerURL := newTrackerRecorder(t)
	workspaceBase, repo := stageTestWorkspace(t)
	catalog, err := parseModelCatalog("goose", "openai/a", "openai/a")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{TrackerAPIUrl: trackerURL, WebhookSecret: testWebhookSecret, WorkspaceBase: workspaceBase, Models: catalog}
	agent := &failingAgent{detail: "remote: https://x-token:" + repo.Token + "@example.com rejected"}
	orchestrator := NewOrchestrator(cfg, agent, NewTrackerClient(cfg), NewState())

	orchestrator.runStage(Task{Repo: repo, IdTask: 1, IdRun: 2, IdUserAgent: 1, IdIssue: 3, Stage: StageDesign})

	complete, ok := recorder.complete("/agent/task/1/complete")
	if !ok {
		t.Fatal("gateway did not report the failed stage")
	}
	if strings.Contains(complete.ErrorDetail, repo.Token) || !strings.Contains(complete.ErrorDetail, "x-token:***@") {
		t.Errorf("ErrorDetail = %q, want the git token masked", complete.ErrorDetail)
	}
}
