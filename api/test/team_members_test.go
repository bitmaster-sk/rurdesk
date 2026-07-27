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

// TeamMemberSuite tests the non-admin GET /api/private/team/:idTeam/members
// endpoint. A team member must receive 200 with the team members; a non-member
// must receive 403.
type TeamMemberSuite struct {
	suite.Suite
	App            *issue.Application
	AdminToken     string
	MemberToken    string
	NonMemberToken string
	TeamID         int64
	MemberID       int64
}

func (s *TeamMemberSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)

	// Create two regular users.
	s.MemberToken = createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"teammember","email":"teammember@test.sk","password":"kreslo"}`)
	s.MemberID = idOfUser(s.T(), s.App, s.AdminToken, "teammember@test.sk")

	_ = createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"nonmember","email":"nonmember@test.sk","password":"kreslo"}`)
	s.NonMemberToken = loginUser(s.T(), s.App, "nonmember@test.sk", "kreslo")

	// Admin creates a team and adds the member user.
	res := Request(s.T(), s.App, "POST", "/api/private/admin/team",
		`{"name":"mention-test-team","color":"#abcdef"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var team struct {
		IdTeam int64 `json:"idTeam"`
	}
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&team))
	s.TeamID = team.IdTeam

	res = Request(s.T(), s.App, "POST", "/api/private/admin/team/member",
		fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, s.TeamID, s.MemberID), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *TeamMemberSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.team WHERE name = 'mention-test-team'")
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email IN ('teammember@test.sk','nonmember@test.sk')")
}

func (s *TeamMemberSuite) Test_01_Member_CanListTeamMembers() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/team/%d/members", s.TeamID), "", s.MemberToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	body := readBody(s.T(), res)
	s.Contains(body, "teammember@test.sk", "response must include the member's email")
}

func (s *TeamMemberSuite) Test_02_NonMember_IsForbidden() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/team/%d/members", s.TeamID), "", s.NonMemberToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *TeamMemberSuite) Test_03_Unauthenticated_IsUnauthorized() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/team/%d/members", s.TeamID), "", "")
	s.Equal(http.StatusUnauthorized, res.StatusCode)
}

func Test_RunTeamMemberSuite(t *testing.T) {
	suite.Run(t, new(TeamMemberSuite))
}

// loginUser logs in with email+password and returns the token.
func loginUser(t *testing.T, app *issue.Application, email, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	res := Request(t, app, "POST", "/api/public/login", body, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("loginUser: expected 200, got %d (email=%s)", res.StatusCode, email)
	}
	var tk struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tk); err != nil {
		t.Fatalf("loginUser: decode error: %v", err)
	}
	return tk.Token
}
