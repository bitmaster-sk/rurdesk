package agent

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// ReleaseNotesRun is the run of an agent writing a release's notes: one release_notes stage whose
// output becomes the notes. It works on no repository and opens no PR.
type ReleaseNotesRun struct {
	*runCommon
}

func (r *ReleaseNotesRun) NextTask(ctx context.Context, run *model.AgentRun, tasks []*model.AgentTask) (*model.AgentTask, error) {
	return r.nextPlannedTask(ctx, run, tasks)
}

func (r *ReleaseNotesRun) Activate(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*Dispatch, error) {
	return r.activatePlannedTask(ctx, run, task)
}

// Payload carries the release with its sections and open issues.
func (r *ReleaseNotesRun) Payload(ctx context.Context, dispatch *Dispatch) (map[string]any, []*model.AgentRunWikiRead, error) {
	run, task := dispatch.Run, dispatch.Task
	bundle, wikiReads, _, err := r.contextBundle(ctx, run, task, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("context bundle build error: %w", err)
	}
	release, err := r.releases.PromptContext(ctx, *run.IdRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("context bundle build error: loading release: %w", err)
	}
	bundle["release"] = release
	payload, err := r.stagePayload(ctx, run, task, bundle, nil, "")
	return payload, wikiReads, err
}

func (r *ReleaseNotesRun) Started(ctx context.Context, dispatch *Dispatch) {
	r.broadcastRun(ctx, dispatch.Run)
}

// Complete stores submitted output as the release's notes and moves the run on.
func (r *ReleaseNotesRun) Complete(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) (model.CompleteStageRes, error) {
	isReconcile, isNoOp := r.reconcileGate(ctx, run, task)
	if isNoOp {
		return unchangedCompletion(run, task), nil
	}
	var res model.CompleteStageRes
	txErr := extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		if body.Outcome == constants.StageOutcomeOutputSubmitted {
			if err := r.releases.NotesWritten(ctx, *run.IdRelease, body.Message); err != nil {
				return fmt.Errorf("writing release notes: %w", err)
			}
		}
		var err error
		res, err = r.finishTask(ctx, run, task, body, nil, false, isReconcile)
		return err
	})
	if txErr != nil {
		return completionError(txErr, run, task)
	}
	r.thinking.Compact(ctx, task.IdTask)
	if updated, err := r.runRepo.LoadById(ctx, run.IdRun); err == nil {
		r.broadcastRun(context.Background(), updated)
	}
	return res, nil
}

func (r *ReleaseNotesRun) Interrupt(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason InterruptReason, detail string) error {
	return r.failPlannedRun(ctx, run, task, reason, detail)
}

// ApplyReport has nothing to take: a release notes run reports only stats.
func (r *ReleaseNotesRun) ApplyReport(context.Context, *model.AgentRun, *model.AgentTask, *TaskReport) error {
	return nil
}

// ApplyPrStatus fails: a release notes run opens no PR.
func (r *ReleaseNotesRun) ApplyPrStatus(_ context.Context, run *model.AgentRun, _ *githost.Status) (*model.Issue, error) {
	return nil, fmt.Errorf("release notes run %d: %w", run.IdRun, errRunHasNoPr)
}

func (r *ReleaseNotesRun) Audience(ctx context.Context, run *model.AgentRun) ([]int64, error) {
	return r.projectMembers(ctx, run)
}
