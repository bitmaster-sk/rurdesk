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
	var plan model.StagePlan
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

func latestTask(tasks []*model.AgentTask) *model.AgentTask {
	var latest *model.AgentTask
	for _, t := range tasks {
		if latest == nil || t.CreatedAt.After(latest.CreatedAt) {
			latest = t
		}
	}
	return latest
}
