package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
)

type eventMappingLoader interface {
	LoadMapping(ctx context.Context, idProject int64, event string) (*model.WorkflowEventMapping, error)
}

type stateLoader interface {
	LoadState(ctx context.Context, idProject, idState int64) (*model.State, error)
}

type issueStateStore interface {
	issueLoader
	UpdateIssueState(ctx context.Context, idIssue int64, idState int64) error
}

type PhaseStateTransitioner struct {
	workflowEventMapRepo eventMappingLoader
	issueRepo            issueStateStore
	stateRepo            stateLoader
	projectRepo          projectMemberLoader
	notifier             *notify.Notifier
}

func NewPhaseStateTransitioner(
	workflowEventMapRepo eventMappingLoader,
	issueRepo issueStateStore,
	stateRepo stateLoader,
	projectRepo projectMemberLoader,
	notifier *notify.Notifier,
) *PhaseStateTransitioner {
	return &PhaseStateTransitioner{
		workflowEventMapRepo: workflowEventMapRepo,
		issueRepo:            issueRepo,
		stateRepo:            stateRepo,
		projectRepo:          projectRepo,
		notifier:             notifier,
	}
}

// Transition moves the issue to the state the project maps the event to, best-effort:
// errors are logged, not propagated — run phase stays canonical.
func (t *PhaseStateTransitioner) Transition(ctx context.Context, idProject, idIssue int64, event string) {
	logger := extctx.GetLogger(ctx)
	mapping, err := t.workflowEventMapRepo.LoadMapping(ctx, idProject, event)
	if err != nil {
		logger.Warn().
			Int64("idProject", idProject).
			Str("event", event).
			Err(err).
			Msg("phase state transition: failed to load mapping")
		return
	}

	if mapping == nil {
		return
	}

	if mapping.IdState == nil {
		logger.Warn().
			Int64("idProject", idProject).
			Str("event", event).
			Msg("phase state transition: mapped state is NULL (deleted?)")
		return
	}

	state, err := t.stateRepo.LoadState(ctx, idProject, *mapping.IdState)
	if err != nil || state == nil {
		logger.Warn().
			Int64("idProject", idProject).
			Int64("idState", *mapping.IdState).
			Err(err).
			Msg("phase state transition: mapped state not found")
		return
	}

	if err := t.issueRepo.UpdateIssueState(ctx, idIssue, *mapping.IdState); err != nil {
		logger.Warn().
			Int64("idIssue", idIssue).
			Int64("idState", *mapping.IdState).
			Err(err).
			Msg("phase state transition: failed to update issue state")
		return
	}

	t.broadcast(ctx, idIssue)

	logger.Info().
		Int64("idIssue", idIssue).
		Str("event", event).
		Int64("idState", *mapping.IdState).
		Str("stateName", state.Name).
		Msg("phase state transition: issue state updated")
}

// broadcast must stay deferred: BroadcastIssueUpdate re-reads the issue on a pool connection after the request context is gone.
func (t *PhaseStateTransitioner) broadcast(ctx context.Context, idIssue int64) {
	if t.notifier == nil || t.projectRepo == nil {
		return
	}
	extctx.AfterCommit(ctx, func(detached context.Context) {
		BroadcastIssueUpdate(detached, t.notifier, t.issueRepo, t.projectRepo, idIssue)
	})
}
