package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/ssotest"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *ssoFixture) createTeam(t *testing.T, name string) int64 {
	res := Request(t, f.app, "POST", "/api/private/admin/team", `{"name":"`+name+`","color":"#00ff00"}`, f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var team struct {
		IdTeam int64 `json:"idTeam"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&team))
	return team.IdTeam
}

func (f *ssoFixture) mapGroup(t *testing.T, groupValue string, idTeam int64) {
	res := f.admin(t, "POST", "/mapping", fmt.Sprintf(`{"groupValue":%q,"idTeam":%d}`, groupValue, idTeam))
	require.Equal(t, http.StatusOK, res.StatusCode, readBody(t, res))
}

func (f *ssoFixture) isTeamMember(t *testing.T, idTeam int64, email string) bool {
	res := Request(t, f.app, "GET", fmt.Sprintf("/api/private/admin/team/%d/member", idTeam), "", f.adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	return strings.Contains(readBody(t, res), email)
}

func (f *ssoFixture) setAdminGroup(t *testing.T, adminGroup string) {
	err := config.NewService(f.app.Pool).UpdateRules(context.Background(), config.UpdateRulesReq{
		AllowedDomains: []string{"acme.test"},
		IsAutoCreate:   true,
		AdminGroup:     adminGroup,
	})
	require.NoError(t, err)
}

func withGroups(claims map[string]any, groups ...string) map[string]any {
	values := make([]any, 0, len(groups))
	for _, group := range groups {
		values = append(values, group)
	}
	claims["groups"] = values
	return claims
}

func TestSsoMapping_IsManagedByTheAdmin(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	idTeam := f.createTeam(t, "mapping-team")

	f.mapGroup(t, " Developers ", idTeam)
	res := f.admin(t, "GET", "/mapping", "")
	require.Equal(t, http.StatusOK, res.StatusCode)
	var mappings []struct {
		IdSsoGroupMapping int64  `json:"idSsoGroupMapping"`
		GroupValue        string `json:"groupValue"`
		IdTeam            int64  `json:"idTeam"`
		TeamName          string `json:"teamName"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&mappings))
	require.Len(t, mappings, 1)
	assert.Equal(t, "Developers", mappings[0].GroupValue)
	assert.Equal(t, "mapping-team", mappings[0].TeamName)

	duplicate := f.admin(t, "POST", "/mapping", fmt.Sprintf(`{"groupValue":"Developers","idTeam":%d}`, idTeam))
	assert.Equal(t, http.StatusConflict, duplicate.StatusCode)
	unknownTeam := f.admin(t, "POST", "/mapping", `{"groupValue":"x","idTeam":999999}`)
	assert.Equal(t, http.StatusUnprocessableEntity, unknownTeam.StatusCode)
	assert.Contains(t, readBody(t, unknownTeam), "SSO_TEAM_NOT_FOUND")

	deleted := f.admin(t, "DELETE", fmt.Sprintf("/mapping/%d", mappings[0].IdSsoGroupMapping), "")
	require.Equal(t, http.StatusOK, deleted.StatusCode)
	assert.JSONEq(t, `[]`, readBody(t, deleted))
	assert.Equal(t, http.StatusNotFound, f.admin(t, "DELETE", fmt.Sprintf("/mapping/%d", mappings[0].IdSsoGroupMapping), "").StatusCode)
}

func TestSsoMapping_WithoutALicenseIsReadOnly(t *testing.T) {
	f := newSsoFixture(t, license.Free{}, ssotest.Options{})

	assert.Equal(t, http.StatusOK, f.admin(t, "GET", "/mapping", "").StatusCode)
	assert.Equal(t, http.StatusForbidden, f.admin(t, "POST", "/mapping", `{"groupValue":"x","idTeam":1}`).StatusCode)
	assert.Equal(t, http.StatusForbidden, f.admin(t, "DELETE", "/mapping/1", "").StatusCode)
}

func TestSsoGroupSync_ManagesOnlyMappedTeams(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	dev, ops, manual := f.createTeam(t, "sync-dev"), f.createTeam(t, "sync-ops"), f.createTeam(t, "sync-manual")
	f.mapGroup(t, "dev", dev)
	f.mapGroup(t, "ops", ops)
	email := uniqueSsoEmail("sync")
	subject := "sub-" + email

	f.signIn(t, "", withGroups(ssoClaims(subject, email), "dev"))
	assert.True(t, f.isTeamMember(t, dev, email))
	assert.False(t, f.isTeamMember(t, ops, email))

	idUser := idOfUser(t, f.app, f.adminToken, email)
	added := Request(t, f.app, "POST", "/api/private/admin/team/member", fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, manual, idUser), f.adminToken)
	require.Equal(t, http.StatusOK, added.StatusCode)

	f.signIn(t, "", withGroups(ssoClaims(subject, email), "ops"))
	assert.False(t, f.isTeamMember(t, dev, email))
	assert.True(t, f.isTeamMember(t, ops, email))
	assert.True(t, f.isTeamMember(t, manual, email))

	f.signIn(t, "", withGroups(ssoClaims(subject, email)))
	assert.False(t, f.isTeamMember(t, ops, email), "an empty list removes mapped memberships")
	assert.True(t, f.isTeamMember(t, manual, email))
}

