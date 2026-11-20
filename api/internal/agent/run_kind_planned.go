package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// The steps issue and release notes runs share. Both walk a stage plan, fail when a task is lost
// and tell the project's members about every change.

var (
	errRunHasNoPr = errors.New("this kind of run opens no PR")
	// errReconcileSuperseded rolls back a completion that a concurrent Restart/Continue overtook.
	errReconcileSuperseded = errors.New("reconcile superseded by concurrent run transition")
)

// nextPlannedTask returns a pending task, or creates the next stage of the plan; nil when the plan
// is done. A pending task can already exist (a revision or other re-queue created it directly):
// dispatch it, as ResolveNextStage may miss it if its stage already shows completed.
func (c *runCommon) nextPlannedTask(ctx context.Context, run *model.AgentRun, tasks []*model.AgentTask) (*model.AgentTask, error) {
	if pending := latestPendingTask(tasks); pending != nil {
		return pending, nil
	}
	stage, err := ResolveNextStage(run, tasks)
	if err != nil || stage == "" {
		return nil, err
	}
	return c.taskRepo.Insert(ctx, run.IdRun, run.IdUserAgent, stage, ResolveNextAttemptNo(tasks, stage))
}

// activatePlannedTask marks the task active, moves the run queued→in_progress and broadcasts the
// snapshot. It broadcasts even when the run was already in_progress (e.g. dispatching the next stage
// right after a user approval), otherwise the timeline shows the prior stage as pending until it
// completes.
func (c *runCommon) activatePlannedTask(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*Dispatch, error) {
	if task.Status != constants.TaskStatusActive {
		if _, err := c.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive); err != nil {
			return nil, err
		}
	}
	activated := run
	if run.Phase == constants.PhaseQueued {
		updated, err := c.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseInProgress, constants.ActorTypeSystem, nil, "dispatch stage "+task.Stage)
		if err != nil {
			log.Debug().Err(err).Int64("idRun", run.IdRun).Msg("queued→in_progress transition skipped")
		} else {
			activated = updated
		}
	}
	c.broadcastRun(ctx, activated)
	// The run is failed from this phase if the dispatch fails, so it must be the current one.
	return &Dispatch{Run: activated, Task: task}, nil
}

func (c *runCommon) broadcastRun(ctx context.Context, run *model.AgentRun) {
	BroadcastRunUpdate(ctx, c.notifier, c.projectRepo, c.issueRepo, c.runRepo, c.taskRepo, run)
}

// failPlannedRun fails the task and the run, so the card surfaces the error and Continue/Restart.
// A run already terminal stays as it is. Without a detail the reason becomes the run's error
// message; a detailed failure keeps its detail on the task.
func (c *runCommon) failPlannedRun(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason InterruptReason, detail string) error {
	var taskErr error
	if !reason.isTaskFailedByCaller() {
		taskErr = c.failActiveTask(ctx, task, string(reason), detail)
	}
	if constants.TerminalPhases[run.Phase] {
		return taskErr
	}
	note := detail
	if note == "" {
		note = string(reason)
	}
	updated, err := c.runRepo.TransitionPhase(ctx, run.IdRun, run.Phase, constants.PhaseFailed, constants.ActorTypeSystem, nil, note)
	if err != nil {
		log.Debug().Err(err).Int64("idRun", run.IdRun).Msg("failing the run skipped")
		return taskErr
	}
	if detail == "" {
		message := string(reason)
		if err := c.runRepo.SetErrorMessage(ctx, run.IdRun, message); err != nil {
			taskErr = errors.Join(taskErr, err)
		}
		updated.ErrorMessage = &message
	}
	c.broadcastRun(ctx, updated)
	return taskErr
}

// failActiveTask fails a task that is still active; one that already ended is left as it is.
func (c *runCommon) failActiveTask(ctx context.Context, task *model.AgentTask, reason, detail string) error {
	if _, err := c.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusFailed); err != nil {
		if errors.Is(err, repository.ErrTaskStatusMismatch) {
			return nil
		}
		return err
	}
	return c.taskRepo.SetError(ctx, task.IdTask, reason, detail)
}

// projectMembers is the audience of a run every member of its project follows.
func (c *runCommon) projectMembers(ctx context.Context, run *model.AgentRun) ([]int64, error) {
	members, err := c.projectRepo.LoadProjectsMembers(ctx, []int64{run.IdProject})
	if err != nil {
		return nil, err
	}
	idsUser := make([]int64, len(members))
	for i, member := range members {
		idsUser[i] = member.IdUser
	}
	return idsUser, nil
}

