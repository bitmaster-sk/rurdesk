package handler

import (
	"errors"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/samlauth"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/gin-gonic/gin"
)

func (h *Handler) Start(c *gin.Context) {
	ctx := c.Request.Context()
	baseUrl, _, err := h.ready(ctx)
	if err != nil {
		h.fail(c, errorSsoUnavailable, err)
		return
	}
	// A proxy that rewrites Host never shows the public host, so the move to it happens once.
	if !isSameHost(c.Request.Host, baseUrl) && c.Query(redirectedParam) == "" {
		c.Redirect(http.StatusFound, baseUrl+redirectedTarget(c.Request.URL))
		return
	}
	connection, err := h.activeConnection(ctx)
	if err != nil {
		h.fail(c, errorSsoUnavailable, err)
		return
	}
	binding, err := flow.NewBinding()
	if err != nil {
		h.fail(c, errorInternal, err)
		return
	}
	state, err := flow.NewState(binding.Hash, c.Query("returnUrl"), c.Query("extended") == "true")
	if err != nil {
		h.fail(c, errorInternal, err)
		return
	}
	authUrl, err := h.authUrl(ctx, connection, baseUrl, state)
	if err != nil {
		h.fail(c, errorSsoUnavailable, err)
		return
	}
	setBindingCookie(c, binding)
	c.Redirect(http.StatusFound, authUrl)
}

func (h *Handler) OidcCallback(c *gin.Context) {
	h.finish(c, c.Query("state"))
}

func (h *Handler) SamlAcs(c *gin.Context) {
	h.finish(c, c.PostForm("RelayState"))
}

func (h *Handler) finish(c *gin.Context, stateKey string) {
	ctx := c.Request.Context()
	state, err := h.deps.Flows.TakeState(ctx, stateKey)
	if err != nil {
		h.fail(c, errorInvalidState, err)
		return
	}
	if state.IsTest {
		h.finishTest(c, state)
		return
	}
	baseUrl, settings, err := h.ready(ctx)
	if err != nil {
		h.fail(c, errorSsoUnavailable, err)
		return
	}
	connection, err := h.activeConnection(ctx)
	if err != nil {
		h.fail(c, errorSsoUnavailable, err)
		return
	}
	id, err := h.identity(c, state, connection, baseUrl)
	if errors.Is(err, samlauth.ErrSubjectUnsupported) {
		h.fail(c, errorSamlSubjectUnsupported, err)
		return
	}
	if err != nil {
		h.fail(c, errorIdp, err)
		return
	}
	outcome, err := h.deps.Resolver.Resolve(ctx, id, rulesOf(settings))
	if err != nil {
		h.fail(c, errorInternal, err)
		return
	}
	if outcome.Action == login.ActionReject {
		h.fail(c, string(outcome.Reason), nil)
		return
	}
	code, err := h.deps.Flows.SaveCode(ctx, flow.Code{
		IdUser:      outcome.User.IdUser,
		IsExtended:  state.IsExtended,
		ReturnUrl:   state.ReturnUrl,
		BindingHash: state.BindingHash,
	})
	if err != nil {
		h.fail(c, errorInternal, err)
		return
	}
	c.Redirect(http.StatusFound, "/auth/sso#code="+code)
}

func (h *Handler) SpMetadata(c *gin.Context) {
	ctx := c.Request.Context()
	baseUrl, err := config.ReadyBaseUrl()
	if err != nil {
		controller.ResponseErr(c, errs.ErrNotFound)
		return
	}
	pair, err := h.deps.Config.KeyPair(ctx)
	if errors.Is(err, config.ErrSpKeyMissing) {
		controller.ResponseErr(c, errs.ErrNotFound)
		return
	}
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	metadata, err := samlauth.SpMetadata(pair, baseUrl)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	c.Data(http.StatusOK, "application/samlmetadata+xml", metadata)
}

func (h *Handler) Exchange(c *gin.Context) {
	var req ExchangeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	ctx := c.Request.Context()
	if !h.isLicensed(ctx) {
		controller.ResponseErr(c, errs.ErrLicenseRequired)
		return
	}
	code, err := h.deps.Flows.TakeCode(ctx, req.Code)
	if errors.Is(err, flow.ErrNotFound) {
		controller.ResponseErr(c, ErrInvalidCode)
		return
	}
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	cookieName := flow.CookieName(code.BindingHash)
	cookieValue, err := c.Cookie(cookieName)
	clearBindingCookie(c, cookieName)
	if err != nil || !flow.IsBindingValid(cookieValue, code.BindingHash) {
		controller.ResponseErr(c, ErrInvalidCode)
		return
	}
	user, err := h.deps.Users.LoadUser(ctx, code.IdUser)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	if !user.IsActive || user.IsAgent {
		controller.ResponseErr(c, ErrUserDisabled)
		return
	}
	isProvisioning, err := h.deps.Scim.IsProvisioning(ctx)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	// A long SSO session is only safe while SCIM can revoke it when the person leaves.
	token, err := h.deps.Sessions.CreateSession(ctx, user, code.IsExtended && isProvisioning)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	controller.ResponseOk(c, ExchangeRes{Token: token, ReturnUrl: code.ReturnUrl})
}