func TestSsoGroupSync_MissingClaimAfterEarlierGroupsMeansNoGroups(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	f.setAdminGroup(t, "rurdesk-admins")
	dev, manual := f.createTeam(t, "left-dev"), f.createTeam(t, "left-manual")
	f.mapGroup(t, "dev", dev)
	email := uniqueSsoEmail("left")
	subject := "sub-" + email

	f.signIn(t, "", withGroups(ssoClaims(subject, email), "dev", "rurdesk-admins"))
	require.True(t, f.isTeamMember(t, dev, email))
	idUser := idOfUser(t, f.app, f.adminToken, email)
	added := Request(t, f.app, "POST", "/api/private/admin/team/member", fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, manual, idUser), f.adminToken)
	require.Equal(t, http.StatusOK, added.StatusCode)

	session := f.signIn(t, "", ssoClaims(subject, email))

	assert.False(t, f.isTeamMember(t, dev, email), "the IdP leaves the claim out once the person is in no group")
	assert.True(t, f.isTeamMember(t, manual, email))
	assert.Contains(t, readBody(t, Request(t, f.app, "GET", "/api/private/user", "", session.Token)), `"isAdmin":false`)
}

func TestSsoGroupSync_SkipsWhenTheIdpNeverSentGroups(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	dev := f.createTeam(t, "never-dev")
	f.mapGroup(t, "dev", dev)
	email := uniqueSsoEmail("never")
	subject := "sub-" + email

	f.signIn(t, "", ssoClaims(subject, email))
	idUser := idOfUser(t, f.app, f.adminToken, email)
	added := Request(t, f.app, "POST", "/api/private/admin/team/member", fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, dev, idUser), f.adminToken)
	require.Equal(t, http.StatusOK, added.StatusCode)

	f.signIn(t, "", ssoClaims(subject, email))

	assert.True(t, f.isTeamMember(t, dev, email), "an IdP that never sent groups may be misconfigured")
}

func TestSsoGroupSync_SkipsOnGroupsOverage(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	dev := f.createTeam(t, "overage-dev")
	f.mapGroup(t, "dev", dev)
	email := uniqueSsoEmail("overage")
	subject := "sub-" + email

	f.signIn(t, "", withGroups(ssoClaims(subject, email), "dev"))
	overage := ssoClaims(subject, email)
	overage["_claim_names"] = map[string]any{"groups": "src1"}
	overage["_claim_sources"] = map[string]any{"src1": map[string]any{"endpoint": "https://graph.example.test/getMemberObjects"}}
	f.signIn(t, "", overage)

	assert.True(t, f.isTeamMember(t, dev, email), "too many groups for the token says nothing about membership")
}

func TestSsoGroupSync_ChangesProjectAccess(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	idTeam := f.createTeam(t, "sync-access")
	f.mapGroup(t, "readers", idTeam)
	idProject := createProject(t, f.app, f.adminToken, "sso-sync-project")
	addTeam := Request(t, f.app, "POST", fmt.Sprintf("/api/private/project/%d/member/team", idProject), fmt.Sprintf(`{"idTeam":%d,"role":"viewer"}`, idTeam), f.adminToken)
	require.Equal(t, http.StatusOK, addTeam.StatusCode, readBody(t, addTeam))
	email := uniqueSsoEmail("access")
	subject := "sub-" + email
	userRole := fmt.Sprintf("/api/private/project/%d/user-role", idProject)

	first := f.signIn(t, "", withGroups(ssoClaims(subject, email), "readers"))
	assert.Equal(t, http.StatusOK, Request(t, f.app, "GET", userRole, "", first.Token).StatusCode)

	f.signIn(t, "", withGroups(ssoClaims(subject, email)))
	assert.Equal(t, http.StatusForbidden, Request(t, f.app, "GET", userRole, "", first.Token).StatusCode)
}

func TestSsoGroupSync_AdminGroupGrantsAndRevokesAdmin(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	f.setAdminGroup(t, "rurdesk-admins")
	email := uniqueSsoEmail("admin")
	subject := "sub-" + email
	isAdmin := func(token string) bool {
		res := Request(t, f.app, "GET", "/api/private/user", "", token)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var user struct {
			IsAdmin bool `json:"isAdmin"`
		}
		require.NoError(t, json.NewDecoder(res.Body).Decode(&user))
		return user.IsAdmin
	}

	first := f.signIn(t, "", withGroups(ssoClaims(subject, email), "rurdesk-admins"))
	assert.True(t, isAdmin(first.Token))

	second := f.signIn(t, "", withGroups(ssoClaims(subject, email), "dev"))
	assert.False(t, isAdmin(second.Token))
	assert.Equal(t, http.StatusUnauthorized, Request(t, f.app, "GET", "/api/private/user", "", first.Token).StatusCode)
}

func TestSsoGroupSync_WithoutAnAdminGroupKeepsTheFlag(t *testing.T) {
	f := newSsoFixture(t, ssoLicense, ssotest.Options{})
	f.activate(t, true)
	email := uniqueSsoEmail("keep")
	t.Cleanup(func() { deleteUsers(f.app, email) })
	subject := "sub-" + email
	f.signIn(t, "", withGroups(ssoClaims(subject, email)))
	idUser := idOfUser(t, f.app, f.adminToken, email)
	promote := Request(t, f.app, "PATCH", fmt.Sprintf("/api/private/admin/user/%d", idUser), `{"isAdmin":true}`, f.adminToken)
	require.Equal(t, http.StatusOK, promote.StatusCode)

	session := f.signIn(t, "", withGroups(ssoClaims(subject, email)))

	assert.Contains(t, readBody(t, Request(t, f.app, "GET", "/api/private/user", "", session.Token)), `"isAdmin":true`)
}
