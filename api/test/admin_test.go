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
	s.False(u.IsBot)
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

func (s *AdminSuite) Test_CreateBot_MintsKey_AndAuthenticates() {
	body := `{"name":"CI Bot","isBot":true}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.True(out.IsBot)
	s.Contains(out.Email, "@")
	s.NotEmpty(out.RawKey, "bot must receive a raw key once")

	authed := Request(s.T(), s.App, "GET", "/api/private/user", "", out.RawKey)
	s.Equal(http.StatusOK, authed.StatusCode)
}

func (s *AdminSuite) Test_CreateBot_CannotPasswordLogin() {
	body := `{"name":"NoLogin Bot","isBot":true}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))

	login := Request(s.T(), s.App, "POST", "/api/public/login",
		fmt.Sprintf(`{"email":%q,"password":"whatever"}`, out.Email), "")
	s.Equal(http.StatusUnauthorized, login.StatusCode)
}

func (s *AdminSuite) Test_CreateBot_EmailCollision_Suffixed() {
	mk := func() model.AdminCreateUserRes {
		res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
			`{"name":"dup bot","isBot":true}`, s.AdminToken)
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

func (s *AdminSuite) Test_BotCannotBecomeAdmin() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"admin bot","isBot":true}`, s.AdminToken)
	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))

	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", out.IdUser), `{"isAdmin":true}`, s.AdminToken)
	s.Equal(http.StatusUnprocessableEntity, patch.StatusCode)
}

func (s *AdminSuite) Test_DeleteUser_RemovesKeysAndMemberships() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"delbot","isBot":true}`, s.AdminToken)
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
	s.Equal(http.StatusUnprocessableEntity, del.StatusCode)
}

func (s *AdminSuite) Test_DeleteUser_WithAgentRun_409() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "agent-del-project")
	adminID := idOfUser(s.T(), s.App, s.AdminToken, "test@test.sk")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"agent bot","isBot":true}`, s.AdminToken)
	var bot model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&bot))

	ctx := context.Background()
	var idIssue int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO issues.issue (id_issue_public, id_project, title, description, create_by, update_by)
		VALUES (1, $1, 'agent issue', 'x', $2, $2) RETURNING id_issue`,
		idProject, adminID).Scan(&idIssue)
	s.Require().Nil(err)
	_, err = s.App.Pool.Exec(ctx, `
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, stage_plan)
		VALUES ($1, $2, $3, '{"stages":[]}'::jsonb)`,
		idIssue, bot.IdUser, idProject)
	s.Require().Nil(err)

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", bot.IdUser), "", s.AdminToken)
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
		`{"name":"keys bot","isBot":true}`, s.AdminToken)
	var bot model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&bot))
	s.NotEmpty(bot.RawKey, "bot creation mints the initial key")

	// A second key is rejected — one key per bot.
	mint := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", bot.IdUser),
		`{"name":"deploy"}`, s.AdminToken)
	s.Equal(http.StatusConflict, mint.StatusCode, "a bot may hold only one key")

	// The initial key still authenticates.
	s.Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", bot.RawKey).StatusCode,
		"the minted key must authenticate")
}

// Test_GetAgentApiKey verifies the single-key GET endpoint returns the agent's one key.
func (s *AdminSuite) Test_GetAgentApiKey() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"list keys bot","isBot":true}`, s.AdminToken)
	var bot model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&bot))

	get := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", bot.IdUser), "", s.AdminToken)
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

func (s *AdminSuite) Test_DeleteBot_PurgesApiKeyCache() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"cached bot","isBot":true}`, s.AdminToken)
	var bot model.AdminCreateUserRes
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&bot))

	// use the key once → warms the 5-minute auth cache
	s.Require().Equal(http.StatusOK,
		Request(s.T(), s.App, "GET", "/api/private/user", "", bot.RawKey).StatusCode)

	del := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/admin/user/%d", bot.IdUser), "", s.AdminToken)
	s.Require().Equal(http.StatusOK, del.StatusCode)

	// without the cache purge this would still return 200 until the TTL expired
	s.Equal(http.StatusUnauthorized,
		Request(s.T(), s.App, "GET", "/api/private/user", "", bot.RawKey).StatusCode,
		"a deleted bot's cached key session must be purged immediately")
}

// Test_CreateBot_FailedAssign_RollsBackBot pins atomicity: when the project
// assignment fails (bots cannot be owners), the half-created bot must be rolled back.
func (s *AdminSuite) Test_CreateBot_FailedAssign_RollsBackBot() {
	idProject := createProject(s.T(), s.App, s.AdminToken, "tx-rollback-project")
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"tx rollback bot","isBot":true,"idProject":%d,"role":"owner"}`, idProject),
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

func Test_RunAdminSuite(t *testing.T) {
	suite.Run(t, new(AdminSuite))
}
