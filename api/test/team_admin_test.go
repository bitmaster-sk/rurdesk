package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/suite"
)

type TeamAdminSuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
	UserToken  string
	UserID     int64
	TeamID     int64
}

func (s *TeamAdminSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)
	s.UserToken = createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"teamuser","email":"teamuser@test.sk","password":"kreslo"}`)
	s.UserID = idOfUser(s.T(), s.App, s.AdminToken, "teamuser@test.sk")
}

func (s *TeamAdminSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.team WHERE name = 'admin-managed-team'")
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'teamuser@test.sk'")
}

func (s *TeamAdminSuite) Test_01_NonAdmin_CannotCreateTeam() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/team",
		`{"name":"x","color":"#fff"}`, s.UserToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *TeamAdminSuite) Test_02_Admin_CreatesTeam_NoAutoJoin() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/team",
		`{"name":"admin-managed-team","color":"#00ff00"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var team struct {
		IdTeam int64 `json:"idTeam"`
	}
	json.NewDecoder(res.Body).Decode(&team)
	s.TeamID = team.IdTeam

	// creator (admin) must NOT be auto-joined
	mres := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/admin/team/%d/member", s.TeamID), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, mres.StatusCode)
	s.Equal("[]", readBody(s.T(), mres))
}

func (s *TeamAdminSuite) Test_03_AddMember_SendsTeamJoinedNotification() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/team/member",
		fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, s.TeamID, s.UserID), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	nres := Request(s.T(), s.App, "GET", "/api/private/notification", "", s.UserToken)
	s.Require().Equal(http.StatusOK, nres.StatusCode)
	s.Contains(readBody(s.T(), nres), "team_joined")
}

func (s *TeamAdminSuite) Test_04_GetAllTeams_VisibleToAnyUser() {
	res := Request(s.T(), s.App, "GET", "/api/private/team", "", s.UserToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Contains(readBody(s.T(), res), "admin-managed-team")
}

func (s *TeamAdminSuite) Test_05_GetMyTeams_OnlyMembership() {
	// teamuser was added in Test_03 → sees the team
	res := Request(s.T(), s.App, "GET", "/api/private/team/my", "", s.UserToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Contains(readBody(s.T(), res), "admin-managed-team")

	// admin is not a member → must NOT see it in /team/my
	ares := Request(s.T(), s.App, "GET", "/api/private/team/my", "", s.AdminToken)
	s.Require().Equal(http.StatusOK, ares.StatusCode)
	s.NotContains(readBody(s.T(), ares), "admin-managed-team")
}

func (s *TeamAdminSuite) Test_06_InvitationRoutes_Gone() {
	res := Request(s.T(), s.App, "POST", "/api/private/team/1/invitation",
		`{"email":"x@test.sk"}`, s.AdminToken)
	s.Equal(http.StatusNotFound, res.StatusCode)

	res = Request(s.T(), s.App, "GET", "/api/private/team/1/invitation", "", s.AdminToken)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func Test_RunTeamAdminSuite(t *testing.T) {
	suite.Run(t, new(TeamAdminSuite))
}
