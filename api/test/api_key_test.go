package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// ApiKeySuite covers admin-on-bot API key management. A bot has at most one key
// (mirrors the 1:1 gateway model): it is created with the bot, then rotated in place
// via .../api-key/token or deleted via DELETE .../api-key. Keys are bot-only — there
// is no self-service human key endpoint.
type ApiKeySuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
	BotID      int64
}

func (s *ApiKeySuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)
	s.BotID, _ = s.newBot("keybot")
}

func (s *ApiKeySuite) keyURL(idUser int64) string {
	return fmt.Sprintf("/api/private/admin/user/%d/api-key", idUser)
}

// newBot creates a bot (which mints a "default" key) and returns its id and the
// one-time raw key. The bot is cleaned up when the test finishes.
func (s *ApiKeySuite) newBot(name string) (int64, string) {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isBot":true}`, name), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var bot model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&bot))
	id := bot.IdUser
	s.T().Cleanup(func() {
		s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", id)
	})
	return id, bot.RawKey
}

func (s *ApiKeySuite) Test_Get_ReturnsSingleKey() {
	res := Request(s.T(), s.App, "GET", s.keyURL(s.BotID), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var key model.ApiKey
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&key))
	s.Equal("default", key.Name)
	s.Greater(key.IdApiKey, int64(0))
}

func (s *ApiKeySuite) Test_Create_Conflict_WhenKeyExists() {
	// s.BotID already owns the "default" key minted at creation.
	res := Request(s.T(), s.App, "POST", s.keyURL(s.BotID), `{"name":"second"}`, s.AdminToken)
	s.Equal(http.StatusConflict, res.StatusCode)
}

func (s *ApiKeySuite) Test_Regenerate_RotatesKey() {
	id, oldRaw := s.newBot("regenbot")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", oldRaw).StatusCode)

	res := Request(s.T(), s.App, "POST", s.keyURL(id)+"/token", "{}", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var out model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&out))
	s.NotEmpty(out.RawKey)
	s.NotEqual(oldRaw, out.RawKey)

	// Old key stops working immediately; the new one authenticates.
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", oldRaw).StatusCode)
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", out.RawKey).StatusCode)
}

func (s *ApiKeySuite) Test_Regenerate_NotFound_WhenNoKey() {
	id, _ := s.newBot("noregenbot")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "DELETE", s.keyURL(id), "", s.AdminToken).StatusCode)

	res := Request(s.T(), s.App, "POST", s.keyURL(id)+"/token", "{}", s.AdminToken)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func (s *ApiKeySuite) Test_Revoke_BlocksAuth_AndClearsKey() {
	id, raw := s.newBot("revokebot")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", raw).StatusCode)

	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "DELETE", s.keyURL(id), "", s.AdminToken).StatusCode)

	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", raw).StatusCode)

	getRes := Request(s.T(), s.App, "GET", s.keyURL(id), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, getRes.StatusCode)
	body, _ := io.ReadAll(getRes.Body)
	s.Equal("null", strings.TrimSpace(string(body)))
}

func (s *ApiKeySuite) Test_Create_AfterRevoke() {
	id, _ := s.newBot("recreatebot")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "DELETE", s.keyURL(id), "", s.AdminToken).StatusCode)

	res := Request(s.T(), s.App, "POST", s.keyURL(id), `{"name":"fresh"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var out model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&out))
	s.NotEmpty(out.RawKey)
	s.Equal("fresh", out.Name)

	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", out.RawKey).StatusCode)
}

func (s *ApiKeySuite) Test_RawKeyAuthenticates() {
	id, raw := s.newBot("authbot")
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", raw)
	s.Require().Equal(http.StatusOK, userRes.StatusCode)
	var u model.User
	json.NewDecoder(userRes.Body).Decode(&u)
	s.Equal(id, u.IdUser)
}

func (s *ApiKeySuite) Test_RateLimit_Override() {
	id, _ := s.newBot("ratelimitbot")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "DELETE", s.keyURL(id), "", s.AdminToken).StatusCode)

	res := Request(s.T(), s.App, "POST", s.keyURL(id),
		`{"name":"rl","rateLimitOverride":3}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created model.CreateApiKeyRes
	json.NewDecoder(res.Body).Decode(&created)

	for i := 0; i < 3; i++ {
		s.Require().Equal(http.StatusOK,
			Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode,
			"request %d should succeed", i+1)
	}
	s.Equal(http.StatusTooManyRequests,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode,
		"4th request should be rate-limited")
}

