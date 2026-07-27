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

// AdminAclSuite verifies the global-admin bypass: the bootstrap admin
// (test@test.sk) has owner rights on a project it was never added to.
type AdminAclSuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
	OwnerToken string
	ProjectID  int64
}

func (s *AdminAclSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)
	s.OwnerToken = createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"aclowner","email":"aclowner@test.sk","password":"kreslo"}`)
	s.ProjectID = createProject(s.T(), s.App, s.OwnerToken, "admin-acl-project")
}

func (s *AdminAclSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE name = 'admin-acl-project'")
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'aclowner@test.sk'")
}

func (s *AdminAclSuite) Test_01_Admin_HasOwnerRole_OnForeignProject() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var body model.UserRoleRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Equal(model.RoleOwner, body.Role)
}

func (s *AdminAclSuite) Test_02_Admin_CanReadMembers_OfForeignProject() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/member", s.ProjectID), "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *AdminAclSuite) Test_03_Admin_SeesAllProjects() {
	res := Request(s.T(), s.App, "GET", "/api/private/project", "", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var projects []*model.Project
	json.NewDecoder(res.Body).Decode(&projects)
	found := false
	for _, p := range projects {
		if p.IdProject == s.ProjectID {
			found = true
		}
	}
	s.True(found, "admin must see projects they are not a member of")
}

func (s *AdminAclSuite) Test_04_NonAdmin_StillForbidden() {
	stranger := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"stranger","email":"aclstranger@test.sk","password":"kreslo"}`)
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'aclstranger@test.sk'")
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", stranger)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func Test_RunAdminAclSuite(t *testing.T) {
	suite.Run(t, new(AdminAclSuite))
}
