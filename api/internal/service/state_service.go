package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StateService struct {
	pool      *pgxpool.Pool
	stateRepo *repository.StateRepository
}

func NewStateService(pool *pgxpool.Pool, stateRepo *repository.StateRepository) *StateService {
	return &StateService{pool: pool, stateRepo: stateRepo}
}

// DeleteWithMigration deletes a state from a project; anything still
// referencing it (issues, project default, agent phases) requires an explicit
// intent — migrateTo repoints, unassign NULLs.
func (s *StateService) DeleteWithMigration(ctx context.Context, idProject, idState int64, migrateTo *int64, unassign bool) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		// pgx.ErrNoRows when the state is not mapped to this project (→ 404)
		state, err := s.stateRepo.LoadState(ctx, idProject, idState)
		if err != nil {
			return err
		}
		usage, err := s.stateRepo.LoadStateUsage(ctx, idProject, idState)
		if err != nil {
			return err
		}
		// gating on issues alone would silently null a default/phase reference
		isInUse := usage.Issues > 0 || usage.IsProjectDefault || usage.AgentPhases > 0
		if isInUse && migrateTo == nil && !unassign {
			return errs.ErrStateInUse
		}
		var target *int64
		if migrateTo != nil {
			if *migrateTo == idState {
				return errs.ErrInvalidStateMigrationTarget
			}
			// LoadState's project join rejects cross-project targets
			if _, err := s.stateRepo.LoadState(ctx, idProject, *migrateTo); err != nil {
				return fmt.Errorf("%w: %v", errs.ErrInvalidStateMigrationTarget, err)
			}
			target = migrateTo
		}

		if err := s.stateRepo.ReassignIssuesState(ctx, idProject, idState, target); err != nil {
			return err
		}
		if err := s.stateRepo.RepointProjectDefaultState(ctx, idProject, idState, target); err != nil {
			return err
		}
		if err := s.stateRepo.RepointAgentPhaseState(ctx, idProject, idState, target); err != nil {
			return err
		}
		if err := s.stateRepo.DeleteProjectState(ctx, state); err != nil {
			return err
		}
		// protected rows are shared; Mappings > 1 means another project's issues
		// would be nulled by the FK
		if !state.Protected && usage.Mappings <= 1 {
			return s.stateRepo.DeleteState(ctx, idState)
		}
		return nil
	})
}
