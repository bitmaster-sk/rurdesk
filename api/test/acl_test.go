package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type AclSuite struct {
	suite.Suite
	App   *issue.Application
	Token string
}

func (s *AclSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
}

func (s *AclSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE TRUE")
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email != 'test@test.sk'")
}

func (s *AclSuite) Test_GetUserRole_DirectOwner() {
	// Create project (creator gets RoleOwner in Task 7)
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"owner1","email":"owner1@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var tkRes struct {
		Token string `json:"token"`
	}
	res2 := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"owner1@test.sk","password":"kreslo"}`, "")
	json.NewDecoder(res2.Body).Decode(&tkRes)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"acl-test-prj","color":"#ff0000"}`, tkRes.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj model.Project
	json.NewDecoder(prjRes.Body).Decode(&prj)

	roleRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", prj.IdProject), "", tkRes.Token)
	s.Equal(http.StatusOK, roleRes.StatusCode)
	var roleBody model.UserRoleRes
	json.NewDecoder(roleRes.Body).Decode(&roleBody)
	s.Equal(model.RoleOwner, roleBody.Role)
}

func Test_RunAclSuite(t *testing.T) {
	suite.Run(t, new(AclSuite))
}
