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

type ProjectMemberSuite struct {
	suite.Suite
	App         *issue.Application
	OwnerToken  string
	OwnerID     int64
	MemberToken string
	ProjectID   int64
}

func (s *ProjectMemberSuite) SetupSuite() {
	s.App = Setup(s.T())

	// Register owner (public registration is closed post-bootstrap; admin creates users)
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"owner","email":"pmowner@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	res := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"pmowner@test.sk","password":"kreslo"}`, "")
	var tk struct {
		Token string `json:"token"`
	}
	json.NewDecoder(res.Body).Decode(&tk)
	s.OwnerToken = tk.Token

	// Get owner ID
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.OwnerToken)
	var owner model.User
	json.NewDecoder(userRes.Body).Decode(&owner)
	s.OwnerID = owner.IdUser

	// Register a second user (member to be added)
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"member","email":"pmmember@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	res2 := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"pmmember@test.sk","password":"kreslo"}`, "")
	var tk2 struct {
		Token string `json:"token"`
	}
	json.NewDecoder(res2.Body).Decode(&tk2)
	s.MemberToken = tk2.Token

	// Create project (owner gets RoleOwner)
	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"member-test-project","color":"#123456"}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj model.Project
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.ProjectID = prj.IdProject
}

func (s *ProjectMemberSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE name IN ('member-test-project','member-race-project')")
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email IN ('pmowner@test.sk','pmmember@test.sk') OR email LIKE 'pmrace%@test.sk'")
}

func (s *ProjectMemberSuite) Test_01_GetUserRole_Owner() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var body model.UserRoleRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Equal(model.RoleOwner, body.Role)
}

func (s *ProjectMemberSuite) Test_02_GetMembers_RequiresOwner() {
	// Member not yet added — should get 403
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/member", s.ProjectID), "", s.MemberToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *ProjectMemberSuite) Test_03_GetMembers_Owner() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/member", s.ProjectID), "", s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var body model.ProjectMembersRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Len(body.Users, 1) // only the owner
	s.Equal(model.RoleOwner, body.Users[0].Role)
}

// A direct member belongs to no team, so IdsTeams stays empty. It must still
// serialize as [] — a nil slice would marshal to null and the client models the
// field as a plain array.
func (s *ProjectMemberSuite) Test_03a_GetMembers_IdsTeamsIsArrayNotNull() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/member", s.ProjectID), "", s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var body struct {
		Users []struct {
			IdsTeams *[]int64 `json:"idsTeams"`
		} `json:"users"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&body))
	s.Require().Len(body.Users, 1)
	s.Require().NotNil(body.Users[0].IdsTeams, "idsTeams must serialize as [], not null")
	s.Empty(*body.Users[0].IdsTeams)
}

func (s *ProjectMemberSuite) Test_04_AddUser_AsMember() {
	// Get member user ID
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.MemberToken)
	var member model.User
	json.NewDecoder(userRes.Body).Decode(&member)

	body := fmt.Sprintf(`{"idUser":%d,"role":"member"}`, member.IdUser)
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID), body, s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *ProjectMemberSuite) Test_05_MemberRole_AfterAdd() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", s.MemberToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var body model.UserRoleRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Equal(model.RoleMember, body.Role)
}

func (s *ProjectMemberSuite) Test_06_GetMembers_Shows2Users() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/member", s.ProjectID), "", s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var body model.ProjectMembersRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Len(body.Users, 2)
}

func (s *ProjectMemberSuite) Test_07_LastOwnerGuard_RemoveSelf() {
	res := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", s.ProjectID, s.OwnerID),
		"", s.OwnerToken)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *ProjectMemberSuite) Test_08_UpdateUserRole() {
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.MemberToken)
	var member model.User
	json.NewDecoder(userRes.Body).Decode(&member)

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", s.ProjectID, member.IdUser),
		`{"role":"viewer"}`, s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)

	roleRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", s.MemberToken)
	var roleBody model.UserRoleRes
	json.NewDecoder(roleRes.Body).Decode(&roleBody)
	s.Equal(model.RoleViewer, roleBody.Role)
}

func (s *ProjectMemberSuite) Test_09_RemoveUser() {
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.MemberToken)
	var member model.User
	json.NewDecoder(userRes.Body).Decode(&member)

	res := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", s.ProjectID, member.IdUser),
		"", s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)

	// Verify member no longer has access
	roleRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", s.MemberToken)
	s.Equal(http.StatusForbidden, roleRes.StatusCode)
}

func (s *ProjectMemberSuite) Test_10_DirectViewer_OverridesTeamOwner() {
	// This test verifies that direct assignment always wins over team inheritance,
	// even when the team grants a higher role. A user with direct "viewer" should
	// resolve to viewer, not the team's "owner".

	// Re-add member as direct viewer
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.MemberToken)
	var member model.User
	json.NewDecoder(userRes.Body).Decode(&member)

	// Create a team and add member to it (team management is admin-only)
	adminToken := Token(s.T(), s.App)
	teamRes := Request(s.T(), s.App, "POST", "/api/private/admin/team",
		`{"name":"acl-test-team","color":"#00ff00"}`, adminToken)
	s.Require().Equal(http.StatusOK, teamRes.StatusCode)
	var team struct {
		IdTeam int64 `json:"idTeam"`
	}
	json.NewDecoder(teamRes.Body).Decode(&team)

	Request(s.T(), s.App, "POST", "/api/private/admin/team/member",
		fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, team.IdTeam, member.IdUser), adminToken)

	// Add team to project as owner
	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/team", s.ProjectID),
		fmt.Sprintf(`{"idTeam":%d,"role":"owner"}`, team.IdTeam), s.OwnerToken)

	// Add member directly as viewer
	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"viewer"}`, member.IdUser), s.OwnerToken)

	// Direct viewer should win over team owner
	roleRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/user-role", s.ProjectID), "", s.MemberToken)
	s.Equal(http.StatusOK, roleRes.StatusCode)
	var roleBody model.UserRoleRes
	json.NewDecoder(roleRes.Body).Decode(&roleBody)
	s.Equal(model.RoleViewer, roleBody.Role)
}

