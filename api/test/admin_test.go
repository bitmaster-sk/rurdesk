package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type AdminSuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
}

func (s *AdminSuite) SetupSuite() {
	s.App = Setup(s.T()) // seeds test@test.sk as the first user → admin
	s.AdminToken = Token(s.T(), s.App)
}

func (s *AdminSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email != 'test@test.sk'")
}

func (s *AdminSuite) Test_FirstUser_IsAdmin() {
	res := Request(s.T(), s.App, "GET", "/api/private/user", "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var u model.User
	s.Nil(json.NewDecoder(res.Body).Decode(&u))
	s.True(u.IsAdmin, "the first registered user must be the bootstrap admin")
	s.False(u.IsAgent)
}

func (s *AdminSuite) Test_ListUsers_AsAdmin() {
	res := Request(s.T(), s.App, "GET", "/api/private/admin/user", "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var users []model.User
	s.Nil(json.NewDecoder(res.Body).Decode(&users))
	s.GreaterOrEqual(len(users), 1)
}

func (s *AdminSuite) Test_AdminOnly_BlocksNonAdmin() {
	// A non-admin user (created by admin, isAdmin defaults false) is rejected by /admin/*.
	tok := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"plain user","email":"plain@test.sk","password":"secret1"}`)
	res := Request(s.T(), s.App, "GET", "/api/private/admin/user", "", tok)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *AdminSuite) Test_CreateHuman_CanLogIn() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'alice@test.sk'")
	body := `{"name":"alice","email":"alice@test.sk","password":"secret1"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	login := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"alice@test.sk","password":"secret1"}`, "")
	s.Equal(http.StatusOK, login.StatusCode)
}

func (s *AdminSuite) Test_CreateHuman_RequiresCredentials() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"nopass"}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *AdminSuite) Test_CreateHuman_WithProject_AssignsRole() {
	// Ensure a clean slate — the suite shares a persistent DB across local runs
	// and creating a fixed-email user twice returns 409. bob is only referenced
	// by ON DELETE CASCADE foreign keys, so this targeted delete always succeeds.
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'bob@test.sk'")

	idProject := createProject(s.T(), s.App, s.AdminToken, "admin-assign-project")
	body := fmt.Sprintf(
		`{"name":"bob","email":"bob@test.sk","password":"secret1","idProject":%d,"role":"member"}`,
		idProject)
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	members := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/member", idProject), "", s.AdminToken)
	s.Equal(http.StatusOK, members.StatusCode)
	s.Contains(readBody(s.T(), members), "bob@test.sk")
}

func (s *AdminSuite) Test_CreateAgent_MintsKey_AndAuthenticates() {
	body := `{"name":"CI Bot","isAgent":true}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.True(out.IsAgent)
	s.Contains(out.Email, "@")
	s.NotEmpty(out.RawKey, "bot must receive a raw key once")

	authed := Request(s.T(), s.App, "GET", "/api/private/user", "", out.RawKey)
	s.Equal(http.StatusOK, authed.StatusCode)
}

func (s *AdminSuite) Test_CreateAgent_CannotPasswordLogin() {
	body := `{"name":"NoLogin Bot","isAgent":true}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))

	login := Request(s.T(), s.App, "POST", "/api/public/login",
		fmt.Sprintf(`{"email":%q,"password":"whatever"}`, out.Email), "")
	s.Equal(http.StatusUnauthorized, login.StatusCode)
}

func (s *AdminSuite) Test_CreateAgent_EmailCollision_Suffixed() {
	mk := func() model.AdminCreateUserRes {
		res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
			`{"name":"dup bot","isAgent":true}`, s.AdminToken)
		s.Equal(http.StatusOK, res.StatusCode)
		var out model.AdminCreateUserRes
		s.Nil(json.NewDecoder(res.Body).Decode(&out))
		return out
	}
	a, b := mk(), mk()
	s.NotEqual(a.Email, b.Email, "second bot with same name must get a suffixed unique email")
}

func (s *AdminSuite) Test_PromoteThenDemote() {
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"promo","email":"promo@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "promo@test.sk")

	// promote
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":true}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	// A session minted AFTER promotion carries isAdmin=true and can hit admin routes.
	// (Existing sessions keep their snapshot until the 24h token expires.)
	login := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"promo@test.sk","password":"secret1"}`, "")
	var tk struct {
		Token string `json:"token"`
	}
	s.Nil(json.NewDecoder(login.Body).Decode(&tk))
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/admin/user", "", tk.Token).StatusCode)

	// demote
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":false}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *AdminSuite) Test_GuardLastAdmin_BlocksSelfDemote() {
	id := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":false}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *AdminSuite) Test_AgentCannotBecomeAdmin() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"admin bot","isAgent":true}`, s.AdminToken)
	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))

	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", out.IdUser), `{"isAdmin":true}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, patch.StatusCode)
}