// reconcileGate decides, before any host I/O, how complete_stage treats the task's status, so no
// orphan PR is created for a completion that will not be applied. A still-live gateway may report
// completion for a task an API/gateway restart already blanket-failed: that late completion is
// reconciled, but only while the run is still `failed`. isNoOp means respond with the task as it is.
func (c *runCommon) reconcileGate(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (isReconcile, isNoOp bool) {
	switch {
	case task.Status == constants.TaskStatusActive:
		return false, false
	case task.Status == constants.TaskStatusFailed && run.Phase == constants.PhaseFailed &&
		task.ErrorReason != nil && constants.RecoverableFailReasons[*task.ErrorReason]:
		// If Continue already moved the run out, or the user Restarted (a newer non-terminal run
		// exists), reconciling would resurrect/duplicate. The in-tx CAS is the final authority for
		// the race after this check.
		if active, err := c.runRepo.LoadActiveOfSubject(ctx, run); err == nil && active != nil && active.IdRun != run.IdRun {
			return false, true
		}
		return true, false
	default:
		// Completed: an idempotent retry of a successful complete_stage. Otherwise a genuine
		// failure or cancellation: no-op, no PR, don't resurrect an abandoned run.
		return false, true
	}
}

// unchangedCompletion answers a complete_stage that changed nothing with the task as it is.
func unchangedCompletion(run *model.AgentRun, task *model.AgentTask) model.CompleteStageRes {
	return model.CompleteStageRes{IdTask: task.IdTask, Status: task.Status, NextPhase: run.Phase}
}

// completionError maps a failed completion transaction to the complete_stage answer: a superseded
// reconcile is a no-op, a bad attachment link the caller's fault, anything else internal.
func completionError(txErr error, run *model.AgentRun, task *model.AgentTask) (model.CompleteStageRes, error) {
	switch {
	case errors.Is(txErr, errReconcileSuperseded):
		return unchangedCompletion(run, task), nil
	case errors.Is(txErr, errs.ErrAttachmentLinkInvalid):
		return model.CompleteStageRes{}, txErr
	default:
		return model.CompleteStageRes{}, errs.ErrInternal.WithMessage(txErr.Error())
	}
}

// finishTask records the task's result and moves the run to the phase the outcome leads to. Runs
// inside the completion transaction, after the kind wrote its output.
func (c *runCommon) finishTask(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq, idMessage *int64, isPrWritten, isReconcile bool) (model.CompleteStageRes, error) {
	targetStatus := constants.TaskStatusCompleted
	if body.Outcome == constants.StageOutcomeErrored {
		targetStatus = constants.TaskStatusFailed
	}
	if err := c.taskRepo.SetResultAndStats(ctx, task.IdTask, idMessage, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
		return model.CompleteStageRes{}, err
	}
	// CompleteReconcilable accepts the task from `active` (normal completion) or from `failed` with
	// a recoverable error_reason (a late completion reconciled after a restart orphaned it). MUST
	// run BEFORE SetError: SetError overwrites error_reason, which could push the row out of the
	// recoverable allowlist and mismatch this CAS.
	if _, err := c.taskRepo.CompleteReconcilable(ctx, task.IdTask, targetStatus, constants.RecoverableFailReasonList); err != nil {
		// A concurrent complete_stage (dual caller: gateway + MCP tool, or a retry) already
		// completed or reconciled this task after the gate. The winner records the work.
		if errors.Is(err, repository.ErrTaskStatusMismatch) {
			return model.CompleteStageRes{}, errReconcileSuperseded
		}
		return model.CompleteStageRes{}, fmt.Errorf("transitioning task status: %w", err)
	}
	if body.Outcome == constants.StageOutcomeErrored {
		reason := "agent_reported_error"
		if body.ErrorReason != nil {
			reason = *body.ErrorReason
		}
		detail := ""
		if body.ErrorDetail != nil {
			detail = *body.ErrorDetail
		}
		if err := c.taskRepo.SetError(ctx, task.IdTask, reason, detail); err != nil {
			return model.CompleteStageRes{}, err
		}
	}

	nextPhase := decideNextRunPhase(task.Stage, body.Outcome)
	// Written PR info already moved the run to pr_open; a second transition would fail the phase guard.
	if !isPrWritten {
		if isReconcile && !constants.TerminalPhases[nextPhase] {
			// Move a crash-orphaned run out of `failed` back into the pipeline, clearing the stale
			// failure stamp. A concurrent Restart/Continue mismatches the CAS: superseded no-op.
			if _, err := c.runRepo.ReconcileToPhase(ctx, run.IdRun, run.Phase, nextPhase, constants.ActorTypeAgent, "complete_stage:reconcile:"+body.Outcome); err != nil {
				if errors.Is(err, repository.ErrPhaseMismatch) {
					return model.CompleteStageRes{}, errReconcileSuperseded
				}
				return model.CompleteStageRes{}, fmt.Errorf("reconcile phase transition: %w", err)
			}
		} else if _, err := c.runRepo.TransitionPhase(ctx, run.IdRun, run.Phase, nextPhase, constants.ActorTypeAgent, nil, "complete_stage:"+body.Outcome); err != nil {
			log.Debug().Err(err).Int64("idRun", run.IdRun).Str("toPhase", nextPhase).Msg("phase transition tolerated")
		}
	}
	if nextPhase == constants.PhaseDone {
		idRun := run.IdRun
		extctx.AfterCommit(ctx, func(detached context.Context) {
			if finished, err := c.runRepo.LoadById(detached, idRun); err == nil {
				c.proposals.RunFinished(detached, finished)
			}
		})
	}
	return model.CompleteStageRes{IdTask: task.IdTask, Status: targetStatus, NextPhase: nextPhase}, nil
}

func decideNextRunPhase(stage, outcome string) string {
	if outcome == constants.StageOutcomeErrored {
		return constants.PhaseFailed
	}
	if outcome == constants.StageOutcomeNoActionNeeded {
		return constants.PhaseInProgress
	}
	switch stage {
	case constants.StagePickup:
		return constants.PhaseInProgress
	case constants.StageBrainstorming:
		if outcome == constants.StageOutcomeQuestionAsked {
			return constants.PhaseAwaitingInput
		}
		return constants.PhaseInProgress
	case constants.StageDesign, constants.StageImplementationPlan:
		return constants.PhaseAwaitingApproval
	case constants.StageImplementation:
		return constants.PhasePrOpen
	case constants.StageReleaseNotes:
		return constants.PhaseDone
	default:
		return constants.PhaseInProgress
	}
}
