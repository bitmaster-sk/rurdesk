package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

func fullStagePlan() json.RawMessage {
	plan := model.StagePlan{Stages: []model.StagePlanEntry{
		{Name: constants.StagePickup},
		{Name: constants.StageBrainstorming, Skippable: true},
		{Name: constants.StageDesign, Skippable: true},
		{Name: constants.StageImplementationPlan},
		{Name: constants.StageImplementation},
	}}
	b, _ := json.Marshal(plan)
	return b
}

func at(min int) *time.Time {
	t := time.Date(2026, 5, 26, 19, min, 0, 0, time.UTC)
	return &t
}

func completedTask(stage string, finished *time.Time, output *int64) *model.AgentTask {
	return &model.AgentTask{
		Stage: stage, AttemptNo: 1, Status: constants.TaskStatusCompleted,
		FinishedAt: finished, IdResultMessage: output, CreatedAt: *finished,
	}
}

func approvalEvent(min int) *model.AgentRunEvent {
	reason := "approved"
	return &model.AgentRunEvent{
		ActorType: constants.ActorTypeUser, Reason: &reason, CreatedAt: *at(min),
	}
}

func stageByName(rows []model.AgentStageProgress, name string) model.AgentStageProgress {
	for _, r := range rows {
		if r.Stage == name {
			return r
		}
	}
	return model.AgentStageProgress{}
}

func TestBuildStageProgress_FailedStageCarriesErrorCode(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseFailed, StagePlan: fullStagePlan()}
	reason := "provider_credit_exhausted"
	detail := "Request failed: credit balance is too low"
	tasks := []*model.AgentTask{
		completedTask(constants.StagePickup, at(19), nil),
		{
			Stage: constants.StageBrainstorming, AttemptNo: 1,
			Status:      constants.TaskStatusFailed,
			FinishedAt:  at(20),
			CreatedAt:   *at(20),
			ErrorReason: &reason,
			ErrorDetail: &detail,
		},
	}

	rows := BuildStageProgress(run, tasks, nil)
	got := stageByName(rows, constants.StageBrainstorming)
	if got.Status != "failed" {
		t.Fatalf("brainstorming status = %q, want failed", got.Status)
	}
	if got.ErrorReason == nil || *got.ErrorReason != reason {
		t.Errorf("ErrorReason = %v, want %q", got.ErrorReason, reason)
	}
	if got.ErrorDetail == nil || *got.ErrorDetail != detail {
		t.Errorf("ErrorDetail = %v, want %q", got.ErrorDetail, detail)
	}
}

func TestBuildStageProgress_HappyPathFullRun(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseDone, StagePlan: fullStagePlan()}
	msgID := int64(1)
	tasks := []*model.AgentTask{
		completedTask(constants.StagePickup, at(19), nil),
		completedTask(constants.StageBrainstorming, at(20), nil), // no output -> no_clarifications
		completedTask(constants.StageDesign, at(22), &msgID),
		completedTask(constants.StageImplementationPlan, at(26), &msgID),
		completedTask(constants.StageImplementation, at(32), &msgID),
	}
	events := []*model.AgentRunEvent{approvalEvent(24), approvalEvent(30)}

	rows := BuildStageProgress(run, tasks, events)
	if len(rows) != 5 {
		t.Fatalf("want 5 rows, got %d", len(rows))
	}

	if got := stageByName(rows, constants.StageBrainstorming); got.Status != "done" || got.Note != stageNoteNoClarifications {
		t.Errorf("brainstorming = %+v, want done/no_clarifications", got)
	}
	design := stageByName(rows, constants.StageDesign)
	if design.Status != "done" || design.Note != stageNoteSubmitted || design.ApprovedAt == nil {
		t.Errorf("design = %+v, want done/submitted/approved", design)
	}
	if !design.ApprovedAt.Equal(*at(24)) {
		t.Errorf("design approval = %v, want 19:24", design.ApprovedAt)
	}
	plan := stageByName(rows, constants.StageImplementationPlan)
	if plan.ApprovedAt == nil || !plan.ApprovedAt.Equal(*at(30)) {
		t.Errorf("impl_plan approval = %v, want 19:30", plan.ApprovedAt)
	}
	if impl := stageByName(rows, constants.StageImplementation); impl.Status != "done" || impl.Note != stageNotePrOpened {
		t.Errorf("implementation = %+v, want done/pr_opened", impl)
	}
}

func TestBuildStageProgress_InProgressShowsActiveAndPending(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseInProgress, StagePlan: fullStagePlan()}
	tasks := []*model.AgentTask{
		completedTask(constants.StagePickup, at(19), nil),
		{Stage: constants.StageBrainstorming, AttemptNo: 1, Status: constants.TaskStatusActive, StartedAt: at(20), CreatedAt: *at(20)},
	}

	rows := BuildStageProgress(run, tasks, nil)
	if got := stageByName(rows, constants.StageBrainstorming); got.Status != "active" {
		t.Errorf("brainstorming = %+v, want active", got)
	}
	if got := stageByName(rows, constants.StageDesign); got.Status != "pending" {
		t.Errorf("design = %+v, want pending", got)
	}
	if got := stageByName(rows, constants.StageImplementation); got.Status != "pending" {
		t.Errorf("implementation = %+v, want pending", got)
	}
}

