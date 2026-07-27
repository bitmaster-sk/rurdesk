package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/suite"
)

type PasswordChangeSuite struct {
	suite.Suite
	App   *issue.Application
	Token string
}

func (s *PasswordChangeSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = createUserAsAdmin(s.T(), s.App, Token(s.T(), s.App),
		`{"name":"pwduser","email":"pwd@test.sk","password":"kreslo"}`)
}

func (s *PasswordChangeSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'pwd@test.sk'")
}

func (s *PasswordChangeSuite) Test_01_WrongCurrentPassword_Forbidden() {
	res := Request(s.T(), s.App, "PUT", "/api/private/user/password",
		`{"currentPassword":"wrong","newPassword":"stolicka"}`, s.Token)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *PasswordChangeSuite) Test_02_ChangePassword_LoginWithNew() {
	res := Request(s.T(), s.App, "PUT", "/api/private/user/password",
		`{"currentPassword":"kreslo","newPassword":"stolicka"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	oldLogin := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"pwd@test.sk","password":"kreslo"}`, "")
	s.Equal(http.StatusUnauthorized, oldLogin.StatusCode)

	newLogin := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"pwd@test.sk","password":"stolicka"}`, "")
	s.Equal(http.StatusOK, newLogin.StatusCode)
}

// changing the password left every previously issued
// session token valid for its full 24h TTL. Other sessions must be booted; the
// acting session (keepToken) survives.
func (s *PasswordChangeSuite) Test_03_ChangePassword_InvalidatesOtherSessions() {
	staleToken := createUserAsAdmin(s.T(), s.App, Token(s.T(), s.App),
		`{"name":"pwd2user","email":"pwd2@test.sk","password":"kreslo"}`)
	s.T().Cleanup(func() {
		s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'pwd2@test.sk'")
	})

	// A second, independent login — this is the session that changes the password.
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"pwd2@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, loginRes.StatusCode)
	var tk struct {
		Token string `json:"token"`
	}
	s.Require().NoError(json.NewDecoder(loginRes.Body).Decode(&tk))
	keepToken := tk.Token

	// Both sessions authenticate before the change.
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", staleToken).StatusCode)

	// Change the password from the acting (keep) session.
	res := Request(s.T(), s.App, "PUT", "/api/private/user/password",
		`{"currentPassword":"kreslo","newPassword":"stolicka"}`, keepToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	// The other pre-change session is booted...
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", staleToken).StatusCode,
		"stale session must be invalidated after password change")

	// ...while the acting session survives its own change.
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", keepToken).StatusCode,
		"acting session must survive its own password change")
}

func Test_RunPasswordChangeSuite(t *testing.T) {
	suite.Run(t, new(PasswordChangeSuite))
}
