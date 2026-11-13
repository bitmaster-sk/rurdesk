package enforce

import (
	"context"
	"errors"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/breakglass"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBreakGlassRequired = errs.NewErr("SSO_BREAK_GLASS_REQUIRED", "enforcing SSO needs at least one active break-glass account", "error.sso_break_glass_required")
	ErrNotEnabled         = errs.NewErr("SSO_NOT_ENABLED", "SSO must be active before it can be enforced", "error.sso_not_enabled")
)

type ISessions interface {
	InvalidateUserSessions(ctx context.Context, idUser int64) error
	InvalidateUserSessionsExcept(ctx context.Context, idUser int64, keepToken string) error
}

type Service struct {
	pool     *pgxpool.Pool
	config   config.IService
	accounts *breakglass.Repository
	sessions ISessions
}

func NewService(pool *pgxpool.Pool, service config.IService, accounts *breakglass.Repository, sessions ISessions) *Service {
	return &Service{pool: pool, config: service, accounts: accounts, sessions: sessions}
}

func (s *Service) Set(ctx context.Context, isEnforced bool, idActor int64, keepToken string) error {
	if !isEnforced {
		return s.config.SetEnforced(ctx, false)
	}
	wasEnforced := false
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.accounts.Lock(ctx); err != nil {
			return err
		}
		settings, err := s.config.Settings(ctx)
		if err != nil {
			return err
		}
		if !settings.IsEnabled {
			return ErrNotEnabled
		}
		if settings.IsEnforced {
			wasEnforced = true
			return nil
		}
		count, err := s.accounts.CountActive(ctx)
		if err != nil {
			return err
		}
		if count == 0 {
			return ErrBreakGlassRequired
		}
		return s.config.SetEnforced(ctx, true)
	})
	if err != nil || wasEnforced {
		return err
	}
	return s.signOutOthers(ctx, idActor, keepToken)
}

func (s *Service) signOutOthers(ctx context.Context, idActor int64, keepToken string) error {
	ids, err := s.accounts.LoadIdsToSignOut(ctx)
	if err != nil {
		return err
	}
	var failures []error
	for _, idUser := range ids {
		if idUser == idActor {
			failures = append(failures, s.sessions.InvalidateUserSessionsExcept(ctx, idUser, keepToken))
			continue
		}
		failures = append(failures, s.sessions.InvalidateUserSessions(ctx, idUser))
	}
	return errors.Join(failures...)
}
