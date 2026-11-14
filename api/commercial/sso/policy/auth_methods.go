package policy

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type Provisioning interface {
	IsProvisioning(ctx context.Context) (bool, error)
}

type AuthMethods struct {
	config       config.IService
	provisioning Provisioning
	license      func() license.Provider
}

var _ authpolicy.AuthMethodsProvider = (*AuthMethods)(nil)

func NewAuthMethods(service config.IService, provisioning Provisioning, licenseProvider func() license.Provider) *AuthMethods {
	return &AuthMethods{config: service, provisioning: provisioning, license: licenseProvider}
}

func (p *AuthMethods) Methods(ctx context.Context) model.AuthMethodsRes {
	methods := model.AuthMethodsRes{Password: true}
	settings, err := p.config.Settings(ctx)
	if err != nil {
		extctx.GetLogger(ctx).Warn().Err(err).Msg("sso: loading settings for the login page")
		return methods
	}
	if !settings.IsEnabled {
		return methods
	}
	isLicensed := p.license().IsAvailable(ctx, license.SSO)
	if _, err := config.ReadyBaseUrl(); err != nil || !isLicensed {
		methods.SsoUnavailable = true
		return methods
	}
	protocol := settings.ActiveProtocol
	if protocol == "" {
		protocol = config.ProtocolOidc
	}
	methods.Sso = &model.SsoMethod{Protocol: protocol, IsEnforced: IsEnforced(settings, isLicensed), CanStaySignedIn: p.canStaySignedIn(ctx)}
	return methods
}

func (p *AuthMethods) canStaySignedIn(ctx context.Context) bool {
	isProvisioning, err := p.provisioning.IsProvisioning(ctx)
	if err != nil {
		extctx.GetLogger(ctx).Warn().Err(err).Msg("sso: loading the scim state for the login page")
		return false
	}
	return isProvisioning
}
