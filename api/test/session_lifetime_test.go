package test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/suite"
)

const (
	sessionUserEmail    = "session-lifetime@test.sk"
	sessionUserPassword = "kreslo"
)

type SessionLifetimeSuite struct {
	suite.Suite
	App *issue.Application
}

func (s *SessionLifetimeSuite) SetupSuite() {
	s.App = Setup(s.T())
	adminToken := Token(s.T(), s.App)
	createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"session tester","email":"`+sessionUserEmail+`","password":"`+sessionUserPassword+`"}`)
}

func (s *SessionLifetimeSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = $1", sessionUserEmail)
}

func (s *SessionLifetimeSuite) login(hasExtendedSessionLifetime bool) string {
	body := `{"email":"` + sessionUserEmail + `","password":"` + sessionUserPassword + `"}`
	if hasExtendedSessionLifetime {
		body = `{"email":"` + sessionUserEmail + `","password":"` + sessionUserPassword +
			`","hasExtendedSessionLifetime":true}`
	}
	res := Request(s.T(), s.App, "POST", "/api/public/login", body, "")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var tk struct {
		Token string `json:"token"`
	}
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&tk))
	s.Require().NotEmpty(tk.Token)
	return tk.Token
}

func (s *SessionLifetimeSuite) ttl(key string) float64 {
	cache, err := injector.GetCache()
	s.Require().Nil(err)
	ttl, err := cache.TTL(context.Background(), key).Result()
	s.Require().Nil(err)
	return ttl.Seconds()
}

func (s *SessionLifetimeSuite) sessionIndexKey() string {
	var idUser int64
	s.Require().Nil(s.App.Pool.QueryRow(context.Background(),
		"SELECT id_user FROM users.user WHERE email = $1", sessionUserEmail).Scan(&idUser))
	return constants.SessionIndexPrefix + strconv.FormatInt(idUser, 10)
}

func (s *SessionLifetimeSuite) Test_Login_DefaultsToShortSession() {
	ttl := s.ttl(s.login(false))

	s.Greater(ttl, constants.SessionLifetime.Seconds()-60)
	s.LessOrEqual(ttl, constants.SessionLifetime.Seconds())
}

func (s *SessionLifetimeSuite) Test_Login_WithExtendedLifetime_LastsLonger() {
	ttl := s.ttl(s.login(true))

	s.Greater(ttl, constants.SessionLifetime.Seconds(),
		"an extended session must outlive the default one")
	s.Greater(ttl, constants.SessionLifetimeExtended.Seconds()-60)
	s.LessOrEqual(ttl, constants.SessionLifetimeExtended.Seconds())
}

func (s *SessionLifetimeSuite) Test_UpdatingProfile_KeepsExtendedSessionAlive() {
	token := s.login(true)

	res := Request(s.T(), s.App, "PATCH", "/api/private/user", `{"name":"renamed tester"}`, token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Greater(s.ttl(token), constants.SessionLifetime.Seconds(),
		"updating the profile must not shorten an extended session")
}

func (s *SessionLifetimeSuite) Test_ShortLogin_KeepsSessionIndexAliveForExtendedSession() {
	s.login(true)
	s.login(false)

	s.Greater(s.ttl(s.sessionIndexKey()), constants.SessionLifetime.Seconds(),
		"a short login must not shorten the session index that still holds an extended session")
}

func Test_RunSessionLifetimeSuite(t *testing.T) {
	suite.Run(t, new(SessionLifetimeSuite))
}
