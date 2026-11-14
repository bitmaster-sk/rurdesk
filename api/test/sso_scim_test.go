package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scimUser struct {
	Id          string `json:"id"`
	ExternalId  string `json:"externalId"`
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

type scimList struct {
	TotalResults int        `json:"totalResults"`
	Resources    []scimUser `json:"Resources"`
}

type scimError struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType"`
}

type scimFixture struct {
	*ssoFixture
	token string
}

func newScimFixture(t *testing.T) *scimFixture {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	err := config.NewService(f.app.Pool).UpdateRules(context.Background(), config.UpdateRulesReq{AllowedDomains: []string{"acme.test"}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, f.admin(t, "PUT", "/scim", `{"isEnabled":true}`).StatusCode)
	res := f.admin(t, "POST", "/scim/token", "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&created))
	require.NotEmpty(t, created.Token)
	return &scimFixture{ssoFixture: f, token: created.Token}
}

func (f *scimFixture) scim(method, path, body string) *http.Response {
	return f.scimWith(f.token, method, path, body)
}

func (f *scimFixture) scimWith(token, method, path, body string) *http.Response {
	headers := map[string]string{"Content-Type": "application/scim+json"}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return RequestWithHeaders(nil, f.app, method, "/api/scim/v2"+path, body, headers)
}

func (f *scimFixture) create(t *testing.T, body string) scimUser {
	res := f.scim("POST", "/Users", body)
	raw := readBody(t, res)
	require.Equal(t, http.StatusCreated, res.StatusCode, raw)
	var user scimUser
	require.NoError(t, json.Unmarshal([]byte(raw), &user))
	return user
}

func decodeScimUser(t *testing.T, res *http.Response) scimUser {
	var user scimUser
	require.NoError(t, json.NewDecoder(res.Body).Decode(&user))
	return user
}

func decodeScimError(t *testing.T, res *http.Response) scimError {
	assert.Equal(t, "application/scim+json", res.Header.Get("Content-Type"))
	var body scimError
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, []string{"urn:ietf:params:scim:api:messages:2.0:Error"}, body.Schemas)
	return body
}

func newScimUserBody(address, externalId string) string {
	return fmt.Sprintf(`{
		"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],
		"externalId":%q,
		"userName":"upn-%s",
		"active":true,
		"displayName":"Scim Person",
		"emails":[{"primary":true,"type":"work","value":%q}]
	}`, externalId, address, address)
}

func (f *scimFixture) deactivate(id, active string) *http.Response {
	return f.scim("PATCH", "/Users/"+id, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
		"Operations":[{"op":"Replace","path":"active","value":`+active+`}]}`)
}

func TestScim_NeedsTheTokenAndTheSwitch(t *testing.T) {
	f := newScimFixture(t)

	for _, token := range []string{"", "rdscim_wrong"} {
		res := f.scimWith(token, "GET", "/Users", "")
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
		assert.Equal(t, "401", decodeScimError(t, res).Status)
	}
	assert.Equal(t, http.StatusOK, f.scim("GET", "/ServiceProviderConfig", "").StatusCode)

	require.Equal(t, http.StatusOK, f.admin(t, "PUT", "/scim", `{"isEnabled":false}`).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, f.scim("GET", "/Users", "").StatusCode)
}

func TestScim_CreatesAndFindsUsers(t *testing.T) {
	f := newScimFixture(t)
	f.activate(t, false)
	address := uniqueSsoEmail("scim-new")
	t.Cleanup(func() { deleteUsers(f.app, address) })

	created := f.create(t, newScimUserBody(address, "ext-"+address))
	assert.Equal(t, address, created.UserName, "the primary e-mail wins over the userName")
	assert.Equal(t, "Scim Person", created.DisplayName)
	assert.True(t, created.Active)

	res := f.scim("GET", "/Users/"+created.Id, "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "ext-"+address, decodeScimUser(t, res).ExternalId)

	for _, filter := range []string{
		fmt.Sprintf(`userName eq "%s"`, strings.ToUpper(address)),
		fmt.Sprintf(`externalId eq "ext-%s"`, address),
	} {
		res := f.scim("GET", "/Users?filter="+url.QueryEscape(filter), "")
		require.Equal(t, http.StatusOK, res.StatusCode)
		var list scimList
		require.NoError(t, json.NewDecoder(res.Body).Decode(&list))
		require.Equal(t, 1, list.TotalResults, filter)
		assert.Equal(t, created.Id, list.Resources[0].Id)
	}
	assert.Equal(t, http.StatusBadRequest, f.scim("GET", "/Users?filter="+url.QueryEscape(`name co "x"`), "").StatusCode)

	session := f.signIn(t, "", ssoClaims("sub-"+address, address))
	assert.Equal(t, http.StatusOK, Request(t, f.app, "GET", "/api/private/user", "", session.Token).StatusCode,
		"the first SSO sign-in links the provisioned account by e-mail")
}

func TestScim_TakesOverAnExistingAccount(t *testing.T) {
	f := newScimFixture(t)
	f.activate(t, false)
	local := f.newPasswordUser(t, "scim-local", false)

	taken := f.create(t, newScimUserBody(strings.ToUpper(local.email), "ext-"+local.email))

	assert.Equal(t, fmt.Sprint(local.idUser), taken.Id)
	assert.True(t, adminUser(t, f.app, f.adminToken, local.idUser).IsScimManaged)
}

func TestScim_RejectsConflictingCreates(t *testing.T) {
	f := newScimFixture(t)
	f.activate(t, false)
	breakGlass := f.newBreakGlass(t)
	other := uniqueSsoEmail("scim-other")
	t.Cleanup(func() { deleteUsers(f.app, other) })
	f.create(t, newScimUserBody(other, "ext-first"))

	outside := f.scim("POST", "/Users", newScimUserBody("someone@elsewhere.test", "ext-x"))
	assert.Equal(t, http.StatusBadRequest, outside.StatusCode)
	assert.Equal(t, "invalidValue", decodeScimError(t, outside).ScimType)

	for _, body := range []string{
		newScimUserBody(breakGlass.email, "ext-bg"),
		newScimUserBody(other, "ext-second"),
	} {
		res := f.scim("POST", "/Users", body)
		assert.Equal(t, http.StatusConflict, res.StatusCode)
		assert.Equal(t, "uniqueness", decodeScimError(t, res).ScimType)
	}
	assert.Equal(t, http.StatusNotFound, f.scim("GET", fmt.Sprintf("/Users/%d", breakGlass.idUser), "").StatusCode,
		"break-glass accounts are invisible to SCIM")
}

func TestScim_DeactivationEndsSessionsAndApiKeysOfASoleOwner(t *testing.T) {
	f := newScimFixture(t)
	f.activate(t, false)
	owner := f.newPasswordUser(t, "scim-owner", false)
	idProject := createProject(t, f.app, owner.token, "scim-sole-owner")
	t.Cleanup(func() {
		_, _ = f.app.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", idProject)
	})
	keyRes := Request(t, f.app, "POST", "/api/private/user/api-key", `{"name":"laptop"}`, owner.token)
	require.Equal(t, http.StatusCreated, keyRes.StatusCode)
	var key model.CreateApiKeyRes
	require.NoError(t, json.NewDecoder(keyRes.Body).Decode(&key))
	require.Equal(t, http.StatusOK, sessionStatus(t, f.ssoFixture, key.RawKey))

	res := f.deactivate(fmt.Sprint(owner.idUser), `"False"`)

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sessionStatus(t, f.ssoFixture, owner.token))
	assert.Equal(t, http.StatusUnauthorized, sessionStatus(t, f.ssoFixture, key.RawKey))

	reactivated := f.deactivate(fmt.Sprint(owner.idUser), `true`)
	require.Equal(t, http.StatusOK, reactivated.StatusCode)
	assert.True(t, decodeScimUser(t, reactivated).Active)
}

func TestScim_DeleteDeactivatesAndHides(t *testing.T) {
	f := newScimFixture(t)
	address := uniqueSsoEmail("scim-delete")
	t.Cleanup(func() { deleteUsers(f.app, address) })
	created := f.create(t, newScimUserBody(address, "ext-"+address))

	assert.Equal(t, http.StatusNoContent, f.scim("DELETE", "/Users/"+created.Id, "").StatusCode)

	assert.Equal(t, http.StatusNotFound, f.scim("GET", "/Users/"+created.Id, "").StatusCode)
	var idUser int64
	_, err := fmt.Sscan(created.Id, &idUser)
	require.NoError(t, err)
	assert.False(t, adminUser(t, f.app, f.adminToken, idUser).IsActive, "the account and its data stay")

	again := f.create(t, newScimUserBody(address, "ext-"+address))
	assert.Equal(t, created.Id, again.Id, "provisioning the e-mail again brings the account back")
	assert.True(t, again.Active)
}

func TestScim_UpdatesProfileWithinTheRules(t *testing.T) {
	f := newScimFixture(t)
	address := uniqueSsoEmail("scim-edit")
	taken := uniqueSsoEmail("scim-taken")
	renamed := uniqueSsoEmail("scim-renamed")
	t.Cleanup(func() { deleteUsers(f.app, address, taken, renamed) })
	created := f.create(t, newScimUserBody(address, "ext-"+address))
	f.create(t, newScimUserBody(taken, "ext-"+taken))

	patchEmail := func(value string) *http.Response {
		return f.scim("PATCH", "/Users/"+created.Id, fmt.Sprintf(`{"Operations":[{"op":"replace","path":"emails[type eq \"work\"].value","value":%q}]}`, value))
	}
	assert.Equal(t, http.StatusBadRequest, patchEmail("jana@elsewhere.test").StatusCode)
	assert.Equal(t, http.StatusConflict, patchEmail(taken).StatusCode)

	res := f.scim("PUT", "/Users/"+created.Id, fmt.Sprintf(`{"userName":%q,"name":{"givenName":"Jana","familyName":"Nová"},"active":true}`, renamed))
	require.Equal(t, http.StatusOK, res.StatusCode)
	updated := decodeScimUser(t, res)
	assert.Equal(t, renamed, updated.UserName)
	assert.Equal(t, "Jana Nová", updated.DisplayName)
}

func TestScim_PatchOfOneNamePartKeepsTheName(t *testing.T) {
	f := newScimFixture(t)
	address := uniqueSsoEmail("scim-name-part")
	t.Cleanup(func() { deleteUsers(f.app, address) })
	created := f.create(t, fmt.Sprintf(`{"externalId":%q,"userName":%q,"active":true,"name":{"givenName":"Jana","familyName":"Nova"}}`, "ext-"+address, address))
	require.Equal(t, "Jana Nova", created.DisplayName)

	patchName := func(operations string) scimUser {
		res := f.scim("PATCH", "/Users/"+created.Id, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[`+operations+`]}`)
		require.Equal(t, http.StatusOK, res.StatusCode)
		return decodeScimUser(t, res)
	}
	onlyFamilyName := patchName(`{"op":"replace","path":"name.familyName","value":"Novak"}`)
	assert.Equal(t, "Jana Nova", onlyFamilyName.DisplayName, "the given name is not stored on its own, so one part cannot replace the name")

	bothParts := patchName(`{"op":"replace","path":"name.givenName","value":"Jana"},{"op":"replace","path":"name.familyName","value":"Novak"}`)
	assert.Equal(t, "Jana Novak", bothParts.DisplayName)
}