func (s *AdminSuite) Test_DeleteUser_RemovesKeysAndMemberships() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"delbot","isAgent":true}`, s.AdminToken)
	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", out.IdUser), "", s.AdminToken)
	s.Equal(http.StatusOK, del.StatusCode)

	// its key no longer authenticates (api_key cascaded away)
	authed := Request(s.T(), s.App, "GET", "/api/private/user", "", out.RawKey)
	s.Equal(http.StatusUnauthorized, authed.StatusCode)
}

func (s *AdminSuite) Test_DeleteUser_GuardLastAdmin() {
	id := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", id), "", s.AdminToken)
	// The bootstrap admin owns projects, so the delete hits the last-owner guard
	// (422 from the admin controller) before/alongside the last-admin guard —
	// either way it must be a 4xx rejection, never a 200 delete.
	s.Contains([]int{http.StatusUnprocessableEntity, http.StatusBadRequest, http.StatusConflict},
		del.StatusCode, readBody(s.T(), del))
}

func (s *AdminSuite) Test_DeleteUser_WithAgentRun_409() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "agent-del-project")
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"agent bot","isAgent":true}`, s.AdminToken)
	var agent model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&agent))

	ctx := context.Background()
	var idIssue int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by)
		VALUES (1, $1, 'agent issue', 'x', $2, $2) RETURNING id_issue`,
		idProject, adminID).Scan(&idIssue)
	s.Require().Nil(err)
	_, err = s.App.Pool.Exec(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, stage_plan)
		VALUES ($1, $2, $3, '{"stages":[]}'::jsonb)`,
		idIssue, agent.IdUser, idProject)
	s.Require().Nil(err)

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", agent.IdUser), "", s.AdminToken)
	s.Equal(http.StatusConflict, del.StatusCode, "deleting a bot with agent history must 409")
}

func (s *AdminSuite) Test_DeleteUser_WithAuthoredIssue_409() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "author-del-project")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"issue author","email":"author-del@test.sk","password":"Passw0rd!23"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var author model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&author))

	ctx := context.Background()
	_, err := s.App.Pool.Exec(ctx, `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by)
		VALUES (900, $1, 'authored issue', 'x', $2, $2)`,
		idProject, author.IdUser)
	s.Require().Nil(err)

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", author.IdUser), "", s.AdminToken)
	s.Equal(http.StatusConflict, del.StatusCode,
		"deleting a user who authored an issue must 409, not 500")

	var stillThere bool
	s.Require().Nil(s.App.Pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users.user WHERE id_user = $1)", author.IdUser).Scan(&stillThere))
	s.True(stillThere, "the rejected delete must leave the user in place")
}

