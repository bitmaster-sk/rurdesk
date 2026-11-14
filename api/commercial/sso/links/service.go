package links

import (
	"context"
	"errors"

	"github.com/bitmaster-sk/rurdesk/api/internal/deactivation"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IService interface {
	List(ctx context.Context, idUser int64) ([]IdentityRes, error)
	Unlink(ctx context.Context, idUser int64) error
	Move(ctx context.Context, idUser, idTarget, idActor int64) error
}

type Service struct {
	pool         *pgxpool.Pool
	repo         *linkRepository
	users        *repository.UserRepository
	deactivation *deactivation.Service
}

var _ IService = (*Service)(nil)

func NewService(pool *pgxpool.Pool, users *repository.UserRepository, deactivationService *deactivation.Service) *Service {
	return &Service{pool: pool, repo: &linkRepository{pool: pool}, users: users, deactivation: deactivationService}
}

func (s *Service) List(ctx context.Context, idUser int64) ([]IdentityRes, error) {
	return s.repo.list(ctx, idUser)
}

func (s *Service) Unlink(ctx context.Context, idUser int64) error {
	deleted, err := s.repo.deleteAll(ctx, idUser)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrIdentityNotFound
	}
	return nil
}

func (s *Service) Move(ctx context.Context, idUser, idTarget, idActor int64) error {
	if idUser == idActor {
		return errs.ErrSelfDisable
	}
	var result *deactivation.Result
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.checkTarget(ctx, idUser, idTarget); err != nil {
			return err
		}
		issuers, err := s.repo.issuersForUpdate(ctx, idUser)
		if err != nil {
			return err
		}
		if len(issuers) == 0 {
			return ErrIdentityNotFound
		}
		hasConflict, err := s.repo.hasAnyIssuer(ctx, idTarget, issuers)
		if err != nil {
			return err
		}
		if hasConflict {
			return ErrIdentityExists
		}
		if err := s.repo.moveAll(ctx, idUser, idTarget); err != nil {
			return err
		}
		result, err = s.deactivation.ApplyInTx(ctx, idUser, false, deactivation.Options{IdActor: idActor, SkipLastOwnerGuard: true})
		return err
	})
	if err != nil {
		return err
	}
	return s.deactivation.AfterCommit(ctx, result)
}

func (s *Service) checkTarget(ctx context.Context, idUser, idTarget int64) error {
	if idTarget == idUser {
		return ErrTargetInvalid
	}
	target, err := s.users.LoadUser(ctx, idTarget)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTargetInvalid
	}
	if err != nil {
		return err
	}
	if target.IsAgent || target.IsBreakGlass || !target.IsActive {
		return ErrTargetInvalid
	}
	return nil
}
