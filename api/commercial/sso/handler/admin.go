package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/oidc"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/samlauth"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) requireLicense(c *gin.Context) {
	if !h.isLicensed(c.Request.Context()) {
		controller.ResponseErr(c, errs.ErrLicenseRequired)
		c.Abort()
	}
}

func (h *Handler) LoadState(c *gin.Context) {
	ctx := c.Request.Context()
	state, err := h.deps.Config.LoadState(ctx)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	state.IsLicensed = h.isLicensed(ctx)
	state.BreakGlassAccounts, err = h.deps.Accounts.ListActive(ctx)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	scimState, err := h.deps.Scim.State(ctx)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	controller.ResponseOk(c, StateRes{StateRes: state, Scim: scimState})
}

func (h *Handler) SetEnforced(c *gin.Context) {
	var req config.SetEnforcedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	ctx := c.Request.Context()
	if *req.IsEnforced && !h.isLicensed(ctx) {
		controller.ResponseErr(c, errs.ErrLicenseRequired)
		return
	}
	acting, _ := extctx.GetUser(ctx)
	if err := h.deps.Enforce.Set(ctx, *req.IsEnforced, acting.IdUser, extctx.BearerToken(c.GetHeader("Authorization"))); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) SaveDraft(c *gin.Context) {
	var req config.SaveDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	ctx := c.Request.Context()
	acting, _ := extctx.GetUser(ctx)
	if err := h.deps.Config.SaveDraft(ctx, acting.IdUser, req); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) Activate(c *gin.Context) {
	ctx := c.Request.Context()
	acting, _ := extctx.GetUser(ctx)
	if err := h.deps.Config.Activate(ctx, acting.IdUser); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) Disable(c *gin.Context) {
	if err := h.deps.Config.Disable(c.Request.Context()); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) UpdateRules(c *gin.Context) {
	var req config.UpdateRulesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	if err := h.deps.Config.UpdateRules(c.Request.Context(), req); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) StartTest(c *gin.Context) {
	ctx := c.Request.Context()
	baseUrl, err := config.ReadyBaseUrl()
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	if err := h.refreshDraftMetadata(ctx); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	connection, hash, err := h.deps.Config.DraftForTest(ctx)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	state, err := flow.NewTestState(hash)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	authUrl, err := h.authUrl(ctx, connection, baseUrl, state)
	if err != nil {
		controller.ResponseErr(c, ErrDiscoveryFailed.WithMessage(err.Error()))
		return
	}
	controller.ResponseOk(c, StartTestRes{Url: authUrl})
}

func (h *Handler) refreshDraftMetadata(ctx context.Context) error {
	draft, _, err := h.deps.Config.DraftForTest(ctx)
	if err != nil || draft.Protocol != config.ProtocolSaml || draft.MetadataUrl == "" {
		return err
	}
	metadataXml, err := samlauth.FetchMetadata(ctx, h.deps.HttpClient, draft.MetadataUrl)
	if err != nil {
		return ErrMetadataUnreachable.WithMessage(err.Error())
	}
	return h.deps.Config.SaveMetadata(ctx, false, metadataXml)
}

func (h *Handler) RegenerateSpCertificate(c *gin.Context) {
	if err := h.deps.Config.RegenerateKeyPair(c.Request.Context()); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.LoadState(c)
}

func (h *Handler) LoadTestResult(c *gin.Context) {
	result, err := h.deps.Flows.LoadTestResult(c.Request.Context(), c.Param("idTest"))
	if errors.Is(err, flow.ErrNotFound) {
		controller.ResponseErr(c, errs.ErrNotFound)
		return
	}
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	controller.ResponseOk(c, result)
}

func (h *Handler) finishTest(c *gin.Context, state flow.State) {
	idTest, err := h.deps.Flows.SaveTestResult(c.Request.Context(), h.runTest(c, state))
	if err != nil {
		h.fail(c, errorInternal, err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/settings/sso?test="+url.QueryEscape(idTest))
}

func (h *Handler) runTest(c *gin.Context, state flow.State) login.TestResult {
	ctx := c.Request.Context()
	id, hash, failure := h.testIdentity(c, state)
	if failure != nil {
		return *failure
	}
	settings, err := h.deps.Config.Settings(ctx)
	if err != nil {
		return login.FailedTest(login.TestErrorInternal, err.Error())
	}
	rules := rulesOf(settings)
	outcome, err := h.deps.Resolver.Preview(ctx, id, rules)
	if err != nil {
		return login.FailedTest(login.TestErrorInternal, err.Error())
	}
	if err := h.deps.Config.MarkTested(ctx, hash); err != nil {
		if errors.Is(err, config.ErrDraftChanged) {
			return login.FailedTest(login.TestErrorDraftChanged, "")
		}
		return login.FailedTest(login.TestErrorInternal, err.Error())
	}
	return login.SucceededTest(id, outcome, rules)
}

func (h *Handler) testIdentity(c *gin.Context, state flow.State) (identity.Identity, string, *login.TestResult) {
	ctx := c.Request.Context()
	fail := func(code login.TestError, detail string) (identity.Identity, string, *login.TestResult) {
		result := login.FailedTest(code, detail)
		return identity.Identity{}, "", &result
	}
	if !h.isLicensed(ctx) {
		return fail(login.TestErrorSsoUnavailable, "the license does not include SSO")
	}
	baseUrl, err := config.ReadyBaseUrl()
	if err != nil {
		return fail(login.TestErrorSsoUnavailable, err.Error())
	}
	connection, hash, err := h.deps.Config.DraftForTest(ctx)
	if err != nil {
		return fail(login.TestErrorInternal, err.Error())
	}
	if hash != state.TestHash {
		return fail(login.TestErrorDraftChanged, "")
	}
	id, err := h.identity(c, state, connection, baseUrl)
	switch {
	case errors.Is(err, errIdpReturnedError):
		return fail(login.TestErrorIdp, err.Error())
	case errors.Is(err, oidc.ErrSubjectMissing), errors.Is(err, samlauth.ErrSubjectMissing):
		return fail(login.TestErrorSubjectMissing, err.Error())
	case errors.Is(err, samlauth.ErrSubjectUnsupported):
		return fail(login.TestErrorSubjectUnsupported, err.Error())
	case err != nil:
		return fail(login.TestErrorTokenInvalid, err.Error())
	}
	return id, hash, nil
}
