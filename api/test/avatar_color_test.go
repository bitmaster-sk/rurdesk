package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// --- Admin create/edit (methods on AdminSuite, defined in admin_test.go) ---

func (s *AdminSuite) Test_CreateUser_HonorsSuppliedAvatarColor() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'colored@test.sk'")
	body := `{"name":"colored","email":"colored@test.sk","password":"secret1","colorAvatarBg":"#123456"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.Equal("#123456", out.ColorAvatarBg, "supplied colour must be used verbatim")
}

func (s *AdminSuite) Test_CreateUser_DefaultsToRandomAvatarColor() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'nocolor@test.sk'")
	body := `{"name":"nocolor","email":"nocolor@test.sk","password":"secret1"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.Regexp(`^#[0-9a-fA-F]{6}$`, out.ColorAvatarBg, "omitted colour must fall back to a random hex")
}

func (s *AdminSuite) Test_CreateBot_HonorsSuppliedAvatarColor() {
	body := `{"name":"Colored Bot","isBot":true,"colorAvatarBg":"#abcdef"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)

	var out model.AdminCreateUserRes
	s.Nil(json.NewDecoder(res.Body).Decode(&out))
	s.Equal("#abcdef", out.ColorAvatarBg)
}

func (s *AdminSuite) Test_CreateUser_RejectsInvalidAvatarColor() {
	body := `{"name":"badcolor","email":"badcolor@test.sk","password":"secret1","colorAvatarBg":"not-a-color"}`
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", body, s.AdminToken)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *AdminSuite) Test_UpdateUser_ChangesAvatarColor() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'recolor@test.sk'")
	create := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"recolor","email":"recolor@test.sk","password":"secret1","colorAvatarBg":"#111111"}`, s.AdminToken)
	s.Equal(http.StatusOK, create.StatusCode)
	var created model.AdminCreateUserRes
	s.Nil(json.NewDecoder(create.Body).Decode(&created))

	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", created.IdUser),
		`{"name":"recolor","email":"recolor@test.sk","colorAvatarBg":"#222222"}`, s.AdminToken)
	s.Equal(http.StatusOK, patch.StatusCode)

	s.Equal("#222222", adminUserColor(s, created.IdUser), "edit must persist the new colour")
}

func (s *AdminSuite) Test_UpdateUser_ColorEditKeepsSession() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE email = 'keepsess@test.sk'")
	token := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"keepsess","email":"keepsess@test.sk","password":"secret1"}`)
	idUser := idOfUser(s.T(), s.App, s.AdminToken, "keepsess@test.sk")

	// Admin edits the user's colour; isAdmin is sent (as the edit form does) but
	// unchanged, so the user's existing session must stay alive.
	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d", idUser),
		`{"name":"keepsess","email":"keepsess@test.sk","isAdmin":false,"colorAvatarBg":"#777777"}`,
		s.AdminToken)
	s.Equal(http.StatusOK, patch.StatusCode)

	me := Request(s.T(), s.App, "GET", "/api/private/user", "", token)
	s.Equal(http.StatusOK, me.StatusCode, "a profile/colour edit must not invalidate the user's token")
}

// adminUserColor reads a user's stored avatar colour back through the admin list.
func adminUserColor(s *AdminSuite, idUser int64) string {
	res := Request(s.T(), s.App, "GET", "/api/private/admin/user", "", s.AdminToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var users []map[string]any
	s.Nil(json.NewDecoder(res.Body).Decode(&users))
	for _, u := range users {
		if int64(u["idUser"].(float64)) == idUser {
			return u["colorAvatarBg"].(string)
		}
	}
	s.Failf("user not found", "idUser %d missing from admin list", idUser)
	return ""
}

// --- Self-service (methods on LoginSuite, defined in user_api_test.go) ---

func (suite *LoginSuite) Test_UpdateUser_SetsAvatarColor() {
	token := Token(suite.T(), suite.App)
	res := Request(suite.T(), suite.App, "PATCH", "/api/private/user",
		`{"name":"tester","colorAvatarBg":"#0a0b0c"}`, token)
	suite.Equal(http.StatusOK, res.StatusCode)

	var usr model.User
	suite.Nil(json.NewDecoder(res.Body).Decode(&usr))
	suite.Equal("#0a0b0c", usr.ColorAvatarBg)
}

func (suite *LoginSuite) Test_UpdateUser_RejectsInvalidAvatarColor() {
	token := Token(suite.T(), suite.App)
	res := Request(suite.T(), suite.App, "PATCH", "/api/private/user",
		`{"name":"tester","colorAvatarBg":"xyz"}`, token)
	suite.Equal(http.StatusBadRequest, res.StatusCode)
}

func (suite *LoginSuite) Test_UpdateUser_OmittedColorLeavesItUnchanged() {
	token := Token(suite.T(), suite.App)

	set := Request(suite.T(), suite.App, "PATCH", "/api/private/user",
		`{"name":"tester","colorAvatarBg":"#654321"}`, token)
	suite.Equal(http.StatusOK, set.StatusCode)

	// Name-only update must not wipe the colour.
	upd := Request(suite.T(), suite.App, "PATCH", "/api/private/user", `{"name":"tester"}`, token)
	suite.Equal(http.StatusOK, upd.StatusCode)
	var usr model.User
	suite.Nil(json.NewDecoder(upd.Body).Decode(&usr))
	suite.Equal("#654321", usr.ColorAvatarBg, "a name-only edit must preserve the avatar colour")
}