func TestScim_TheLastAdminStays(t *testing.T) {
	f := newScimFixture(t)
	idAdmin := idOfTestUser(t, f.app)
	_, err := f.app.Pool.Exec(context.Background(), "UPDATE users.user SET is_admin = FALSE WHERE is_admin AND id_user <> $1", idAdmin)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.app.Pool.Exec(context.Background(), "UPDATE users.user SET is_active = TRUE WHERE id_user = $1", idAdmin)
	})

	res := f.deactivate(fmt.Sprint(idAdmin), `false`)

	assert.Equal(t, http.StatusConflict, res.StatusCode)
	decodeScimError(t, res)
}

func TestScim_WithoutALicenseOnlyDeactivationWorks(t *testing.T) {
	f := newScimFixture(t)
	address := uniqueSsoEmail("scim-expired")
	deleted := uniqueSsoEmail("scim-expired-delete")
	t.Cleanup(func() { deleteUsers(f.app, address, deleted) })
	created := f.create(t, newScimUserBody(address, "ext-"+address))
	toDelete := f.create(t, newScimUserBody(deleted, "ext-"+deleted))
	f.app = rebuildWithLicense(t, license.Free{})

	assert.Equal(t, http.StatusForbidden, f.scim("GET", "/Users/"+created.Id, "").StatusCode)
	assert.Equal(t, http.StatusForbidden, f.scim("POST", "/Users", newScimUserBody(uniqueSsoEmail("x"), "ext-x")).StatusCode)
	rename := `{"Operations":[{"op":"replace","path":"displayName","value":"Renamed"}]}`
	assert.Equal(t, http.StatusForbidden, f.scim("PATCH", "/Users/"+created.Id, rename).StatusCode)

	res := f.scim("PATCH", "/Users/"+created.Id, `{"Operations":[
		{"op":"replace","path":"displayName","value":"Renamed"},
		{"op":"replace","path":"active","value":false}]}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	user := decodeScimUser(t, res)
	assert.False(t, user.Active)
	assert.Equal(t, "Scim Person", user.DisplayName, "other changes wait for a license")

	assert.Equal(t, http.StatusNoContent, f.scim("DELETE", "/Users/"+toDelete.Id, "").StatusCode)
}

func TestScimAdmin_TokenAndSwitch(t *testing.T) {
	f := newScimFixture(t)
	state := f.scimState(t)
	assert.True(t, state.IsEnabled)
	assert.True(t, state.HasToken)
	assert.Nil(t, state.LastUsedAt)
	assert.Equal(t, ssoBaseUrl+"/api/scim/v2", state.BaseUrl)

	f.scim("GET", "/Users", "")
	assert.NotNil(t, f.scimState(t).LastUsedAt)

	f.app = rebuildWithLicense(t, license.Free{})
	assert.Equal(t, http.StatusForbidden, f.admin(t, "POST", "/scim/token", "").StatusCode)
	assert.Equal(t, http.StatusOK, f.admin(t, "DELETE", "/scim/token", "").StatusCode)
	assert.False(t, f.scimState(t).HasToken)
	assert.Equal(t, http.StatusForbidden, f.admin(t, "PUT", "/scim", `{"isEnabled":true}`).StatusCode)
	assert.Equal(t, http.StatusOK, f.admin(t, "PUT", "/scim", `{"isEnabled":false}`).StatusCode)
	assert.False(t, f.scimState(t).IsEnabled)
}

type scimAdminState struct {
	IsEnabled  bool       `json:"isEnabled"`
	HasToken   bool       `json:"hasToken"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	BaseUrl    string     `json:"baseUrl"`
}

func (f *ssoFixture) scimState(t *testing.T) scimAdminState {
	res := f.admin(t, "GET", "", "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var state struct {
		Scim scimAdminState `json:"scim"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&state))
	return state.Scim
}

func TestSsoSession_StaysLongOnlyWithScim(t *testing.T) {
	f := newScimFixture(t)
	f.activate(t, true)
	cache, err := injector.GetCache()
	require.NoError(t, err)
	address := uniqueSsoEmail("long-session")
	claims := ssoClaims("sub-"+address, address)

	long := f.signIn(t, "?extended=true", claims)
	assert.Greater(t, cache.TTL(context.Background(), long.Token).Val(), 25*time.Hour)

	require.Equal(t, http.StatusOK, f.admin(t, "DELETE", "/scim/token", "").StatusCode)
	short := f.signIn(t, "?extended=true", claims)
	assert.LessOrEqual(t, cache.TTL(context.Background(), short.Token).Val(), 24*time.Hour)
}
