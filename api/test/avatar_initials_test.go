package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// --- Admin create/edit (methods on AdminSuite, defined in admin_test.go) ---

func (s *AdminSuite) Test_CreateUser_StoresAvatarInitialsUppercase() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'initials@test.sk'")
	body := `{"name":"Initials User","email":"initials@test.sk","password":"secret1","avatarInitials":"ab"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.Require().NotNil(out.AvatarInitials, "initials must be stored")
	s.Equal("AB", *out.AvatarInitials, "initials must be stored uppercased")

	// A lower-case value with a leading space is trimmed + uppercased server-side
	// (binding max=2 counts runes of the raw value, so the padding must fit).
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'initials2@test.sk'")
	body = `{"name":"Initials Two","email":"initials2@test.sk","password":"secret1","avatarInitials":" c"}`
	res = Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.Require().NotNil(out.AvatarInitials)
	s.Equal("C", *out.AvatarInitials, "initials must be trimmed and uppercased")
}

func (s *AdminSuite) Test_CreateUser_RejectsAvatarInitialsLongerThanTwo() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'longinit@test.sk'")
	body := `{"name":"longinit","email":"longinit@test.sk","password":"secret1","avatarInitials":"abc"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *AdminSuite) Test_CreateUser_WithoutAvatarInitialsLeavesThemNull() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'noinit@test.sk'")
	body := `{"name":"noinit","email":"noinit@test.sk","password":"secret1"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.Nil(out.AvatarInitials, "omitted initials must stay NULL (auto-computed from the name)")
}

func (s *AdminSuite) Test_UpdateUser_NameOnlyEditKeepsAvatarInitials() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'keepinit@test.sk'")
	create := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"Keep Init","email":"keepinit@test.sk","password":"secret1","avatarInitials":"ki"}`, s.AdminToken)
	s.Equal(http.StatusOK, create.StatusCode)
	var created model.AdminCreateUserRes
	s.Nil(json.NewDecoder(create.Body).Decode(&created))

	// Name-only edit: no avatarInitials in the body must not wipe them.
	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", created.IdUser),
		`{"name":"Keep Init Renamed","email":"keepinit@test.sk"}`, s.AdminToken)
	s.Equal(http.StatusOK, patch.StatusCode)

	s.Equal("KI", adminUserInitials(s, created.IdUser), "a name-only edit must not wipe the initials")
}

func (s *AdminSuite) Test_UpdateUser_EmptyAvatarInitialsClearsToAuto() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'clearinit@test.sk'")
	create := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"Clear Init","email":"clearinit@test.sk","password":"secret1","avatarInitials":"ci"}`, s.AdminToken)
	s.Equal(http.StatusOK, create.StatusCode)
	var created model.AdminCreateUserRes
	s.Nil(json.NewDecoder(create.Body).Decode(&created))

	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", created.IdUser),
		`{"name":"Clear Init","email":"clearinit@test.sk","avatarInitials":""}`, s.AdminToken)
	s.Equal(http.StatusOK, patch.StatusCode)

	s.Equal("", adminUserInitials(s, created.IdUser), "an explicit empty string must clear to auto (NULL)")
}

// adminUserInitials reads a user's stored avatar initials back through the admin
// list; "" means NULL (auto-computed from the name).
func adminUserInitials(s *AdminSuite, idUser int64) string {
	res := Request(s.T(), s.App, "GET", "/api/private/admin/user", "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var users []map[string]any
	s.Nil(json.NewDecoder(res.Body).Decode(&users))
	for _, u := range users {
		if int64(u["idUser"].(float64)) == idUser {
			if init, ok := u["avatarInitials"].(string); ok {
				return init
			}
			return ""
		}
	}
	s.Failf("user not found", "idUser %d missing from admin list", idUser)
	return ""
}

// --- Self-service (methods on LoginSuite, defined in user_api_test.go) ---

func (suite *LoginSuite) Test_UpdateUser_SetsAvatarInitials() {
	token := Token(suite.T(), suite.App)
	res := Request(suite.T(), suite.App, "PATCH", "/api/private/user",
		`{"name":"tester","avatarInitials":"tt"}`, token)
	suite.Equal(http.StatusOK, res.StatusCode)

	var usr model.User
	suite.Nil(json.NewDecoder(res.Body).Decode(&usr))
	suite.Require().NotNil(usr.AvatarInitials)
	suite.Equal("TT", *usr.AvatarInitials)
}

func (suite *LoginSuite) Test_UpdateUser_RejectsAvatarInitialsLongerThanTwo() {
	token := Token(suite.T(), suite.App)
	res := Request(suite.T(), suite.App, "PATCH", "/api/private/user",
		`{"name":"tester","avatarInitials":"toolong"}`, token)
	suite.Equal(http.StatusBadRequest, res.StatusCode)
}

func (suite *LoginSuite) Test_UpdateUser_OmittedInitialsLeaveThemUnchanged() {
	token := Token(suite.T(), suite.App)

	set := Request(suite.T(), suite.App, "PATCH", "/api/private/user",
		`{"name":"tester","avatarInitials":"tt"}`, token)
	suite.Equal(http.StatusOK, set.StatusCode)

	// Name-only update must not wipe the initials.
	upd := Request(suite.T(), suite.App, "PATCH", "/api/private/user", `{"name":"tester"}`, token)
	suite.Equal(http.StatusOK, upd.StatusCode)
	var usr model.User
	suite.Nil(json.NewDecoder(upd.Body).Decode(&usr))
	suite.Require().NotNil(usr.AvatarInitials)
	suite.Equal("TT", *usr.AvatarInitials, "a name-only edit must preserve the avatar initials")
}
