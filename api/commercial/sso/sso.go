package sso

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/breakglass"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/enforce"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/groups"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/handler"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/links"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/oidc"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/policy"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/samlauth"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/scim"
	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/router"
)

const idpTimeout = 10 * time.Second

func Install() {
	injector.SetRouteExtension(registerRoutes)
	injector.SetAuthMethodsProvider(authMethods{})
	injector.SetPasswordLoginPolicy(passwordLogin{})
	injector.SetUserChangeGuard(changeGuard{})
}

func registerRoutes(routes router.RouteGroups) {
	// issue.New opens the pool and the cache before it builds the router, so these never fail here.
	pool, err := injector.GetDb()
	if err != nil {
		panic(fmt.Errorf("sso: database is unavailable: %w", err))
	}
	cache, err := injector.GetCache()
	if err != nil {
		panic(fmt.Errorf("sso: cache is unavailable: %w", err))
	}
	users := injector.GetUserRepository()
	configService := config.NewService(pool)
	accounts := breakglass.NewRepository(pool)
	scimService := scim.NewService(pool, users, injector.GetDeactivationService(), configService)
	groupSync := groups.NewService(pool, users, injector.GetTeamRepository(), injector.GetAclService(), injector.GetUserService())
	httpClient := &http.Client{Timeout: idpTimeout}
	handler.New(handler.Deps{
		Config:     configService,
		Flows:      flow.NewStore(cache),
		Clients:    oidc.NewClients(httpClient),
		Resolver:   login.NewResolver(pool, users, groupSync),
		Groups:     groupSync,
		Links:      links.NewService(pool, users, injector.GetDeactivationService()),
		Enforce:    enforce.NewService(pool, configService, accounts, injector.GetUserService()),
		Accounts:   accounts,
		Scim:       scimService,
		Replay:     samlauth.NewReplayCache(cache),
		HttpClient: httpClient,
		Users:      users,
		Sessions:   injector.GetUserService(),
		License:    injector.GetLicenseProvider,
	}).Register(routes)
	scim.NewHandler(scimService, injector.GetLicenseProvider).Register(routes.Engine)
}

type authMethods struct{}

func (authMethods) Methods(ctx context.Context) model.AuthMethodsRes {
	pool, err := injector.GetDb()
	if err != nil {
		extctx.GetLogger(ctx).Warn().Err(err).Msg("sso: database is unavailable for the login page")
		return authpolicy.Free{}.Methods(ctx)
	}
	return policy.NewAuthMethods(config.NewService(pool), scim.NewRepository(pool), injector.GetLicenseProvider).Methods(ctx)
}

type passwordLogin struct{}

func (passwordLogin) Allow(ctx context.Context, user *model.User) bool {
	pool, err := injector.GetDb()
	if err != nil {
		extctx.GetLogger(ctx).Warn().Err(err).Msg("sso: database is unavailable for a password sign-in")
		return policy.IsBreakGlassAccount(user)
	}
	return policy.NewPasswordLogin(config.NewService(pool), injector.GetLicenseProvider).Allow(ctx, user)
}

type changeGuard struct{}

func (changeGuard) Check(ctx context.Context, change authpolicy.UserChange) error {
	pool, err := injector.GetDb()
	if err != nil {
		return fmt.Errorf("sso: database is unavailable for the break-glass guard: %w", err)
	}
	return policy.NewChangeGuard(config.NewService(pool), breakglass.NewRepository(pool)).Check(ctx, change)
}
