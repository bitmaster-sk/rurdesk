package test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/publicurl"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	persistentNameId = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	transientNameId  = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	emailNameId      = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
)

var signatureElement = regexp.MustCompile(`(?s)<ds:Signature.*?</ds:Signature>`)

type samlFixture struct {
	*ssoFixture
	samlIdp *ssotest.SamlIdp
}

func newSamlFixture(t *testing.T) *samlFixture {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	samlIdp, err := ssotest.NewSamlIdp()
	require.NoError(t, err)
	t.Cleanup(samlIdp.Close)
	return &samlFixture{ssoFixture: f, samlIdp: samlIdp}
}

func (f *samlFixture) saveDraft(t *testing.T, body string) {
	res := f.admin(t, "PUT", "/draft", body)
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
	metadata := f.do("GET", "/api/public/sso/saml/metadata", "", "", nil)
	require.Equal(t, http.StatusOK, metadata.StatusCode)
	require.NoError(t, f.samlIdp.TrustServiceProvider([]byte(readBody(t, metadata))))
}

func (f *samlFixture) saveUrlDraft(t *testing.T) {
	f.saveDraft(t, `{"protocol":"saml","metadataUrl":"`+f.samlIdp.MetadataUrl()+`"}`)
}

func (f *samlFixture) activateSaml(t *testing.T) {
	metadataXml, err := f.samlIdp.MetadataXml()
	require.NoError(t, err)
	body, err := json.Marshal(map[string]string{"protocol": "saml", "metadataXml": metadataXml})
	require.NoError(t, err)
	f.saveDraft(t, string(body))
	ctx := context.Background()
	service := config.NewService(f.app.Pool)
	_, hash, err := service.DraftForTest(ctx)
	require.NoError(t, err)
	require.NoError(t, service.MarkTested(ctx, hash))
	require.NoError(t, service.Activate(ctx, idOfTestUser(t, f.app)))
	require.NoError(t, service.UpdateRules(ctx, config.UpdateRulesReq{AllowedDomains: []string{"acme.test"}, IsAutoCreate: true}))
}

func samlUser(nameId, address string, groups ...string) ssotest.SamlUser {
	return ssotest.SamlUser{
		NameId:       nameId,
		NameIdFormat: persistentNameId,
		Attributes:   map[string][]string{"email": {address}, "name": {"Jana SAML"}, "groups": groups},
	}
}

func (f *samlFixture) postAcs(t *testing.T, post ssotest.SamlPost) *http.Response {
	acsUrl, err := url.Parse(post.AcsUrl)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", acsUrl.Path, strings.NewReader(post.Form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	f.app.ServeHTTP(w, req)
	return w.Result()
}

func (f *samlFixture) authorizeSaml(t *testing.T, start ssoStart, user ssotest.SamlUser) ssotest.SamlPost {
	post, err := f.samlIdp.Authorize(start.authUrl, user)
	require.NoError(t, err)
	return post
}

func TestSaml_SignInEndToEnd(t *testing.T) {
	f := newSamlFixture(t)
	f.activateSaml(t)
	address := uniqueSsoEmail("saml")

	start := f.start(t, "?returnUrl=/project")
	assert.Contains(t, start.authUrl, f.samlIdp.MetadataUrl()[:len(f.samlIdp.MetadataUrl())-len("/metadata")]+"/sso?SAMLRequest=")
	assert.Contains(t, start.authUrl, "Signature=", "the AuthnRequest is signed")
	res := f.postAcs(t, f.authorizeSaml(t, start, samlUser("saml-"+address, address, "dev")))
	exchanged := f.exchange(codeFrom(t, res), start.cookies)
	body := readBody(t, exchanged)
	require.Equal(t, http.StatusOK, exchanged.StatusCode, body)
	var session ssoSession
	require.NoError(t, json.Unmarshal([]byte(body), &session))
	assert.Equal(t, "/project", session.ReturnUrl)

	user := readBody(t, Request(t, f.app, "GET", "/api/private/user", "", session.Token))
	assert.Contains(t, user, address)
	assert.Contains(t, user, "Jana SAML")

	methods := Request(t, f.app, "GET", "/api/public/auth/methods", "", "")
	var res2 model.AuthMethodsRes
	require.NoError(t, json.NewDecoder(methods.Body).Decode(&res2))
	require.NotNil(t, res2.Sso)
	assert.Equal(t, config.ProtocolSaml, res2.Sso.Protocol)
}

