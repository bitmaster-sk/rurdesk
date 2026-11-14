package policy

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type PasswordLogin struct {
	config  config.IService
	license func() license.Provider
}

var _ authpolicy.PasswordLoginPolicy = (*PasswordLogin)(nil)

func NewPasswordLogin(service config.IService, licenseProvider func() license.Provider) *PasswordLogin {
	return &PasswordLogin{config: service, license: licenseProvider}
}

func (p *PasswordLogin) Allow(ctx context.Context, user *model.User) bool {
	if IsBreakGlassAccount(user) {
		return true
	}
	settings, err := p.config.Settings(ctx)
	if err != nil {
		extctx.GetLogger(ctx).Warn().Err(err).Msg("sso: loading settings for a password sign-in")
		return false
	}
	return !IsEnforced(settings, p.license().IsAvailable(ctx, license.SSO))
}

// IsBreakGlassAccount trusts the flag because the core clears it whenever the account stops qualifying.
func IsBreakGlassAccount(user *model.User) bool {
	return user.IsBreakGlass && user.IsActive && user.IsAdmin && !user.IsAgent && !user.IsPasswordless
}
