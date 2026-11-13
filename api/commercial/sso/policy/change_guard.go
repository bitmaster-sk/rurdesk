package policy

import (
	"context"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/breakglass"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

var ErrLastBreakGlass = errs.NewErrWithStatus("LAST_BREAK_GLASS", "SSO is enforced, the last break-glass account must stay", "error.last_break_glass", http.StatusConflict)

type ChangeGuard struct {
	config   config.IService
	accounts *breakglass.Repository
}

var _ authpolicy.UserChangeGuard = (*ChangeGuard)(nil)

func NewChangeGuard(service config.IService, accounts *breakglass.Repository) *ChangeGuard {
	return &ChangeGuard{config: service, accounts: accounts}
}

// Check reads the stored enforcement flag, not the license, so an expired license cannot strip the last way in.
func (g *ChangeGuard) Check(ctx context.Context, change authpolicy.UserChange) error {
	if err := g.accounts.Lock(ctx); err != nil {
		return err
	}
	settings, err := g.config.Settings(ctx)
	if err != nil {
		return err
	}
	if !settings.IsEnforced {
		return nil
	}
	isActive, err := g.accounts.IsActive(ctx, change.IdUser)
	if err != nil || !isActive {
		return err
	}
	count, err := g.accounts.CountActive(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastBreakGlass
	}
	return nil
}