func (s *ProjectMemberSuite) Test_11_DuplicateAdd_Returns409() {
	// Adding a user who already has a direct membership should return 409 Conflict.
	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.MemberToken)
	var member model.User
	json.NewDecoder(userRes.Body).Decode(&member)

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, member.IdUser), s.OwnerToken)
	s.Equal(http.StatusConflict, res.StatusCode)
}

// Test_12_LastOwnerGuard_ConcurrentRemove fires many simultaneous removals of all the
// project's owners at once. The guard must let all but one through, so the project always
// keeps exactly one owner. Many concurrent removers (rather than two) are used on
// purpose: the TOCTOU window between the owner count and the commit is sub-millisecond, so
// only high concurrency reliably exercises the race — with the unlocked guard several
// transactions read count > 1 at once and drain the project to zero owners.
//
// With the advisory-lock fix the removals serialize: each re-reads the decremented count, so
// exactly raceOwners-1 succeed and the last is rejected, leaving one owner. Uses a fresh
// project to stay independent of state mutated by earlier tests in the suite.
func (s *ProjectMemberSuite) Test_12_LastOwnerGuard_ConcurrentRemove() {
	const raceOwners = 8

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"member-race-project","color":"#abcdef"}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj model.Project
	json.NewDecoder(prjRes.Body).Decode(&prj)
	projectID := prj.IdProject

	// Owner ids to remove: the project creator plus (raceOwners-1) freshly created humans,
	// each promoted to owner.
	adminToken := Token(s.T(), s.App)
	ownerIDs := []int64{s.OwnerID}
	for i := 1; i < raceOwners; i++ {
		email := fmt.Sprintf("pmrace%d@test.sk", i)
		createUserAsAdmin(s.T(), s.App, adminToken,
			fmt.Sprintf(`{"name":"race%d","email":"%s","password":"kreslo"}`, i, email))
		idUser := idOfUser(s.T(), s.App, adminToken, email)
		addRes := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", projectID),
			fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, idUser), s.OwnerToken)
		s.Require().Equal(http.StatusOK, addRes.StatusCode)
		ownerIDs = append(ownerIDs, idUser)
	}

	// Release all removals at once via the start barrier. The remover is the global
	// admin, NOT one of the owners being removed: an admin is an implicit owner of every
	// project (not a project_user row), so its delete permission never depends on the rows
	// under test and is never invalidated mid-burst. Using an owner from the removal set as
	// the actor instead adds an unrelated race — its own self-removal invalidates its ACL
	// cache, so the remaining removals are rejected at the permission check (403) before ever
	// reaching the last-owner guard. That masks what this test targets: the guard itself.
	start := make(chan struct{})
	results := make(chan int, len(ownerIDs))
	for _, idUser := range ownerIDs {
		go func(idUser int64) {
			<-start
			res := Request(s.T(), s.App, "DELETE",
				fmt.Sprintf("/api/private/project/%d/member/user/%d", projectID, idUser),
				"", adminToken)
			results <- res.StatusCode
		}(idUser)
	}
	close(start)

	statuses := make(map[int]int)
	for range ownerIDs {
		statuses[<-results]++
	}

	// All but one removal succeeds; exactly one is rejected by the guard.
	s.Equal(raceOwners-1, statuses[http.StatusOK], "expected all-but-one removals to succeed")
	s.Equal(1, statuses[http.StatusBadRequest], "expected exactly one removal blocked by guard")

	// Invariant: the project still has exactly one owner.
	var owners int
	err := s.App.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM projects.project_user WHERE id_project = $1 AND role = 'owner'",
		projectID).Scan(&owners)
	s.Require().NoError(err)
	s.Equal(1, owners)
}

func Test_RunProjectMemberSuite(t *testing.T) {
	suite.Run(t, new(ProjectMemberSuite))
}
