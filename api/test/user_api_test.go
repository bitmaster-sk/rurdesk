package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type LoginSuite struct {
	suite.Suite
	App   *issue.Application
	Token string
}

func (suite *LoginSuite) SetupSuite() {
	suite.App = Setup(suite.T())
	suite.Token = Token(suite.T(), suite.App)
}

func (suite *LoginSuite) TearDownSuite() {
	suite.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email != 'test@test.sk'")
}

func (suite *LoginSuite) Test_Login() {
	res := Request(suite.T(), suite.App, "GET", "/api/private/user", "", suite.Token)
	suite.Equal(http.StatusOK, res.StatusCode)

	var usr model.User
	err := json.NewDecoder(res.Body).Decode(&usr)
	suite.Nil(err)

	suite.Equal("test@test.sk", usr.Email)
	suite.Equal("tester", usr.Name)
}

func (suite *LoginSuite) Test_Logout() {
	res := Request(suite.T(), suite.App, "GET", "/api/private/user", "", suite.Token)
	suite.Equal(http.StatusOK, res.StatusCode)

	var usr model.User
	err := json.NewDecoder(res.Body).Decode(&usr)
	suite.Nil(err)

	suite.Equal("test@test.sk", usr.Email)
	suite.Equal("tester", usr.Name)

	res = Request(suite.T(), suite.App, "DELETE", "/api/private/logout", "", suite.Token)
	suite.Equal(http.StatusOK, res.StatusCode)

	res = Request(suite.T(), suite.App, "GET", "/api/private/user", "", suite.Token)
	suite.Equal(http.StatusUnauthorized, res.StatusCode)
}

func (suite *LoginSuite) Test_PublicRegister_ClosedAfterBootstrap() {
	// The bootstrap admin (test@test.sk) already exists, so public registration is closed.
	rb := `{"name":"tester2","email":"t2@t.sk","password":"testpwd"}`
	res := Request(suite.T(), suite.App, "POST", "/api/public/register", rb, "")
	suite.Equal(http.StatusForbidden, res.StatusCode,
		"public registration must be closed once the bootstrap admin exists")
}

func (suite *LoginSuite) Test_UserPayload_IncludesIsAdmin() {
	// Fresh token: Test_Logout invalidates suite.Token, and suite methods run alphabetically.
	token := Token(suite.T(), suite.App)
	res := Request(suite.T(), suite.App, "GET", "/api/private/user", "", token)
	suite.Equal(http.StatusOK, res.StatusCode)

	var body map[string]any
	suite.Nil(json.NewDecoder(res.Body).Decode(&body))
	_, hasIsAdmin := body["isAdmin"]
	suite.True(hasIsAdmin, "user payload must include isAdmin so the UI can gate admin features")
}

func Test_RunLoginSuite(t *testing.T) {
	suite.Run(t, new(LoginSuite))
}
