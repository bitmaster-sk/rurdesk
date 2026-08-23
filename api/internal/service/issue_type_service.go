package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueTypeService struct {
	pool          *pgxpool.Pool
	issueTypeRepo *repository.IssueTypeRepository
}

func NewIssueTypeService(pool *pgxpool.Pool, issueTypeRepo *repository.IssueTypeRepository) *IssueTypeService {
	return &IssueTypeService{pool: pool, issueTypeRepo: issueTypeRepo}
}

func (s *IssueTypeService) DeleteWithMigration(ctx context.Context, idProject, idIssueType int64, migrateTo *int64, unassign bool) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		issueType, err := s.issueTypeRepo.LoadIssueType(ctx, idProject, idIssueType)
		if err != nil {
			return err
		}
		if issueType.Protected {
			return errs.ErrIssueTypeProtected
		}
		usage, err := s.issueTypeRepo.LoadIssueTypeUsage(ctx, idProject, idIssueType)
		if err != nil {
			return err
		}
		isInUse := usage.Issues > 0 || usage.IsProjectDefault
		if isInUse && migrateTo == nil && !unassign {
			return errs.ErrIssueTypeInUse
		}
		var target *int64
		if migrateTo != nil {
			if *migrateTo == idIssueType {
				return errs.ErrInvalidIssueTypeMigrationTarget
			}
			if _, err := s.issueTypeRepo.LoadIssueType(ctx, idProject, *migrateTo); err != nil {
				return fmt.Errorf("%w: %v", errs.ErrInvalidIssueTypeMigrationTarget, err)
			}
			target = migrateTo
		}
		if err := s.issueTypeRepo.ReassignIssuesIssueType(ctx, idProject, idIssueType, target); err != nil {
			return err
		}
		if err := s.issueTypeRepo.RepointProjectDefaultIssueType(ctx, idProject, idIssueType, target); err != nil {
			return err
		}
		return s.issueTypeRepo.DeleteIssueType(ctx, idProject, idIssueType)
	})
}
