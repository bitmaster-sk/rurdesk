package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SeverityService struct {
	pool         *pgxpool.Pool
	severityRepo *repository.SeverityRepository
}

func NewSeverityService(pool *pgxpool.Pool, severityRepo *repository.SeverityRepository) *SeverityService {
	return &SeverityService{pool: pool, severityRepo: severityRepo}
}

// DeleteWithMigration mirrors StateService.DeleteWithMigration (no agent phase map).
func (s *SeverityService) DeleteWithMigration(ctx context.Context, idProject, idSeverity int64, migrateTo *int64, unassign bool) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		severity, err := s.severityRepo.LoadSeverity(ctx, idProject, idSeverity)
		if err != nil {
			return err
		}
		usage, err := s.severityRepo.LoadSeverityUsage(ctx, idProject, idSeverity)
		if err != nil {
			return err
		}
		isInUse := usage.Issues > 0 || usage.IsProjectDefault
		if isInUse && migrateTo == nil && !unassign {
			return errs.ErrSeverityInUse
		}
		var target *int64
		if migrateTo != nil {
			if *migrateTo == idSeverity {
				return errs.ErrInvalidSeverityMigrationTarget
			}
			if _, err := s.severityRepo.LoadSeverity(ctx, idProject, *migrateTo); err != nil {
				return fmt.Errorf("%w: %v", errs.ErrInvalidSeverityMigrationTarget, err)
			}
			target = migrateTo
		}
		if err := s.severityRepo.ReassignIssuesSeverity(ctx, idProject, idSeverity, target); err != nil {
			return err
		}
		if err := s.severityRepo.RepointProjectDefaultSeverity(ctx, idProject, idSeverity, target); err != nil {
			return err
		}
		if err := s.severityRepo.DeleteProjectSeverity(ctx, severity); err != nil {
			return err
		}
		if !severity.Protected && usage.Mappings <= 1 {
			return s.severityRepo.DeleteSeverity(ctx, idSeverity)
		}
		return nil
	})
}
