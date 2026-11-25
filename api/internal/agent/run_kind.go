package agent

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// RunKind holds the behavior that differs between kinds of agent run. The shared mechanics
// (scheduler, dispatcher, heartbeat sweep, merge poller, gateway callbacks) call it and never test
// run.Kind. A new kind is a new implementation registered in NewRunKinds.
type RunKind interface {
	// NextTask returns the task to dispatch, creating it when needed, or nil when the run has
	// nothing left to do. Called under the run row lock.
	NextTask(ctx context.Context, run *model.AgentRun, tasks []*model.AgentTask) (*model.AgentTask, error)
	// Activate makes the task active and the run in_progress and tells the run's audience. Returns
	// nil when the run can no longer be dispatched.
	Activate(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*Dispatch, error)
	// Payload builds the stage_execute payload and returns the wiki pages its prompt carries.
	Payload(ctx context.Context, dispatch *Dispatch) (map[string]any, []*model.AgentRunWikiRead, error)
	// Started tells the run's audience that the gateway took the task.
	Started(ctx context.Context, dispatch *Dispatch)
	// Complete applies complete_stage after the controller checked the caller and the outcome.
	Complete(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) (model.CompleteStageRes, error)
	// Interrupt handles a task the agent will not finish. A task still active fails with reason and
	// detail; one the caller already failed keeps its error.
	Interrupt(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason InterruptReason, detail string) error
	// ApplyReport takes what the gateway reported after the agent process ended, beyond the stats
	// stored for every run.
	ApplyReport(ctx context.Context, run *model.AgentRun, task *model.AgentTask, report *TaskReport) error
	// ApplyPrStatus acts on the polled status of the PR of a run in pr_open. Returns the issue whose
	// MR badge shows the PR, or nil when no badge does.
	ApplyPrStatus(ctx context.Context, run *model.AgentRun, status *githost.Status) (*model.Issue, error)
	// Audience returns the users notified about the run.
	Audience(ctx context.Context, run *model.AgentRun) ([]int64, error)
	// SendFeedback hands a run waiting for a person the comments its agent has not seen: a new
	// attempt of the latest completed stage, and the run back to queued. Called under the run row
	// lock; a kind that takes no feedback refuses with errs.ErrRunNotAwaitingFeedback.
	SendFeedback(ctx context.Context, run *model.AgentRun, idUser int64) (*model.AgentRun, error)
}

// Dispatch is an activated task on its way to the gateway.
type Dispatch struct {
	Run  *model.AgentRun
	Task *model.AgentTask
	// RunToken is the raw token the agent authenticates with; only its hash is stored.
	RunToken string
}

// InterruptReason tells why the agent will not finish a task.
type InterruptReason string

const (
	InterruptHeartbeatStale InterruptReason = constants.FailReasonHeartbeatStale
	InterruptGatewayRestart InterruptReason = constants.FailReasonGatewayRestart
	InterruptDispatchFailed InterruptReason = constants.FailReasonDispatchFailed
)

// isTaskFailedByCaller tells whether the caller failed the task before the interrupt: the
// heartbeat sweep and a gateway restart fail tasks in bulk; an undeliverable task is still active.
func (r InterruptReason) isTaskFailedByCaller() bool {
	return r != InterruptDispatchFailed
}

// TaskReport is what the gateway reports with a task's stats.
type TaskReport struct {
	// HeadCommit is the commit a chat turn's worktree is at.
	HeadCommit string
}

// RunKinds finds the implementation of a run's kind.
type RunKinds struct {
	byName map[string]RunKind
}

func NewRunKinds(deps RunKindDeps) *RunKinds {
	common := newRunCommon(deps)
	return &RunKinds{byName: map[string]RunKind{
		constants.RunKindIssue:        &IssueRun{runCommon: common},
		constants.RunKindReleaseNotes: &ReleaseNotesRun{runCommon: common},
		constants.RunKindChat:         &ChatRun{runCommon: common},
	}}
}

// For returns the implementation of the run's kind. A kind without one is a programming error the
// registry test catches, so it panics.
func (k *RunKinds) For(run *model.AgentRun) RunKind {
	kind, isKnown := k.byName[run.Kind]
	if !isKnown {
		panic(fmt.Sprintf("agent run %d has kind %q without an implementation", run.IdRun, run.Kind))
	}
	return kind
}