func TestBuildStageProgress_AwaitingApprovalWhenNoApprovalYet(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseAwaitingApproval, StagePlan: fullStagePlan()}
	msgID := int64(1)
	tasks := []*model.AgentTask{
		completedTask(constants.StagePickup, at(19), nil),
		completedTask(constants.StageBrainstorming, at(20), nil),
		completedTask(constants.StageDesign, at(22), &msgID),
	}

	rows := BuildStageProgress(run, tasks, nil) // no approval events yet
	if got := stageByName(rows, constants.StageDesign); got.Status != "awaiting_approval" {
		t.Errorf("design = %+v, want awaiting_approval", got)
	}
}

func TestBuildStageProgress_SkippedAndFailed(t *testing.T) {
	plan := model.StagePlan{Stages: []model.StagePlanEntry{
		{Name: constants.StagePickup},
		{Name: constants.StageBrainstorming, Skippable: true, Skip: true},
		{Name: constants.StageDesign, Skippable: true},
	}}
	planJSON, _ := json.Marshal(plan)
	run := &model.AgentRun{Phase: constants.PhaseFailed, StagePlan: planJSON}
	tasks := []*model.AgentTask{
		completedTask(constants.StagePickup, at(19), nil),
		{Stage: constants.StageDesign, AttemptNo: 1, Status: constants.TaskStatusFailed, FinishedAt: at(22), CreatedAt: *at(21)},
	}

	rows := BuildStageProgress(run, tasks, nil)
	if got := stageByName(rows, constants.StageBrainstorming); got.Status != "skipped" {
		t.Errorf("brainstorming = %+v, want skipped", got)
	}
	if got := stageByName(rows, constants.StageDesign); got.Status != "failed" {
		t.Errorf("design = %+v, want failed", got)
	}
}

func TestBuildStageProgress_CarriesBotProvenance(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseInProgress, StagePlan: fullStagePlan()}
	botA := int64(7)
	botB := int64(9)
	pickup := completedTask(constants.StagePickup, at(19), nil)
	pickup.IdUserBot = &botA
	design := &model.AgentTask{Stage: constants.StageDesign, AttemptNo: 1, Status: constants.TaskStatusActive, StartedAt: at(25), CreatedAt: *at(25), IdUserBot: &botB}

	rows := BuildStageProgress(run, []*model.AgentTask{pickup, design}, nil)
	if got := stageByName(rows, constants.StagePickup); got.IdUserBot == nil || *got.IdUserBot != botA {
		t.Errorf("pickup bot = %v, want %d", got.IdUserBot, botA)
	}
	if got := stageByName(rows, constants.StageDesign); got.IdUserBot == nil || *got.IdUserBot != botB {
		t.Errorf("design bot = %v, want %d", got.IdUserBot, botB)
	}
}

func TestBuildStageProgress_LatestAttemptWins(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseInProgress, StagePlan: fullStagePlan()}
	tasks := []*model.AgentTask{
		{Stage: constants.StageDesign, AttemptNo: 1, Status: constants.TaskStatusFailed, FinishedAt: at(20), CreatedAt: *at(20)},
		{Stage: constants.StageDesign, AttemptNo: 2, Status: constants.TaskStatusActive, StartedAt: at(25), CreatedAt: *at(25)},
	}

	rows := BuildStageProgress(run, tasks, nil)
	got := stageByName(rows, constants.StageDesign)
	if got.Status != "active" || got.AttemptNo != 2 {
		t.Errorf("design = %+v, want active/attempt 2", got)
	}
}

// The snapshot carries what the feed needs to render a stage's thinking row:
// which message it hangs under, the tail, and whether full text still exists.
func TestBuildStageProgress_CarriesThinkingFields(t *testing.T) {
	run := &model.AgentRun{Phase: constants.PhaseInProgress, StagePlan: fullStagePlan()}
	idMessage := int64(77)
	tail := "the tail of the thinking"
	task := completedTask(constants.StageDesign, at(21), nil)
	task.IdResultMessage = &idMessage
	task.ThinkingTail = &tail
	task.HasThinking = true

	rows := BuildStageProgress(run, []*model.AgentTask{task}, nil)
	got := stageByName(rows, constants.StageDesign)

	if got.IdResultMessage == nil || *got.IdResultMessage != idMessage {
		t.Errorf("IdResultMessage = %v, want %d", got.IdResultMessage, idMessage)
	}
	if got.ThinkingTail == nil || *got.ThinkingTail != tail {
		t.Errorf("ThinkingTail = %v, want %q", got.ThinkingTail, tail)
	}
	if !got.HasThinking {
		t.Error("HasThinking = false, want true when the full text is stored")
	}
}
