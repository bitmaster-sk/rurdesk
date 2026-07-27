package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

func planJSON(t *testing.T, skipBrainstorming, skipDesign bool) json.RawMessage {
	t.Helper()
	plan := model.StagePlan{
		Stages: []model.StagePlanEntry{
			{Name: constants.StagePickup, Skip: false},
			{Name: constants.StageBrainstorming, Skip: skipBrainstorming},
			{Name: constants.StageDesign, Skip: skipDesign},
			{Name: constants.StageImplementationPlan, Skip: false},
			{Name: constants.StageImplementation, Skip: false},
		},
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	return b
}

func TestResolveNextStage_NoTasks_ReturnsFirstStage(t *testing.T) {
	run := &model.AgentRun{StagePlan: planJSON(t, false, false)}
	next, err := ResolveNextStage(run, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != constants.StagePickup {
		t.Errorf("got %q, want %q", next, constants.StagePickup)
	}
}

func TestResolveNextStage_SkipsSkippedStages(t *testing.T) {
	run := &model.AgentRun{StagePlan: planJSON(t, true, true)}
	tasks := []*model.AgentTask{
		{Stage: constants.StagePickup, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: time.Now()},
	}
	next, err := ResolveNextStage(run, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != constants.StageImplementationPlan {
		t.Errorf("got %q, want %q (brainstorming + design skipped)", next, constants.StageImplementationPlan)
	}
}

func TestResolveNextStage_FailedAttempt_RetriesSameStage(t *testing.T) {
	run := &model.AgentRun{StagePlan: planJSON(t, false, false)}
	tasks := []*model.AgentTask{
		{Stage: constants.StagePickup, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: time.Now().Add(-2 * time.Minute)},
		{Stage: constants.StageBrainstorming, Status: constants.TaskStatusFailed, AttemptNo: 1, CreatedAt: time.Now()},
	}
	next, err := ResolveNextStage(run, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != constants.StageBrainstorming {
		t.Errorf("got %q, want %q (failed brainstorming should retry)", next, constants.StageBrainstorming)
	}
}

func TestResolveNextStage_AllCompleted_ReturnsEmpty(t *testing.T) {
	run := &model.AgentRun{StagePlan: planJSON(t, false, false)}
	now := time.Now()
	tasks := []*model.AgentTask{
		{Stage: constants.StagePickup, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: now.Add(-5 * time.Minute)},
		{Stage: constants.StageBrainstorming, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: now.Add(-4 * time.Minute)},
		{Stage: constants.StageDesign, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: now.Add(-3 * time.Minute)},
		{Stage: constants.StageImplementationPlan, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: now.Add(-2 * time.Minute)},
		{Stage: constants.StageImplementation, Status: constants.TaskStatusCompleted, AttemptNo: 1, CreatedAt: now.Add(-1 * time.Minute)},
	}
	next, err := ResolveNextStage(run, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != "" {
		t.Errorf("got %q, want \"\" (all stages completed)", next)
	}
}

func TestResolveNextAttemptNo_NoExistingAttempt_ReturnsOne(t *testing.T) {
	if n := ResolveNextAttemptNo(nil, constants.StageDesign); n != 1 {
		t.Errorf("got %d, want 1", n)
	}
}

func TestResolveNextAttemptNo_IncrementsPerStage(t *testing.T) {
	tasks := []*model.AgentTask{
		{Stage: constants.StageDesign, AttemptNo: 1, Status: constants.TaskStatusFailed},
		{Stage: constants.StageDesign, AttemptNo: 2, Status: constants.TaskStatusFailed},
		{Stage: constants.StageBrainstorming, AttemptNo: 1, Status: constants.TaskStatusCompleted},
	}
	if n := ResolveNextAttemptNo(tasks, constants.StageDesign); n != 3 {
		t.Errorf("design next attempt: got %d, want 3", n)
	}
	if n := ResolveNextAttemptNo(tasks, constants.StageBrainstorming); n != 2 {
		t.Errorf("brainstorming next attempt: got %d, want 2", n)
	}
	if n := ResolveNextAttemptNo(tasks, constants.StageImplementation); n != 1 {
		t.Errorf("impl next attempt (no priors): got %d, want 1", n)
	}
}

func TestResolveNextStage_BadJSONFails(t *testing.T) {
	run := &model.AgentRun{StagePlan: json.RawMessage(`{not json`)}
	if _, err := ResolveNextStage(run, nil); err == nil {
		t.Errorf("expected error for malformed stage_plan")
	}
}