func (s *AdminSuite) Test_DeleteUser_OnlyAssigned_UnassignsAndSucceeds() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "assignee-del-project")
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"assignee","email":"assignee-del@test.sk","password":"Passw0rd!23"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var assignee model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&assignee))

	ctx := context.Background()
	var idIssue int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by, assigned_to)
		VALUES (901, $1, 'assigned issue', 'x', $2, $2, $3) RETURNING id_issue`,
		idProject, adminID, assignee.IdUser).Scan(&idIssue)
	s.Require().Nil(err)

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", assignee.IdUser), "", s.AdminToken)
	s.Equal(http.StatusOK, del.StatusCode, "an assignee with no authoring history is deletable")

	var assignedTo *int64
	s.Require().Nil(s.App.Pool.QueryRow(ctx,
		"SELECT assigned_to FROM issues.issue WHERE id_issue = $1", idIssue).Scan(&assignedTo))
	s.Nil(assignedTo, "deleting the assignee must unassign the issue")
}

// Test_AgentApiKey_SecondKeyRejected pins the current key-management semantics: an
// agent holds at most one API key (partial unique index api_key_agent_one_per_user).
// Agent creation mints the initial key; minting a second is rejected with 409 —
// rotation happens via the regenerate endpoint, not by adding keys.
func (s *AdminSuite) Test_AgentApiKey_SecondKeyRejected() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"keys bot","isAgent":true}`, s.AdminToken)
	var agent model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&agent))
	s.NotEmpty(agent.RawKey, "bot creation mints the initial key")

	// A second key is rejected — one key per agent.
	mint := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", agent.IdUser),
		`{"name":"deploy"}`, s.AdminToken)
	s.Equal(http.StatusConflict, mint.StatusCode, "a bot may hold only one key")

	// The initial key still authenticates.
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode,
		"the minted key must authenticate")
}

// Test_GetAgentApiKey verifies the single-key GET endpoint returns the agent's one key.
func (s *AdminSuite) Test_GetAgentApiKey() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"list keys bot","isAgent":true}`, s.AdminToken)
	var agent model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&agent))

	get := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", agent.IdUser), "", s.AdminToken)
	s.Equal(http.StatusOK, get.StatusCode)
	var key model.ApiKey
	s.Nil(json.NewDecoder(get.Body).Decode(&key))
	s.NotZero(key.IdApiKey, "bot has its minted key")
	s.Equal("default", key.Name, "the initial key is named \"default\"")
}

func (s *AdminSuite) Test_AgentApiKeyEndpoints_RejectHumanTarget() {
	id := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk") // a human (admin)
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", id), `{"name":"x"}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *AdminSuite) Test_PublicRegisterClosed() {
	res := Request(s.T(), s.App, "POST", "/api/public/register",
		`{"name":"intruder","email":"intruder@test.sk","password":"secret1"}`, "")
	s.Equal(http.StatusForbidden, res.StatusCode)
}

// Test_DemoteNonAdmin_NoOp_NotBlocked pins that guardLastAdmin only fires on a real
// demote. Relies on the suite's default single-admin state (only the seed admin).
func (s *AdminSuite) Test_DemoteNonAdmin_NoOp_NotBlocked() {
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"plain noop","email":"plainnoop@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "plainnoop@test.sk")

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":false}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode,
		"demoting a user who is not an admin must be a 200 no-op, not a last-admin 422")
}

func (s *AdminSuite) Test_Demote_InvalidatesExistingSession() {
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"stale admin","email":"stale@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "stale@test.sk")

	// promote, then log in → session carries isAdmin=true
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	login := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"stale@test.sk","password":"secret1"}`, "")
	var tk struct {
		Token string `json:"token"`
	}
	s.Require().Nil(json.NewDecoder(login.Body).Decode(&tk))
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/admin/user", "", tk.Token).StatusCode)

	// demote → the old session must die immediately, not after the 24h TTL
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", tk.Token).StatusCode,
		"a demoted user's existing session must be invalidated")
}

func (s *AdminSuite) Test_DeleteUser_InvalidatesSession() {
	tok := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"doomed user","email":"doomed@test.sk","password":"secret1"}`)
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", tok).StatusCode)

	id := idOfUser(s.T(), s.App, s.AdminToken, "doomed@test.sk")
	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", id), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, del.StatusCode)

	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", tok).StatusCode,
		"a deleted user's session must be invalidated")
}