// a key that authenticated while valid (populating the
// 5-min Redis session cache) kept authenticating for the whole TTL even after
// expires_at passed, because the cached snapshot ignored expiry. The cache TTL
// must now be capped to the key's remaining lifetime, so the key stops
// authenticating once it expires — not up to 5 minutes later.
func (s *ApiKeySuite) Test_ExpiredKey_StopsAuth_EvenAfterCaching() {
	id, raw := s.newBot("expirebot")

	// Give the key a short lifetime (expires_at is UTC wall-clock, matching the
	// LoadByHash comparison against NOW() AT TIME ZONE 'utc').
	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE users.api_key SET expires_at = (NOW() AT TIME ZONE 'utc') + interval '2 seconds' WHERE id_user = $1", id)
	s.Require().NoError(err)

	// Authenticate while still valid — this caches the session snapshot.
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", raw).StatusCode)

	// Wait past expiry. Before the fix the cached snapshot would keep working;
	// with the TTL cap the entry is gone and the DB re-check rejects it.
	time.Sleep(2500 * time.Millisecond)

	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", raw).StatusCode)
}

// rateLimitOverride=0 used to be accepted and then
// locked the bot out permanently (first request already exceeded the 0 quota →
// 429 forever). Non-positive overrides must now be rejected at the boundary;
// omitting the field still falls back to the default limit.
func (s *ApiKeySuite) Test_Create_RejectsNonPositiveRateLimit() {
	id, _ := s.newBot("rlzerobot")
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "DELETE", s.keyURL(id), "", s.AdminToken).StatusCode)

	s.Equal(http.StatusBadRequest,
		Request(s.T(), s.App, "POST", s.keyURL(id), `{"name":"rl","rateLimitOverride":0}`, s.AdminToken).StatusCode,
		"rateLimitOverride 0 must be rejected")
	s.Equal(http.StatusBadRequest,
		Request(s.T(), s.App, "POST", s.keyURL(id), `{"name":"rl","rateLimitOverride":-5}`, s.AdminToken).StatusCode,
		"negative rateLimitOverride must be rejected")

	// Omitting the override still works (falls back to the default limit).
	res := Request(s.T(), s.App, "POST", s.keyURL(id), `{"name":"rl"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created model.CreateApiKeyRes
	json.NewDecoder(res.Body).Decode(&created)
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", created.RawKey).StatusCode)
}

func (s *ApiKeySuite) Test_SelfServiceCreate_Removed() {
	res := Request(s.T(), s.App, "POST", "/api/private/api-key", `{"name":"x"}`, s.AdminToken)
	s.Equal(http.StatusNotFound, res.StatusCode, "self-service key route must be gone")
}

func (s *ApiKeySuite) Test_KeyEndpoints_RejectHumanTarget() {
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
	res := Request(s.T(), s.App, "POST", s.keyURL(adminID), `{"name":"x"}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *ApiKeySuite) Test_BotCannotBeOwner() {
	bot, _ := s.newBot("ownerbot")

	idProject := createProject(s.T(), s.App, s.AdminToken, "bot-owner-test-prj")

	// Add bot as member — must succeed.
	addRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, bot), s.AdminToken)
	s.Require().Equal(http.StatusOK, addRes.StatusCode)

	// Promote bot to owner — must be rejected.
	promoteRes := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject, bot),
		`{"role":"owner"}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, promoteRes.StatusCode)

	// Direct add as owner — also rejected.
	Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject, bot),
		"", s.AdminToken)
	addOwnerRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, bot), s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, addOwnerRes.StatusCode)
}

func Test_RunApiKeySuite(t *testing.T) {
	suite.Run(t, new(ApiKeySuite))
}
