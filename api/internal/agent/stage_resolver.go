package agent

import (
	"encoding/json"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// ResolveNextStage returns the next stage to execute for a run: "" when every
// non-skipped stage is completed, or the most recent task's stage if it
// failed (Continue redoes it).
func ResolveNextStage(run *model.AgentRun, tasks []*model.AgentTask) (string, error) {
	var plan model.AgentStagePlan
	if err := json.Unmarshal(run.StagePlan, &plan); err != nil {
		return "", fmt.Errorf("unmarshalling stage_plan: %w", err)
	}

	completedByStage := map[string]bool{}
	for _, t := range tasks {
		if t.Status == constants.TaskStatusCompleted {
			completedByStage[t.Stage] = true
		}
	}

	if latest := latestTask(tasks); latest != nil && latest.Status == constants.TaskStatusFailed {
		return latest.Stage, nil
	}

	for _, entry := range plan.Stages {
		if entry.Skip {
			continue
		}
		if !completedByStage[entry.Name] {
			return entry.Name, nil
		}
	}
	return "", nil
}

// ResolveNextAttemptNo returns the attempt number for the next task at the
// given stage (1 if no prior attempt, else max(attempt_no)+1).
func ResolveNextAttemptNo(tasks []*model.AgentTask, stage string) int {
	maxN := 0
	for _, t := range tasks {
		if t.Stage == stage && t.AttemptNo > maxN {
			maxN = t.AttemptNo
		}
	}
	return maxN + 1
}

// ResolveAttemptContext returns why this attempt of the stage exists:
// resumedAfter is how the attempt right before it stopped (its failure reason,
// "unknown" or "cancelled"), isRevision is set when an earlier attempt completed.
func ResolveAttemptContext(tasks []*model.AgentTask, task *model.AgentTask) (resumedAfter string, isRevision bool) {
	var previous *model.AgentTask
	for _, t := range tasks {
		if t.Stage != task.Stage || t.AttemptNo >= task.AttemptNo {
			continue
		}
		if t.Status == constants.TaskStatusCompleted {
			isRevision = true
		}
		if previous == nil || t.AttemptNo > previous.AttemptNo {
			previous = t
		}
	}
	if previous == nil {
		return "", isRevision
	}
	switch previous.Status {
	case constants.TaskStatusFailed:
		if previous.ErrorReason == nil || *previous.ErrorReason == "" {
			return "unknown", isRevision
		}
		return *previous.ErrorReason, isRevision
	case constants.TaskStatusCancelled:
		return "cancelled", isRevision
	default:
		return "", isRevision
	}
}

func latestTask(tasks []*model.AgentTask) *model.AgentTask {
	var latest *model.AgentTask
	for _, t := range tasks {
		if latest == nil || t.CreatedAt.After(latest.CreatedAt) {
			latest = t
		}
	}
	return latest
}
