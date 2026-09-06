package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// UserApiKeySuite covers the self-service personal access token endpoints:
// a human mints named tokens, each authenticates as that human, and no user can
// see or touch another user's tokens.
type UserApiKeySuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
	AdminID    int64
}

const userApiKeyURL = "/api/private/user/api-key"

func (s *UserApiKeySuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)
	s.AdminID = idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
}

func (s *UserApiKeySuite) SetupTest() {
	s.setApiKeyLimit(10)
	_, err := s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.api_key WHERE NOT is_agent")
	s.Require().NoError(err)
}

func (s *UserApiKeySuite) TearDownSuite() {
	s.setApiKeyLimit(constants.KnownAppNumericSettings[constants.SettingUserApiKeyLimit].Default)
}

func (s *UserApiKeySuite) setApiKeyLimit(limit int) {
	res := Request(s.T(), s.App, "PATCH", "/api/private/admin/settings",
		fmt.Sprintf(`{"userApiKeyLimit":%d}`, limit), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *UserApiKeySuite) create(token, name string) model.CreateApiKeyRes {
	res := Request(s.T(), s.App, "POST", userApiKeyURL,
		fmt.Sprintf(`{"name":%q}`, name), token)
	s.Require().Equal(http.StatusCreated, res.StatusCode)
	var out model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&out))
	return out
}

func (s *UserApiKeySuite) list(token string) []model.ApiKey {
	res := Request(s.T(), s.App, "GET", userApiKeyURL, "", token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var keys []model.ApiKey
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&keys))
	return keys
}

func (s *UserApiKeySuite) Test_Create_ReturnsRawKey_ThatAuthenticates() {
	created := s.create(s.AdminToken, "laptop")
	s.NotEmpty(created.RawKey)
	s.Equal("laptop", created.Name)

	res := Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var user model.User
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&user))
	s.Equal(s.AdminID, user.IdUser)
}

func (s *UserApiKeySuite) Test_List_ReturnsOwnTokensOnly() {
	s.create(s.AdminToken, "laptop")
	s.create(s.AdminToken, "ci")

	otherToken := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"pat-other","email":"pat-other@test.sk","password":"kreslo"}`)
	s.create(otherToken, "other-laptop")

	names := []string{}
	for _, key := range s.list(s.AdminToken) {
		names = append(names, key.Name)
	}
	s.ElementsMatch([]string{"laptop", "ci"}, names)
}

func (s *UserApiKeySuite) Test_Revoke_BlocksAuthImmediately() {
	created := s.create(s.AdminToken, "revoke-me")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode)

	res := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("%s/%d", userApiKeyURL, created.IdApiKey), "", s.AdminToken)
	s.Require().Equal(http.StatusNoContent, res.StatusCode)

	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode)
	s.Empty(s.list(s.AdminToken))
}

func (s *UserApiKeySuite) Test_Regenerate_RotatesInPlace() {
	created := s.create(s.AdminToken, "rotate-me")

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("%s/%d/token", userApiKeyURL, created.IdApiKey), "{}", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var rotated model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&rotated))
	s.NotEqual(created.RawKey, rotated.RawKey)
	s.Equal(created.IdApiKey, rotated.IdApiKey)

	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode)
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", rotated.RawKey).StatusCode)
}

func (s *UserApiKeySuite) Test_ForeignToken_CannotBeRevokedOrRotated() {
	victim := s.create(s.AdminToken, "victim")
	attackerToken := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"pat-attacker","email":"pat-attacker@test.sk","password":"kreslo"}`)

	s.Equal(http.StatusNotFound, Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("%s/%d", userApiKeyURL, victim.IdApiKey), "", attackerToken).StatusCode)
	s.Equal(http.StatusNotFound, Request(s.T(), s.App, "POST",
		fmt.Sprintf("%s/%d/token", userApiKeyURL, victim.IdApiKey), "{}", attackerToken).StatusCode)

	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", victim.RawKey).StatusCode)
}