func TestSaml_ConnectionTestFetchesTheMetadata(t *testing.T) {
	f := newSamlFixture(t)
	f.saveUrlDraft(t)
	assert.False(t, f.state(t).IsDraftTested)

	authUrl := f.startTest(t)
	post, err := f.samlIdp.Authorize(authUrl, ssotest.SamlUser{
		NameId:       "jana@acme.test",
		NameIdFormat: emailNameId,
		Attributes:   map[string][]string{"groups": {"dev"}},
	})
	require.NoError(t, err)
	result := f.testResult(t, f.postAcs(t, post))

	assert.True(t, result.IsSuccess, result.Detail)
	assert.Equal(t, "jana@acme.test", result.Email, "an e-mail NameID fills the e-mail")
	assert.Contains(t, result.Warnings, "saml_subject_unstable")
	state := f.state(t)
	assert.True(t, state.IsDraftTested)
	assert.Equal(t, f.samlIdp.EntityId(), state.Draft.Issuer)
	assert.Equal(t, http.StatusOK, f.admin(t, "POST", "/activate", "").StatusCode)
}

func TestSaml_RejectsReplayedUnsolicitedAndUnsignedResponses(t *testing.T) {
	f := newSamlFixture(t)
	f.activateSaml(t)
	address := uniqueSsoEmail("saml-reject")
	user := samlUser("saml-"+address, address)

	first := f.start(t, "")
	used := f.authorizeSaml(t, first, user)
	codeFrom(t, f.postAcs(t, used))
	assert.Equal(t, "invalid_state", ssoErrorFrom(t, f.postAcs(t, used)))

	replayed := used
	replayed.Form = url.Values{"SAMLResponse": used.Form["SAMLResponse"], "RelayState": {relayStateOf(t, f.start(t, ""))}}
	assert.Equal(t, "idp_error", ssoErrorFrom(t, f.postAcs(t, replayed)))

	unsolicited, err := f.samlIdp.Unsolicited(ssoBaseUrl+config.SpMetadataPath, user)
	require.NoError(t, err)
	unsolicited.Form.Set("RelayState", relayStateOf(t, f.start(t, "")))
	assert.Equal(t, "idp_error", ssoErrorFrom(t, f.postAcs(t, unsolicited)))

	f.samlIdp.StopEncrypting()
	codeFrom(t, f.postAcs(t, f.authorizeSaml(t, f.start(t, ""), user)))
	unsigned := f.authorizeSaml(t, f.start(t, ""), user)
	decoded, err := base64.StdEncoding.DecodeString(unsigned.Form.Get("SAMLResponse"))
	require.NoError(t, err)
	stripped := signatureElement.ReplaceAll(decoded, nil)
	require.NotEqual(t, decoded, stripped)
	unsigned.Form.Set("SAMLResponse", base64.StdEncoding.EncodeToString(stripped))
	assert.Equal(t, "idp_error", ssoErrorFrom(t, f.postAcs(t, unsigned)))
}

func relayStateOf(t *testing.T, start ssoStart) string {
	authUrl, err := url.Parse(start.authUrl)
	require.NoError(t, err)
	return authUrl.Query().Get("RelayState")
}

func TestSaml_TransientNameIdIsRejected(t *testing.T) {
	f := newSamlFixture(t)
	f.activateSaml(t)
	user := samlUser("tmp-1", uniqueSsoEmail("saml-transient"))
	user.NameIdFormat = transientNameId

	res := f.postAcs(t, f.authorizeSaml(t, f.start(t, ""), user))

	assert.Equal(t, "saml_subject_unsupported", ssoErrorFrom(t, res))
}

func TestSaml_MetadataNeedsACertificate(t *testing.T) {
	f := newSamlFixture(t)
	_, err := f.app.Pool.Exec(context.Background(), `UPDATE users.sso_settings SET sp_cert = '', sp_key_enc = NULL, sp_key_nonce = NULL`)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, f.do("GET", "/api/public/sso/saml/metadata", "", "", nil).StatusCode)

	f.saveUrlDraft(t)
	metadata := readBody(t, f.do("GET", "/api/public/sso/saml/metadata", "", "", nil))
	assert.Contains(t, metadata, `entityID="`+ssoBaseUrl+config.SpMetadataPath+`"`)
	assert.Contains(t, metadata, ssoBaseUrl+config.AcsPath)
	state := f.state(t)
	assert.Equal(t, ssoBaseUrl+config.AcsPath, state.AcsUrl)
	assert.False(t, state.IsSpKeyUndecryptable)
}