func (s *AdminSuite) Test_DeleteAgent_PurgesApiKeyCache() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"cached bot","isAgent":true}`, s.AdminToken)
	var agent model.AdminCreateUserRes
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&agent))

	// use the key once → warms the 5-minute auth cache
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode)

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", agent.IdUser), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, del.StatusCode)

	// without the cache purge this would still return 200 until the TTL expired
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode,
		"a deleted bot's cached key session must be purged immediately")
}

// Test_CreateAgent_FailedAssign_RollsBackAgent pins atomicity: when the project
// assignment fails (agents cannot be owners), the half-created agent must be rolled back.
func (s *AdminSuite) Test_CreateAgent_FailedAssign_RollsBackAgent() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "tx-rollback-project")
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"tx rollback bot","isAgent":true,"idProject":%d,"role":"owner"}`, idProject),
		s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)

	list := Request(s.T(), s.App, "GET", "/api/private/admin/user", "", s.AdminToken)
	s.NotContains(readBody(s.T(), list), "tx rollback bot",
		"a failed create must not leave a half-created bot behind")
}

func (s *AdminSuite) Test_CreateHuman_DuplicateEmail_409() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'duphuman@test.sk'")
	body := `{"name":"dup human","email":"duphuman@test.sk","password":"secret1"}`
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken).StatusCode)
	s.Equal(http.StatusConflict,
		Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken).StatusCode)
}

func (s *AdminSuite) Test_DisableUser_BlocksLogin_ExistingSessionDies() {
	tok := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"disabled user","email":"disabled@test.sk","password":"secret1"}`)
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", tok).StatusCode)
	id := idOfUser(s.T(), s.App, s.AdminToken, "disabled@test.sk")

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	// existing session is invalidated
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", tok).StatusCode,
		"a disabled user's existing session must be invalidated")
	// fresh login is rejected with the same 401 as a wrong password
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "POST", "/api/public/login",
			`{"email":"disabled@test.sk","password":"secret1"}`, "").StatusCode,
		"a disabled user must not be able to log in")

	// re-enable restores login
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "POST", "/api/public/login",
			`{"email":"disabled@test.sk","password":"secret1"}`, "").StatusCode,
		"a re-enabled user must be able to log in again")
}

func (s *AdminSuite) Test_DisableUser_ListShowsFlag() {
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"listed user","email":"listed@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "listed@test.sk")

	getUser := func() model.User {
		res := Request(s.T(), s.App, "GET", "/api/private/admin/user", "", s.AdminToken)
		s.Require().Equal(http.StatusOK, res.StatusCode)
		var users []model.User
		s.Require().Nil(json.NewDecoder(res.Body).Decode(&users))
		for _, u := range users {
			if u.IdUser == id {
				return u
			}
		}
		s.Fail("user not listed")
		return model.User{}
	}
	s.True(getUser().IsActive, "a freshly created user must be active")

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.False(getUser().IsActive, "the list must expose isActive=false after disabling")
}

func (s *AdminSuite) Test_DisableAgent_StopsApiKey_AndDispatch() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "agent-disable-project")
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"dis bot","isAgent":true}`, s.AdminToken)
	var agent model.AdminCreateUserRes
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&agent))

	ctx := context.Background()
	var idIssue int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by)
		VALUES ((SELECT COALESCE(MAX(id_issue_public), 0) + 1 FROM issues.issue WHERE id_project = $1), $1, 'disabled agent issue', 'x', $2, $2)
		RETURNING id_issue`,
		idProject, adminID).Scan(&idIssue)
	s.Require().Nil(err)
	_, err = s.App.Pool.Exec(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'queued', '{"stages":[]}'::jsonb)`,
		idIssue, agent.IdUser, idProject)
	s.Require().Nil(err)

	// scheduler picks the agent up while active
	ids, err := injector.GetAgentRunRepository().LoadActiveAgentIds(ctx)
	s.Require().Nil(err)
	s.Contains(ids, agent.IdUser, "an active agent with a queued run must be dispatched")

	// warm the api-key cache
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode)

	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", agent.IdUser), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, patch.StatusCode)

	// the cached key session is purged and the DB path filters disabled users
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode,
		"a disabled agent's api key must stop authenticating immediately")
	// no further dispatch for the disabled agent: the queued run was cancelled
	// by the disable, so the agent id disappears from the active set entirely.
	ids, err = injector.GetAgentRunRepository().LoadActiveAgentIds(ctx)
	s.Require().Nil(err)
	s.NotContains(ids, agent.IdUser, "a disabled agent must not be dispatched again")
	var phase string
	s.Require().Nil(s.App.Pool.QueryRow(ctx, `
		SELECT phase FROM agent.run WHERE id_issue = $1`, idIssue).Scan(&phase))
	s.Equal("cancelled", phase, "the disable must cancel the agent's non-terminal run")

	// re-enable: the key works again (the key itself was never deleted)
	patch = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", agent.IdUser), `{"isActive":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, patch.StatusCode)
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", agent.RawKey).StatusCode,
		"a re-enabled agent's api key must work again")
	// The cancelled run stays terminal — re-enabling does not resurrect it.
	ids, err = injector.GetAgentRunRepository().LoadActiveAgentIds(ctx)
	s.Require().Nil(err)
	s.NotContains(ids, agent.IdUser, "a cancelled run must not make the agent dispatchable again")
}