func (s *UserApiKeySuite) Test_Create_RejectedAtLimit_AndFreedByRevoke() {
	s.setApiKeyLimit(2)
	first := s.create(s.AdminToken, "one")
	s.create(s.AdminToken, "two")

	s.Equal(http.StatusConflict,
		Request(s.T(), s.App, "POST", userApiKeyURL, `{"name":"three"}`, s.AdminToken).StatusCode)

	s.Require().Equal(http.StatusNoContent, Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("%s/%d", userApiKeyURL, first.IdApiKey), "", s.AdminToken).StatusCode)
	s.create(s.AdminToken, "three")

	// Raising the limit frees a slot without touching existing tokens.
	s.Equal(http.StatusConflict,
		Request(s.T(), s.App, "POST", userApiKeyURL, `{"name":"four"}`, s.AdminToken).StatusCode)
	s.setApiKeyLimit(3)
	s.create(s.AdminToken, "four")
}

func (s *UserApiKeySuite) Test_TokenCannotManageTokens() {
	created := s.create(s.AdminToken, "self-minting")

	s.Equal(http.StatusForbidden,
		Request(s.T(), s.App, "GET", userApiKeyURL, "", created.RawKey).StatusCode)
	s.Equal(http.StatusForbidden,
		Request(s.T(), s.App, "POST", userApiKeyURL, `{"name":"child"}`, created.RawKey).StatusCode)
	s.Equal(http.StatusForbidden, Request(s.T(), s.App, "POST",
		fmt.Sprintf("%s/%d/token", userApiKeyURL, created.IdApiKey), "{}", created.RawKey).StatusCode)
	s.Equal(http.StatusForbidden, Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("%s/%d", userApiKeyURL, created.IdApiKey), "", created.RawKey).StatusCode)
}

func (s *UserApiKeySuite) Test_AgentApiKey_IsNotReachableAsUserApiKey() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"patagent","isBot":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	s.T().Cleanup(func() {
		s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", agent.IdUser)
	})

	keyRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", agent.IdUser), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, keyRes.StatusCode)
	var agentKey model.ApiKey
	s.Require().NoError(json.NewDecoder(keyRes.Body).Decode(&agentKey))

	s.Empty(s.list(s.AdminToken), "an agent key must never surface as a personal token")
	s.Equal(http.StatusNotFound, Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("%s/%d", userApiKeyURL, agentKey.IdApiKey), "", s.AdminToken).StatusCode)
	s.Equal(http.StatusNotFound, Request(s.T(), s.App, "POST",
		fmt.Sprintf("%s/%d/token", userApiKeyURL, agentKey.IdApiKey), "{}", s.AdminToken).StatusCode)

	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode,
		"the agent key must still authenticate")
}

func (s *UserApiKeySuite) Test_UserApiKey_IsNotReachableAsAgentApiKey() {
	created := s.create(s.AdminToken, "not-an-agent-key")

	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", s.AdminID), "", s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)

	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode)
}

func (s *UserApiKeySuite) Test_Create_IgnoresRateLimitOverride() {
	res := Request(s.T(), s.App, "POST", userApiKeyURL,
		`{"name":"greedy","rateLimitOverride":9999}`, s.AdminToken)
	s.Require().Equal(http.StatusCreated, res.StatusCode)
	var created model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	s.Nil(created.RateLimitOverride, "a self-minted token must run on the default limit")
}

func (s *UserApiKeySuite) Test_TokenCannotReachAdminRoutes() {
	created := s.create(s.AdminToken, "admin-probe")

	s.Equal(http.StatusForbidden,
		Request(s.T(), s.App, "GET", "/api/private/admin/user", "", created.RawKey).StatusCode,
		"a token must not grant admin-console access, even for an admin user")
}

func (s *UserApiKeySuite) Test_Create_RequiresName() {
	s.Equal(http.StatusBadRequest,
		Request(s.T(), s.App, "POST", userApiKeyURL, `{}`, s.AdminToken).StatusCode)
}

func (s *UserApiKeySuite) Test_ExpiredToken_DoesNotAuthenticate() {
	res := Request(s.T(), s.App, "POST", userApiKeyURL,
		`{"name":"expired","expiresAt":"2020-01-01T00:00:00Z"}`, s.AdminToken)
	s.Require().Equal(http.StatusCreated, res.StatusCode)
	var created model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))

	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode)
}

func Test_RunUserApiKeySuite(t *testing.T) {
	suite.Run(t, new(UserApiKeySuite))
}
