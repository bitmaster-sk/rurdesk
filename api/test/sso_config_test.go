package test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newConfigService(t *testing.T) (*issue.Application, *config.Service, int64) {
	app := Setup(t)
	useSsoEnv(t)
	resetSso(t, app)
	return app, config.NewService(app.Pool), idOfTestUser(t, app)
}

func draftReq() config.SaveDraftReq {
	return config.SaveDraftReq{
		Protocol:     config.ProtocolOidc,
		Issuer:       "https://idp.example.com",
		ClientId:     "rurdesk",
		ClientSecret: "s3cret",
	}
}

func TestSsoConfig_FirstDraftNeedsASecret(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	req := draftReq()
	req.ClientSecret = ""

	err := service.SaveDraft(context.Background(), idAdmin, req)

	assert.ErrorIs(t, err, config.ErrSecretRequired)
}

func TestSsoConfig_EmptySecretKeepsTheSavedOneAndStateNeverShowsIt(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	require.NoError(t, service.SaveDraft(ctx, idAdmin, draftReq()))

	req := draftReq()
	req.ClientSecret = ""
	req.ClientId = "rurdesk-2"
	require.NoError(t, service.SaveDraft(ctx, idAdmin, req))

	connection, _, err := service.DraftForTest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", connection.ClientSecret)
	assert.Equal(t, "rurdesk-2", connection.ClientId)
	assert.Equal(t, config.DefaultEmailClaim, connection.EmailClaim)
	assert.Equal(t, config.DefaultGroupsClaim, connection.GroupsClaim)

	state, err := service.LoadState(ctx)
	require.NoError(t, err)
	require.NotNil(t, state.Draft)
	assert.True(t, state.Draft.HasClientSecret)
	assert.Nil(t, state.Active)
	assert.Equal(t, ssoBaseUrl+config.CallbackPath, state.RedirectUri)
	body, err := json.Marshal(state)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "s3cret")
}

func TestSsoConfig_ActivationNeedsASuccessfulTestOfTheCurrentDraft(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	require.NoError(t, service.SaveDraft(ctx, idAdmin, draftReq()))

	assert.ErrorIs(t, service.Activate(ctx, idAdmin), config.ErrNotTested)

	_, hash, err := service.DraftForTest(ctx)
	require.NoError(t, err)
	require.NoError(t, service.MarkTested(ctx, hash))
	require.NoError(t, service.Activate(ctx, idAdmin))

	settings, err := service.Settings(ctx)
	require.NoError(t, err)
	assert.True(t, settings.IsEnabled)
	active, err := service.Active(ctx)
	require.NoError(t, err)
	assert.Equal(t, "https://idp.example.com", active.Issuer)
	assert.Equal(t, "s3cret", active.ClientSecret)

	state, err := service.LoadState(ctx)
	require.NoError(t, err)
	assert.True(t, state.IsDraftTested)
	assert.True(t, state.IsDraftActive)
	require.NotNil(t, state.Active)
}

func TestSsoConfig_EditingTheDraftKeepsTheActiveConnection(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	activateDraft(t, service, idAdmin, draftReq())

	req := draftReq()
	req.Issuer = "https://other.example.com"
	require.NoError(t, service.SaveDraft(ctx, idAdmin, req))

	active, err := service.Active(ctx)
	require.NoError(t, err)
	assert.Equal(t, "https://idp.example.com", active.Issuer)
	state, err := service.LoadState(ctx)
	require.NoError(t, err)
	assert.False(t, state.IsDraftTested)
	assert.False(t, state.IsDraftActive)
	assert.ErrorIs(t, service.Activate(ctx, idAdmin), config.ErrNotTested)

	require.NoError(t, service.SaveDraft(ctx, idAdmin, draftReq()))
	state, err = service.LoadState(ctx)
	require.NoError(t, err)
	assert.True(t, state.IsDraftTested)
	assert.True(t, state.IsDraftActive)
}

