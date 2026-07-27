package agent

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// Stage note tokens the client maps to i18n strings — not display text.
// Empty means no note.
const (
	stageNoteNoClarifications = "no_clarifications"
	stageNoteSubmitted        = "submitted"
	stageNotePrOpened         = "pr_opened"
)

// BuildRunSnapshot assembles the payload the client renders — run, events,
// and derived stage timeline. Shared by GET endpoints and the websocket
// broadcast so they never diverge.
func BuildRunSnapshot(run *model.AgentRun, events []*model.AgentRunEvent, tasks []*model.AgentTask) *model.AgentRunWithEvents {
	return &model.AgentRunWithEvents{
		AgentRun: *run,
		Events:   events,
		Stages:   BuildStageProgress(run, tasks, events),
	}
}

// BuildStageProgress derives the timeline stepper rows from the run's stage
// plan, tasks (one per attempt), and events (approval waypoints). Robust
// under retries/revisions since stage identity comes from task rows, not
// counted phase transitions.
func BuildStageProgress(run *model.AgentRun, tasks []*model.AgentTask, events []*model.AgentRunEvent) []model.AgentStageProgress {
	stages := stagePlanEntries(run)

	// Latest attempt per stage wins (highest attempt_no, then newest).
	latestByStage := make(map[string]*model.AgentTask, len(stages))
	for _, t := range tasks {
		cur, ok := latestByStage[t.Stage]
		if !ok || t.AttemptNo > cur.AttemptNo ||
			(t.AttemptNo == cur.AttemptNo && t.CreatedAt.After(cur.CreatedAt)) {
			latestByStage[t.Stage] = t
		}
	}

	approvals := userApprovalsChronological(events)

	out := make([]model.AgentStageProgress, 0, len(stages))
	approvalIdx := 0
	for _, entry := range stages {
		prog := model.AgentStageProgress{Stage: entry.Name}

		if entry.Skip {
			prog.Status = "skipped"
			out = append(out, prog)
			continue
		}

		task := latestByStage[entry.Name]
		if task == nil {
			prog.Status = "pending"
			out = append(out, prog)
			continue
		}

		prog.AttemptNo = task.AttemptNo
		prog.IdUserBot = task.IdUserBot
		switch task.Status {
		case constants.TaskStatusActive, constants.TaskStatusPending:
			prog.Status = "active"
			prog.At = task.StartedAt
			if prog.At == nil {
				at := task.CreatedAt
				prog.At = &at
			}
		case constants.TaskStatusFailed:
			prog.Status = "failed"
			prog.At = task.FinishedAt
			prog.ErrorReason = task.ErrorReason
			prog.ErrorDetail = task.ErrorDetail
		case constants.TaskStatusCancelled:
			// A cancelled attempt isn't progress — show pending so a
			// subsequent Continue/Restart reads naturally.
			prog.Status = "pending"
		case constants.TaskStatusCompleted:
			prog.Status = "done"
			prog.At = task.FinishedAt
			prog.Note = completedNote(entry.Name, task)
			if isApprovable(entry.Name) {
				if approvalIdx < len(approvals) {
					at := approvals[approvalIdx]
					prog.ApprovedAt = &at
					approvalIdx++
				} else if run.Phase == constants.PhaseAwaitingApproval {
					prog.Status = "awaiting_approval"
				}
			}
		}
		out = append(out, prog)
	}
	return out
}

// stagePlanEntries reads the run's stage plan, falling back to the canonical
// definition order when the column is empty or unparseable.
func stagePlanEntries(run *model.AgentRun) []model.StagePlanEntry {
	var plan model.StagePlan
	if len(run.StagePlan) > 0 {
		_ = json.Unmarshal(run.StagePlan, &plan)
	}
	if len(plan.Stages) > 0 {
		return plan.Stages
	}
	entries := make([]model.StagePlanEntry, 0, len(constants.StageDefinitions))
	for _, d := range constants.StageDefinitions {
		entries = append(entries, model.StagePlanEntry{Name: d.Name, Skippable: d.Skippable})
	}
	return entries
}

func userApprovalsChronological(events []*model.AgentRunEvent) []time.Time {
	var approvals []time.Time
	for _, e := range events {
		if e.ActorType == constants.ActorTypeUser && e.Reason != nil && *e.Reason == "approved" {
			approvals = append(approvals, e.CreatedAt)
		}
	}
	sort.Slice(approvals, func(i, j int) bool { return approvals[i].Before(approvals[j]) })
	return approvals
}

func isApprovable(stage string) bool {
	return stage == constants.StageDesign || stage == constants.StageImplementationPlan
}

func completedNote(stage string, task *model.AgentTask) string {
	switch stage {
	case constants.StageBrainstorming:
		if task.IdOutputMessage == nil {
			return stageNoteNoClarifications
		}
		return ""
	case constants.StageDesign, constants.StageImplementationPlan:
		return stageNoteSubmitted
	case constants.StageImplementation:
		return stageNotePrOpened
	default:
		return ""
	}
}
