package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ssoIdentityRes struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

func (f *ssoFixture) identities(t *testing.T, idUser int64) []ssoIdentityRes {
	res := f.admin(t, "GET", fmt.Sprintf("/identity/%d", idUser), "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var identities []ssoIdentityRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&identities))
	return identities
}

func (f *ssoFixture) hasSsoIdentity(t *testing.T, idUser int64) bool {
	res := Request(t, f.app, "GET", "/api/private/admin/user", "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var users []struct {
		IdUser         int64 `json:"idUser"`
		HasSsoIdentity bool  `json:"hasSsoIdentity"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&users))
	for _, user := range users {
		if user.IdUser == idUser {
			return user.HasSsoIdentity
		}
	}
	t.Fatalf("user %d not found", idUser)
	return false
}

func (f *ssoFixture) ssoUser(t *testing.T, prefix string) (string, int64, ssoSession) {
	email := uniqueSsoEmail(prefix)
	session := f.signIn(t, "", ssoClaims("sub-"+email, email))
	return email, idOfUser(t, f.app, f.adminToken, email), session
}

func TestSsoIdentity_IsShownAndCanBeUnlinked(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email, idUser, _ := f.ssoUser(t, "unlink")

	identities := f.identities(t, idUser)
	require.Len(t, identities, 1)
	assert.Equal(t, f.idp.Issuer(), identities[0].Issuer)
	assert.Equal(t, "sub-"+email, identities[0].Subject)
	assert.True(t, f.hasSsoIdentity(t, idUser))

	unlinked := f.admin(t, "DELETE", fmt.Sprintf("/identity/%d", idUser), "")
	require.Equal(t, http.StatusOK, unlinked.StatusCode)
	assert.JSONEq(t, `[]`, readBody(t, unlinked))
	assert.False(t, f.hasSsoIdentity(t, idUser))
	assert.Equal(t, http.StatusNotFound, f.admin(t, "DELETE", fmt.Sprintf("/identity/%d", idUser), "").StatusCode)
}

func TestSsoIdentity_MoveDeactivatesTheSourceAccount(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	sourceEmail, idSource, sourceSession := f.ssoUser(t, "move-source")
	targetEmail := uniqueSsoEmail("move-target")
	createUserAsAdmin(t, f.app, f.adminToken, `{"name":"Cieľové konto","email":"`+targetEmail+`","password":"kreslo123"}`)
	idTarget := idOfUser(t, f.app, f.adminToken, targetEmail)

	moved := f.admin(t, "POST", fmt.Sprintf("/identity/%d/move", idSource), fmt.Sprintf(`{"idTargetUser":%d}`, idTarget))

	require.Equal(t, http.StatusOK, moved.StatusCode, readBody(t, moved))
	assert.Empty(t, f.identities(t, idSource))
	assert.Len(t, f.identities(t, idTarget), 1)
	assert.Equal(t, http.StatusUnauthorized, Request(t, f.app, "GET", "/api/private/user", "", sourceSession.Token).StatusCode)
	session := f.signIn(t, "", ssoClaims("sub-"+sourceEmail, sourceEmail))
	assert.Contains(t, readBody(t, Request(t, f.app, "GET", "/api/private/user", "", session.Token)), targetEmail)
}

func TestSsoIdentity_MoveRejectsAnInvalidTarget(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	_, idSource, _ := f.ssoUser(t, "reject-source")
	_, idLinked, _ := f.ssoUser(t, "reject-linked")
	move := func(idTarget int64) *http.Response {
		return f.admin(t, "POST", fmt.Sprintf("/identity/%d/move", idSource), fmt.Sprintf(`{"idTargetUser":%d}`, idTarget))
	}

	sameIssuer := move(idLinked)
	assert.Equal(t, http.StatusConflict, sameIssuer.StatusCode)
	assert.Contains(t, readBody(t, sameIssuer), "SSO_IDENTITY_EXISTS")

	itself := move(idSource)
	assert.Equal(t, http.StatusUnprocessableEntity, itself.StatusCode)
	assert.Contains(t, readBody(t, itself), "SSO_IDENTITY_TARGET_INVALID")

	missing := move(999999)
	assert.Equal(t, http.StatusUnprocessableEntity, missing.StatusCode)

	assert.Len(t, f.identities(t, idSource), 1)
}

func TestSsoIdentity_WithoutALicenseIsReadOnly(t *testing.T) {
	f := newSsoFixture(t, license.Free{}, ssotest.Options{})
	idAdmin := idOfTestUser(t, f.app)

	assert.Equal(t, http.StatusOK, f.admin(t, "GET", fmt.Sprintf("/identity/%d", idAdmin), "").StatusCode)
	assert.Equal(t, http.StatusForbidden, f.admin(t, "DELETE", fmt.Sprintf("/identity/%d", idAdmin), "").StatusCode)
	assert.Equal(t, http.StatusForbidden, f.admin(t, "POST", fmt.Sprintf("/identity/%d/move", idAdmin), `{"idTargetUser":1}`).StatusCode)
}