func (s *AdminSuite) Test_DisableSelf_Rejected() {
	id := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":false}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode,
		"an admin must not be able to disable their own account")

	var stillActive bool
	s.Require().Nil(s.App.Pool.QueryRow(context.Background(),
		"SELECT is_active FROM users.user WHERE id_user = $1", id).Scan(&stillActive))
	s.True(stillActive, "the rejected self-disable must leave the account active")
}

func (s *AdminSuite) Test_DisableAdmin_WithAnotherActiveAdmin() {
	// Disabling an admin is allowed when another active admin remains — the
	// account simply loses access; history is preserved.
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"admin to disable","email":"admin-disable@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "admin-disable@test.sk")
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "POST", "/api/public/login",
			`{"email":"admin-disable@test.sk","password":"secret1"}`, "").StatusCode,
		"a disabled admin must not be able to log in")
	// The acting admin is untouched.
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/admin/user", "", s.AdminToken).StatusCode)
}

func (s *AdminSuite) Test_DisableUser_SelfProfile_StillEditable() {
	// Session invalidation on disable must not leak into an unrelated profile edit.
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"profile edit","email":"profile-edit@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "profile-edit@test.sk")
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"name":"profile edited"}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var name string
	s.Require().Nil(s.App.Pool.QueryRow(context.Background(),
		"SELECT name FROM users.user WHERE id_user = $1", id).Scan(&name))
	s.Equal("profile edited", name)
}