func TestSaml_SpMetadataIsServedOnceTheSettingsAreOpened(t *testing.T) {
	f := newSamlFixture(t)
	state := f.state(t)
	require.Nil(t, state.Draft)
	require.Equal(t, ssoBaseUrl+config.SpMetadataPath, state.SpEntityId)

	res := f.do("GET", config.SpMetadataPath, "", "", nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, readBody(t, res), `entityID="`+state.SpEntityId+`"`)
	certificate := f.spCertificate(t)
	require.NotEmpty(t, certificate)

	f.state(t)
	assert.Equal(t, certificate, f.spCertificate(t), "opening the settings again keeps the certificate")
}

func TestSaml_NoCertificateIsCreatedWithoutAPublicUrl(t *testing.T) {
	f := newSamlFixture(t)
	viper.Set(publicurl.EnvBaseUrl, "http://tracker.example.org")

	assert.False(t, f.state(t).IsPublicBaseUrlValid)

	assert.Empty(t, f.spCertificate(t))
}

func (f *samlFixture) spCertificate(t *testing.T) string {
	var certificate string
	require.NoError(t, f.app.Pool.QueryRow(context.Background(), `SELECT sp_cert FROM users.sso_settings`).Scan(&certificate))
	return certificate
}

func TestSaml_LostKeyNeedsANewCertificate(t *testing.T) {
	f := newSamlFixture(t)
	f.saveUrlDraft(t)
	before := readBody(t, f.do("GET", "/api/public/sso/saml/metadata", "", "", nil))
	assert.Equal(t, http.StatusUnprocessableEntity, f.admin(t, "POST", "/sp-certificate", "").StatusCode, "a working key stays")

	viper.Set(config.EnvEncryptionKey, ssoOtherKey)
	assert.True(t, f.state(t).IsSpKeyUndecryptable)
	require.Equal(t, http.StatusOK, f.admin(t, "POST", "/sp-certificate", "").StatusCode)

	assert.False(t, f.state(t).IsSpKeyUndecryptable)
	assert.NotEqual(t, before, readBody(t, f.do("GET", "/api/public/sso/saml/metadata", "", "", nil)))
}

func TestSaml_DraftNeedsMetadata(t *testing.T) {
	f := newSamlFixture(t)
	for body, code := range map[string]string{
		`{"protocol":"saml"}`:                                             "SSO_METADATA_REQUIRED",
		`{"protocol":"saml","metadataXml":"<x/>"}`:                        "SSO_METADATA_INVALID",
		`{"protocol":"saml","metadataUrl":"http://idp.example.com/meta"}`: "SSO_METADATA_URL_INVALID",
		`{"protocol":"oidc","issuer":"https://idp.example.com"}`:          "SSO_CLIENT_ID_REQUIRED",
	} {
		res := f.admin(t, "PUT", "/draft", body)
		assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode, body)
		assert.Contains(t, readBody(t, res), code, body)
	}
}

func TestSaml_RefusesMetadataUrlTogetherWithXml(t *testing.T) {
	f := newSamlFixture(t)
	f.saveUrlDraft(t)
	other, err := ssotest.NewSamlIdp()
	require.NoError(t, err)
	t.Cleanup(other.Close)
	otherXml, err := other.MetadataXml()
	require.NoError(t, err)
	draftBody := func(metadataUrl string) string {
		body, err := json.Marshal(map[string]string{"protocol": "saml", "metadataUrl": metadataUrl, "metadataXml": otherXml})
		require.NoError(t, err)
		return string(body)
	}

	res := f.admin(t, "PUT", "/draft", draftBody(f.samlIdp.MetadataUrl()))
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	assert.Contains(t, readBody(t, res), "SSO_METADATA_AMBIGUOUS")
	assert.Equal(t, f.samlIdp.MetadataUrl(), f.state(t).Draft.MetadataUrl, "the saved draft stays")

	require.Equal(t, http.StatusOK, f.admin(t, "PUT", "/draft", draftBody("  ")).StatusCode)
	draft := f.state(t).Draft
	assert.Empty(t, draft.MetadataUrl)
	assert.Equal(t, other.EntityId(), draft.Issuer, "the pasted metadata is the saved one")
}

