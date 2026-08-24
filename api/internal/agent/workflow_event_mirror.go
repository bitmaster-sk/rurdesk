package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type WorkflowEventMirror struct {
	workflowEventMapRepo *repository.WorkflowEventMapRepository
	issueRepo            *repository.IssueRepository
	stateRepo            *repository.StateRepository
}

func NewWorkflowEventMirror(
	workflowEventMapRepo *repository.WorkflowEventMapRepository,
	issueRepo *repository.IssueRepository,
	stateRepo *repository.StateRepository,
) *WorkflowEventMirror {
	return &WorkflowEventMirror{
		workflowEventMapRepo: workflowEventMapRepo,
		issueRepo:            issueRepo,
		stateRepo:            stateRepo,
	}
}

// ApplyMirror sets the issue state from the event mapping, best-effort:
// errors are logged, not propagated — run phase stays canonical.
func (m *WorkflowEventMirror) ApplyMirror(ctx context.Context, idProject, idIssue int64, event string) {
	logger := extctx.GetLogger(ctx)
	mapping, err := m.workflowEventMapRepo.LoadMapping(ctx, idProject, event)
	if err != nil {
		logger.Warn().
			Int64("idProject", idProject).
			Str("event", event).
			Err(err).
			Msg("workflow event mirror: failed to load mapping")
		return
	}

	if mapping == nil {
		return
	}

	if mapping.IdState == nil {
		logger.Warn().
			Int64("idProject", idProject).
			Str("event", event).
			Msg("workflow event mirror: mapped state is NULL (deleted?)")
		return
	}

	state, err := m.stateRepo.LoadState(ctx, idProject, *mapping.IdState)
	if err != nil || state == nil {
		logger.Warn().
			Int64("idProject", idProject).
			Int64("idState", *mapping.IdState).
			Err(err).
			Msg("workflow event mirror: mapped state not found")
		return
	}

	if err := m.issueRepo.UpdateIssueState(ctx, idIssue, *mapping.IdState); err != nil {
		logger.Warn().
			Int64("idIssue", idIssue).
			Int64("idState", *mapping.IdState).
			Err(err).
			Msg("workflow event mirror: failed to update issue state")
		return
	}

	logger.Info().
		Int64("idIssue", idIssue).
		Str("event", event).
		Int64("idState", *mapping.IdState).
		Str("stateName", state.Name).
		Msg("workflow event mirror: issue state updated")
}