// createIssueInProject inserts an issue directly (bypassing the API) so tests can
// drive agent assignment without depending on state/severity/type fixtures.
func (s *AdminSuite) createIssueRaw(idProject int64, title string) int64 {
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
	var idIssue int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by)
		VALUES ((SELECT COALESCE(MAX(id_issue_public), 0) + 1 FROM issues.issue WHERE id_project = $1), $1, $2, 'x', $3, $3)
		RETURNING id_issue`,
		idProject, title, adminID).Scan(&idIssue)
	s.Require().Nil(err)
	return idIssue
}

// createAgentWithGateway registers an agent and gives it a gateway row so the
// assign path's gateway check passes.
func (s *AdminSuite) createAgentWithGateway(name string) model.AdminCreateUserRes {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.AdminToken)
	if !s.Equal(http.StatusOK, res.StatusCode) {
		s.FailNow(readBody(s.T(), res))
	}
	var agent model.AdminCreateUserRes
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&agent))
	_, err := s.App.Pool.Exec(context.Background(),
		`INSERT INTO agent.gateway (id_user_agent, gateway_url, webhook_secret) VALUES ($1, 'http://gateway.test', decode('00','hex'))`,
		agent.IdUser)
	s.Require().Nil(err)
	return agent
}

func (s *AdminSuite) Test_DisableAgent_CancelsNonTerminalRuns() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "agent-disable-runs-project")
	agent := s.createAgentWithGateway("cancel bot")
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")

	ctx := context.Background()
	var idIss int64
	s.Require().Nil(s.App.Pool.QueryRow(ctx, `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by)
		VALUES ((SELECT COALESCE(MAX(id_issue_public), 0) + 1 FROM issues.issue WHERE id_project = $1), $1, 'run to cancel', 'x', $2, $2)
		RETURNING id_issue`, idProject, adminID).Scan(&idIss))
	var idRun int64
	s.Require().Nil(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'in_progress', '{"stages":[]}'::jsonb) RETURNING id_run`,
		idIss, agent.IdUser, idProject).Scan(&idRun))
	_, taskErr := s.App.Pool.Exec(ctx, `
		INSERT INTO agent.task (id_run, stage, attempt_no, status) VALUES ($1, 'implementation', 1, 'active')`, idRun)
	s.Require().Nil(taskErr)

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", agent.IdUser), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	var phase string
	s.Require().Nil(s.App.Pool.QueryRow(ctx, "SELECT phase FROM agent.run WHERE id_run = $1", idRun).Scan(&phase))
	s.Equal("cancelled", phase, "disabling an agent must cancel its non-terminal run")
	var taskStatus string
	s.Require().Nil(s.App.Pool.QueryRow(ctx, `
		SELECT status FROM agent.task WHERE id_run = $1`, idRun).Scan(&taskStatus))
	s.Equal("cancelled", taskStatus, "the in-flight task must be cancelled too")
	var events int
	s.Require().Nil(s.App.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM agent.run_event WHERE id_run = $1 AND to_phase = 'cancelled'`, idRun).Scan(&events))
	s.GreaterOrEqual(events, 1, "the cancel must be recorded as a run event")
	// The acting admin must be recorded as the actor so run_event shows who
	// cancelled the run.
	var actorID int64
	s.Require().Nil(s.App.Pool.QueryRow(ctx, `
		SELECT id_user FROM agent.run_event WHERE id_run = $1 AND to_phase = 'cancelled' ORDER BY id_event DESC LIMIT 1`, idRun).Scan(&actorID))
	s.Equal(adminID, actorID, "the cancel event must name the acting admin as actor")
}

func (s *AdminSuite) Test_DisableLastActiveAdmin_StillBlocked_ToggleDisabledAdminAllowed() {
	// One active admin (the bootstrap) + one admin that gets disabled: with
	// CountAdmins counting active admins only, the guard must NOT block acting on
	// the *disabled* admin (demote/delete/disable again is a no-op or allowed),
	// but must still block disabling the single remaining active admin's own…
	// self-disable is rejected separately, so here: demoting the disabled admin
	// must succeed.
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"second admin","email":"second-admin@test.sk","password":"secret1"}`)
	id := idOfUser(s.T(), s.App, s.AdminToken, "second-admin@test.sk")
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	// Now only the bootstrap admin is active. Demoting the DISABLED admin must
	// not trip the last-admin guard (it is not an active admin).
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", id), `{"isAdmin":false}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"demoting an already-disabled admin must not hit the last-admin guard")

	// But demoting the last ACTIVE admin is still blocked.
	bootstrap := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", bootstrap), `{"isAdmin":false}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode,
		"demoting the last active admin must stay blocked")
}

func (s *AdminSuite) Test_DisableLastProjectOwner_Rejected_WithProjectName() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "owner-guard-project")
	// The bootstrap admin is the project creator → sole owner. Create a second
	// user, make them project member (not owner), then disabling the admin-owner
	// is the acting admin itself (self-disable) — use a second admin as owner.
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"guard admin","email":"guard-admin@test.sk","password":"secret1"}`)
	ownerID := idOfUser(s.T(), s.App, s.AdminToken, "guard-admin@test.sk")
	// Promote to admin and add as owner of the project.
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", ownerID), `{"isAdmin":true}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	res = Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, ownerID), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Make the bootstrap admin a non-owner so ownerID is the only *active* owner
	// when the bootstrap is disabled… simpler: demote the bootstrap's ownership.
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject,
			idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")),
		`{"role":"member"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Disable the sole owner → must be rejected with 422 and name the project.
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", ownerID), `{"isActive":false}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
	s.Contains(readBody(s.T(), res), "owner-guard-project",
		"the error must name the project whose last owner would be disabled")

	// Add a second active owner, then the disable succeeds.
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"second owner","email":"second-owner@test.sk","password":"secret1"}`)
	secondOwner := idOfUser(s.T(), s.App, s.AdminToken, "second-owner@test.sk")
	res = Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, secondOwner), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", ownerID), `{"isActive":false}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"with another active owner present the disable must succeed")
}

