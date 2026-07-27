package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type PhaseStateMirror struct {
	phaseStateMapRepo *repository.PhaseStateMapRepository
	issueRepo         *repository.IssueRepository
	stateRepo         *repository.StateRepository
}

func NewPhaseStateMirror(
	phaseStateMapRepo *repository.PhaseStateMapRepository,
	issueRepo *repository.IssueRepository,
	stateRepo *repository.StateRepository,
) *PhaseStateMirror {
	return &PhaseStateMirror{
		phaseStateMapRepo: phaseStateMapRepo,
		issueRepo:         issueRepo,
		stateRepo:         stateRepo,
	}
}

// ApplyMirror sets the issue state from the phase mapping, best-effort:
// errors are logged, not propagated — run phase stays canonical.
func (m *PhaseStateMirror) ApplyMirror(ctx context.Context, idProject, idIssue int64, toPhase string) {
	logger := extctx.GetLogger(ctx)
	mapping, err := m.phaseStateMapRepo.LoadMapping(ctx, idProject, toPhase)
	if err != nil {
		logger.Warn().
			Int64("idProject", idProject).
			Str("phase", toPhase).
			Err(err).
			Msg("phase state mirror: failed to load mapping")
		return
	}

	if mapping == nil {
		return
	}

	if mapping.IdState == nil {
		logger.Warn().
			Int64("idProject", idProject).
			Str("phase", toPhase).
			Msg("phase state mirror: mapped state is NULL (deleted?)")
		return
	}

	state, err := m.stateRepo.LoadState(ctx, idProject, *mapping.IdState)
	if err != nil || state == nil {
		logger.Warn().
			Int64("idProject", idProject).
			Int64("idState", *mapping.IdState).
			Err(err).
			Msg("phase state mirror: mapped state not found")
		return
	}

	if err := m.issueRepo.UpdateIssueState(ctx, idIssue, *mapping.IdState); err != nil {
		logger.Warn().
			Int64("idIssue", idIssue).
			Int64("idState", *mapping.IdState).
			Err(err).
			Msg("phase state mirror: failed to update issue state")
		return
	}

	logger.Info().
		Int64("idIssue", idIssue).
		Str("phase", toPhase).
		Int64("idState", *mapping.IdState).
		Str("stateName", state.Name).
		Msg("phase state mirror: issue state updated")
}
