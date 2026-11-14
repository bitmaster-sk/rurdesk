package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/breakglass"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/enforce"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/groups"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/links"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/oidc"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/scim"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/router"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/crewjam/saml"
	"github.com/gin-gonic/gin"
)

type Deps struct {
	Config     config.IService
	Flows      flow.IStore
	Clients    oidc.IClients
	Resolver   login.IResolver
	Groups     groups.IService
	Links      links.IService
	Enforce    *enforce.Service
	Accounts   *breakglass.Repository
	Scim       *scim.Service
	Replay     saml.AssertionReplayCache
	HttpClient *http.Client
	Users      *repository.UserRepository
	Sessions   *service.UserService
	License    func() license.Provider
}

type Handler struct {
	deps          Deps
	metadataRetry metadataRetry
}

func New(deps Deps) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) Register(groups router.RouteGroups) {
	public := groups.Public.Group("/sso")
	public.GET("/start", h.Start)
	public.GET("/oidc/callback", h.OidcCallback)
	public.POST("/saml/acs", h.SamlAcs)
	public.GET("/saml/metadata", h.SpMetadata)
	public.POST("/exchange", h.Exchange)

	admin := groups.Admin.Group("/sso")
	admin.GET("", h.LoadState)
	admin.POST("/disable", h.Disable)
	admin.PUT("/enforce", h.SetEnforced)
	admin.GET("/mapping", h.ListMappings)
	admin.GET("/identity/:idUser", h.ListIdentities)
	admin.PUT("/scim", h.SetScimEnabled)
	admin.DELETE("/scim/token", h.DeleteScimToken)

	licensed := admin.Group("", h.requireLicense)
	licensed.PUT("/draft", h.SaveDraft)
	licensed.POST("/test", h.StartTest)
	licensed.GET("/test/:idTest", h.LoadTestResult)
	licensed.POST("/activate", h.Activate)
	licensed.PUT("/settings", h.UpdateRules)
	licensed.POST("/mapping", h.InsertMapping)
	licensed.DELETE("/mapping/:idMapping", h.DeleteMapping)
	licensed.DELETE("/identity/:idUser", h.UnlinkIdentity)
	licensed.POST("/identity/:idUser/move", h.MoveIdentity)
	licensed.POST("/scim/token", h.GenerateScimToken)
	licensed.POST("/sp-certificate", h.RegenerateSpCertificate)
}

func (h *Handler) isLicensed(ctx context.Context) bool {
	return h.deps.License().IsAvailable(ctx, license.SSO)
}

func (h *Handler) ready(ctx context.Context) (string, config.Settings, error) {
	if !h.isLicensed(ctx) {
		return "", config.Settings{}, errNotLicensed
	}
	settings, err := h.deps.Config.Settings(ctx)
	if err != nil {
		return "", config.Settings{}, err
	}
	if !settings.IsEnabled {
		return "", config.Settings{}, errNotEnabled
	}
	baseUrl, err := config.ReadyBaseUrl()
	if err != nil {
		return "", config.Settings{}, err
	}
	return baseUrl, settings, nil
}

func (h *Handler) fail(c *gin.Context, code string, err error) {
	extctx.GetLogger(c.Request.Context()).Warn().Err(err).Str("ssoError", code).Msg("sso: sign-in failed")
	c.Redirect(http.StatusFound, "/login?ssoError="+url.QueryEscape(code))
}

func rulesOf(settings config.Settings) login.Rules {
	return login.Rules{AllowedDomains: settings.AllowedDomains, IsAutoCreate: settings.IsAutoCreate, AdminGroup: settings.AdminGroup}
}