func (s *AdminSuite) Test_DisabledOwner_NoLongerCounts_AsProjectOwner() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "owner-count-project")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"count owner a","email":"count-a@test.sk","password":"secret1"}`)
	idA := idOfUser(s.T(), s.App, s.AdminToken, "count-a@test.sk")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"count owner b","email":"count-b@test.sk","password":"secret1"}`)
	idB := idOfUser(s.T(), s.App, s.AdminToken, "count-b@test.sk")
	for _, id := range []int64{idA, idB} {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, id), s.AdminToken)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
	// Demote the bootstrap owner to member so A and B are the only owners.
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject,
			idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")),
		`{"role":"member"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Disable A: owner count must drop to 1 (only B counts as active), so
	// demoting B afterwards must be rejected by the member controller's own
	// last-owner guard (ErrLastOwner), proving the count no longer sees A.
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", idA), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject, idB),
		`{"role":"member"}`, s.AdminToken)
	// The member controller maps ErrLastOwner to 400 (its existing convention).
	s.Equal(http.StatusBadRequest, res.StatusCode, readBody(s.T(), res),
		"with only one ACTIVE owner left, demoting them must hit the last-owner guard")
}

func (s *AdminSuite) Test_AssignDisabledAgent_Rejected() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "assign-disabled-project")
	agent := s.createAgentWithGateway("assign bot")
	// The agent must be able to read the project for the assign path to reach the
	// disabled check (same gate as the EditIssue assignee path).
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, agent.IdUser), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	idIssue := s.createIssueRaw(idProject, "assign-disabled issue")
	var idIssuePublic int64
	s.Require().Nil(s.App.Pool.QueryRow(context.Background(),
		"SELECT id_issue_public FROM issues.issue WHERE id_issue = $1", idIssue).Scan(&idIssuePublic))

	// Assign while active → 200.
	res = Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", idProject, idIssuePublic),
		fmt.Sprintf(`{"idUserAgent":%d}`, agent.IdUser), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Disable the agent, then re-assign a DIFFERENT issue to it via EditIssue → 422.
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", agent.IdUser), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	idIssue3 := s.createIssueRaw(idProject, "assign-disabled issue 3")
	var idIssuePublic3 int64
	s.Require().Nil(s.App.Pool.QueryRow(context.Background(),
		"SELECT id_issue_public FROM issues.issue WHERE id_issue = $1", idIssue3).Scan(&idIssuePublic3))
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, idIssuePublic3),
		fmt.Sprintf(`{"assignedTo":%d}`, agent.IdUser), s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode, readBody(s.T(), res),
		"assigning a disabled agent via EditIssue must be rejected")

	// AssignAgent endpoint on another issue → 422 as well.
	idIssue2 := s.createIssueRaw(idProject, "assign-disabled issue 2")
	var idIssuePublic2 int64
	s.Require().Nil(s.App.Pool.QueryRow(context.Background(),
		"SELECT id_issue_public FROM issues.issue WHERE id_issue = $1", idIssue2).Scan(&idIssuePublic2))
	res = Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", idProject, idIssuePublic2),
		fmt.Sprintf(`{"idUserAgent":%d}`, agent.IdUser), s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode, readBody(s.T(), res),
		"the assign-agent endpoint must reject a disabled agent")
}

// A disabled owner must be removable/demotable while another active owner
// remains: the member controller's last-owner guard must only run when the
// target is itself active (GetProjectOwnerCount already excludes disabled
// users, so guarding a disabled owner wrongly blocks with ErrLastOwner).
func (s *AdminSuite) Test_RemoveDisabledOwner_WithActiveOwnerRemaining_Succeeds() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "disabled-owner-remove-project")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"owner x","email":"owner-x@test.sk","password":"secret1"}`)
	idX := idOfUser(s.T(), s.App, s.AdminToken, "owner-x@test.sk")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"owner y","email":"owner-y@test.sk","password":"secret1"}`)
	idY := idOfUser(s.T(), s.App, s.AdminToken, "owner-y@test.sk")
	for _, id := range []int64{idX, idY} {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, id), s.AdminToken)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
	// Demote the bootstrap creator to member so X and Y are the only owners.
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject,
			idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")),
		`{"role":"member"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Disable X; Y remains the only active owner.
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", idX), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Removing the DISABLED owner X must succeed (Y stays active owner).
	res = Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject, idX), "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"removing a disabled owner while another active owner remains must not hit the last-owner guard")

	// Demoting the disabled owner must also succeed — re-add X as owner first.
	res = Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, idX), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject, idX),
		`{"role":"member"}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"demoting a disabled owner while another active owner remains must not hit the last-owner guard")
}

// Deleting a disabled owner while another active owner remains must succeed:
// AdminController.DeleteUser must run the last-owner guard only for active targets.
// The user must have no agent activity and no authored content for DELETE to
// reach the guards at all.
func (s *AdminSuite) Test_DeleteDisabledOwner_WithActiveOwnerRemaining_Succeeds() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "disabled-owner-delete-project")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"del owner x","email":"del-owner-x@test.sk","password":"secret1"}`)
	idX := idOfUser(s.T(), s.App, s.AdminToken, "del-owner-x@test.sk")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"del owner y","email":"del-owner-y@test.sk","password":"secret1"}`)
	idY := idOfUser(s.T(), s.App, s.AdminToken, "del-owner-y@test.sk")
	for _, id := range []int64{idX, idY} {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"owner"}`, id), s.AdminToken)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", idProject,
			idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")),
		`{"role":"member"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", idX), `{"isActive":false}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	res = Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", idX), "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"deleting a disabled owner while another active owner remains must not hit the last-owner guard")
}