func TestSsoConfig_TestOfAChangedDraftIsRejected(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	require.NoError(t, service.SaveDraft(ctx, idAdmin, draftReq()))
	_, hash, err := service.DraftForTest(ctx)
	require.NoError(t, err)

	req := draftReq()
	req.EmailClaim = "preferred_username"
	require.NoError(t, service.SaveDraft(ctx, idAdmin, req))

	assert.ErrorIs(t, service.MarkTested(ctx, hash), config.ErrDraftChanged)
}

func TestSsoConfig_ActivationNeedsAnHttpsPublicUrl(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	require.NoError(t, service.SaveDraft(ctx, idAdmin, draftReq()))
	_, hash, err := service.DraftForTest(ctx)
	require.NoError(t, err)
	require.NoError(t, service.MarkTested(ctx, hash))

	for _, baseUrl := range []string{"", "http://tracker.example.org"} {
		viper.Set(publicurl.EnvBaseUrl, baseUrl)
		assert.ErrorIs(t, service.Activate(ctx, idAdmin), config.ErrPublicUrlInvalid, baseUrl)
	}
	state, err := service.LoadState(ctx)
	require.NoError(t, err)
	assert.False(t, state.IsPublicBaseUrlValid)
	assert.Empty(t, state.RedirectUri)
}

func TestSsoConfig_RulesAreNormalized(t *testing.T) {
	_, service, _ := newConfigService(t)
	ctx := context.Background()

	err := service.UpdateRules(ctx, config.UpdateRulesReq{AllowedDomains: []string{" Acme.TEST ", "acme.test"}, IsAutoCreate: true})
	require.NoError(t, err)

	settings, err := service.Settings(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"acme.test"}, settings.AllowedDomains)
	assert.True(t, settings.IsAutoCreate)
	assert.ErrorIs(t, service.UpdateRules(ctx, config.UpdateRulesReq{AllowedDomains: []string{"not a domain"}}), config.ErrDomainInvalid)
}

func TestSsoConfig_DisableKeepsTheConfiguration(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	activateDraft(t, service, idAdmin, draftReq())

	require.NoError(t, service.Disable(ctx))

	settings, err := service.Settings(ctx)
	require.NoError(t, err)
	assert.False(t, settings.IsEnabled)
	_, err = service.Active(ctx)
	assert.NoError(t, err)
}

func TestSsoConfig_ReportsAnUndecryptableSecretAndAMissingKey(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	ctx := context.Background()
	require.NoError(t, service.SaveDraft(ctx, idAdmin, draftReq()))

	viper.Set(config.EnvEncryptionKey, ssoOtherKey)
	state, err := service.LoadState(ctx)
	require.NoError(t, err)
	assert.True(t, state.HasEncryptionKey)
	assert.True(t, state.IsSecretUndecryptable)
	_, _, err = service.DraftForTest(ctx)
	assert.ErrorIs(t, err, config.ErrSecretUndecryptable)

	viper.Set(config.EnvEncryptionKey, "")
	state, err = service.LoadState(ctx)
	require.NoError(t, err)
	assert.False(t, state.HasEncryptionKey)
	assert.ErrorIs(t, service.SaveDraft(ctx, idAdmin, draftReq()), config.ErrKeyMissing)
}

func TestSsoConfig_RejectsAMultiTenantIssuer(t *testing.T) {
	_, service, idAdmin := newConfigService(t)
	req := draftReq()
	req.Issuer = "https://login.microsoftonline.com/common/v2.0"

	assert.ErrorIs(t, service.SaveDraft(context.Background(), idAdmin, req), config.ErrIssuerMultiTenant)
}

func activateDraft(t *testing.T, service *config.Service, idAdmin int64, req config.SaveDraftReq) {
	ctx := context.Background()
	require.NoError(t, service.SaveDraft(ctx, idAdmin, req))
	_, hash, err := service.DraftForTest(ctx)
	require.NoError(t, err)
	require.NoError(t, service.MarkTested(ctx, hash))
	require.NoError(t, service.Activate(ctx, idAdmin))
}