func TestSaml_MetadataRefreshKeepsTheDraftActive(t *testing.T) {
	f := newSamlFixture(t)
	f.saveUrlDraft(t)
	post, err := f.samlIdp.Authorize(f.startTest(t), samlUser("sub-refresh", "refresh@acme.test"))
	require.NoError(t, err)
	require.True(t, f.testResult(t, f.postAcs(t, post)).IsSuccess)
	require.Equal(t, http.StatusOK, f.admin(t, "POST", "/activate", "").StatusCode)
	require.True(t, f.state(t).IsDraftActive)

	_, err = f.app.Pool.Exec(context.Background(),
		`UPDATE users.sso_config SET idp_metadata_fetched_at = now() - interval '2 days' WHERE kind = 'active'`)
	require.NoError(t, err)
	stale := f.activeFetchedAt(t)
	f.start(t, "")
	require.NotEqual(t, stale, f.activeFetchedAt(t), "the sign-in downloaded the metadata again")

	state := f.state(t)
	assert.True(t, state.IsDraftActive, "nobody edited the connection, so the draft is still the live one")
	assert.True(t, state.IsDraftTested)
}

func TestSaml_AUrlDraftWithoutDownloadedMetadataNeedsANewTest(t *testing.T) {
	f := newSamlFixture(t)
	f.saveUrlDraft(t)
	post, err := f.samlIdp.Authorize(f.startTest(t), samlUser("sub-retest", "retest@acme.test"))
	require.NoError(t, err)
	require.True(t, f.testResult(t, f.postAcs(t, post)).IsSuccess)
	require.True(t, f.state(t).IsDraftTested)

	require.Equal(t, http.StatusOK, f.admin(t, "PUT", "/draft", `{"protocol":"saml","metadataUrl":"https://other.example.com/metadata"}`).StatusCode)
	f.saveUrlDraft(t)

	state := f.state(t)
	require.False(t, state.Draft.HasMetadata, "saving another URL dropped the downloaded copy")
	assert.False(t, state.IsDraftTested, "the earlier test must not unlock a draft that holds no metadata")
	assert.Equal(t, http.StatusUnprocessableEntity, f.admin(t, "POST", "/activate", "").StatusCode)
}

func (f *samlFixture) activeFetchedAt(t *testing.T) string {
	var fetchedAt string
	err := f.app.Pool.QueryRow(context.Background(),
		`SELECT coalesce(idp_metadata_fetched_at::text, '') FROM users.sso_config WHERE kind = 'active'`).Scan(&fetchedAt)
	require.NoError(t, err)
	return fetchedAt
}

func TestSaml_ActiveMetadataIsRefreshedLazily(t *testing.T) {
	f := newSamlFixture(t)
	f.saveUrlDraft(t)
	post, err := f.samlIdp.Authorize(f.startTest(t), samlUser("sub-refresh", "refresh@acme.test"))
	require.NoError(t, err)
	require.True(t, f.testResult(t, f.postAcs(t, post)).IsSuccess)
	require.Equal(t, http.StatusOK, f.admin(t, "POST", "/activate", "").StatusCode)
	_, err = f.app.Pool.Exec(context.Background(),
		`UPDATE users.sso_config SET idp_metadata_fetched_at = now() - interval '2 days' WHERE kind = 'active'`)
	require.NoError(t, err)
	stale := f.activeFetchedAt(t)

	f.start(t, "")
	refreshed := f.activeFetchedAt(t)
	assert.NotEqual(t, stale, refreshed)

	_, err = f.app.Pool.Exec(context.Background(),
		`UPDATE users.sso_config SET idp_metadata_url = 'http://127.0.0.1:1/metadata', idp_metadata_fetched_at = now() - interval '2 days' WHERE kind = 'active'`)
	require.NoError(t, err)
	unreachable := f.activeFetchedAt(t)
	f.start(t, "")
	assert.Equal(t, unreachable, f.activeFetchedAt(t), "a failed refresh keeps the last metadata and the sign-in still starts")

	_, err = f.app.Pool.Exec(context.Background(),
		`UPDATE users.sso_config SET idp_metadata_url = $1 WHERE kind = 'active'`, f.samlIdp.MetadataUrl())
	require.NoError(t, err)
	f.start(t, "")
	assert.Equal(t, unreachable, f.activeFetchedAt(t), "after a failed refresh the next sign-ins do not download the metadata again")
}