// Team variant: an owner team whose members are all disabled must be
// demotable/removable while another active owner remains.
func (s *AdminSuite) Test_RemoveAllDisabledOwnerTeam_WithActiveOwnerRemaining_Succeeds() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "disabled-owner-team-project")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"team owner z","email":"team-owner-z@test.sk","password":"secret1"}`)
	idZ := idOfUser(s.T(), s.App, s.AdminToken, "team-owner-z@test.sk")
	createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"team member w","email":"team-member-w@test.sk","password":"secret1"}`)
	idW := idOfUser(s.T(), s.App, s.AdminToken, "team-member-w@test.sk")
	// Bootstrap creator stays a direct owner so the project always has an active owner.
	// Team names are unique per instance, so suffix with the project name in case a
	// previous run left one behind.
	teamRes := Request(s.T(), s.App, "POST", "/api/private/admin/team",
		`{"name":"disabled-owner-team-x","color":"#123456"}`, s.AdminToken)
	teamBody := readBody(s.T(), teamRes)
	s.Require().Equal(http.StatusOK, teamRes.StatusCode, teamBody)
	var team struct {
		IdTeam int64 `json:"idTeam"`
	}
	s.Require().NoError(json.Unmarshal([]byte(teamBody), &team))
	for _, id := range []int64{idZ, idW} {
		res := Request(s.T(), s.App, "POST", "/api/private/admin/team/member",
			fmt.Sprintf(`{"idTeam":%d,"idUser":%d}`, team.IdTeam, id), s.AdminToken)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
	// Team is an owner of the project; the bootstrap admin is the active owner.
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/team", idProject),
		fmt.Sprintf(`{"idTeam":%d,"role":"owner"}`, team.IdTeam), s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	// Disable both team members, then demote the team — the guard must not fire
	// (no active member would lose ownership; the bootstrap owner stays).
	for _, id := range []int64{idZ, idW} {
		res = Request(s.T(), s.App, "PATCH",
			fmt.Sprintf("/api/private/admin/user/%d", id), `{"isActive":false}`, s.AdminToken)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/team/%d", idProject, team.IdTeam),
		`{"role":"member"}`, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"demoting an owner team with only disabled members must not hit the last-owner guard")

	// Re-add as owner, then remove the team — also allowed.
	res = Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/member/team/%d", idProject, team.IdTeam),
		`{"role":"owner"}`, s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	res = Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/member/team/%d", idProject, team.IdTeam), "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res),
		"removing an owner team with only disabled members must not hit the last-owner guard")
}

func Test_RunAdminSuite(t *testing.T) {
	suite.Run(t, new(AdminSuite))
}
