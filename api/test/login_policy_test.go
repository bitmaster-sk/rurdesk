package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

type fakeLoginPolicy struct{ deniedEmail string }

var _ authpolicy.PasswordLoginPolicy = fakeLoginPolicy{}

func (policy fakeLoginPolicy) Allow(_ context.Context, user *model.User) bool {
	return user.Email != policy.deniedEmail
}

func loginStatus(t *testing.T, app *issue.Application, email, password string) int {
	return Request(t, app, "POST", "/api/public/login",
		fmt.Sprintf(`{"email":%q,"password":%q}`, email, password), "").StatusCode
}

func storedEmail(t *testing.T, app *issue.Application, adminToken, email string) string {
	res := Request(t, app, "GET", "/api/private/admin/user", "", adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var users []model.User
	require.NoError(t, json.NewDecoder(res.Body).Decode(&users))
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u.Email
		}
	}
	t.Fatalf("user %s not found", email)
	return ""
}

func deleteUsers(app *issue.Application, emails ...string) {
	for _, email := range emails {
		_, _ = app.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE lower(email) = lower($1)", email)
	}
}

func TestLogin_EmailIsCaseInsensitive(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "mixed.case@test.sk")
	t.Cleanup(func() { deleteUsers(app, "mixed.case@test.sk") })

	res := Request(t, app, "POST", "/api/private/admin/user",
		`{"name":"mixed","email":"Mixed.Case@Test.SK","password":"secret1"}`, admin)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "mixed.case@test.sk", storedEmail(t, app, admin, "mixed.case@test.sk"))

	require.Equal(t, http.StatusOK, loginStatus(t, app, "MIXED.case@test.sk", "secret1"))

	dup := Request(t, app, "POST", "/api/private/admin/user",
		`{"name":"dup","email":"mixed.CASE@test.sk","password":"secret1"}`, admin)
	require.Equal(t, http.StatusConflict, dup.StatusCode)
}

func TestLogin_PasswordlessAccountLooksLikeAWrongPassword(t *testing.T) {
	app := Setup(t)
	deleteUsers(app, "nopass@test.sk")
	t.Cleanup(func() { deleteUsers(app, "nopass@test.sk") })
	_, err := app.Pool.Exec(context.Background(),
		"INSERT INTO users.user (email, name, color_avatar_bg) VALUES ('nopass@test.sk', 'no pass', '#123456')")
	require.NoError(t, err)

	res := Request(t, app, "POST", "/api/public/login", `{"email":"nopass@test.sk","password":"anything"}`, "")
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	wrong := Request(t, app, "POST", "/api/public/login", `{"email":"test@test.sk","password":"wrong-one"}`, "")
	require.Equal(t, readBody(t, wrong), readBody(t, res))
}

func TestLogin_PolicyRejectsWithTheWrongPasswordError(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "blocked@test.sk")
	t.Cleanup(func() { deleteUsers(app, "blocked@test.sk") })
	createUserAsAdmin(t, app, admin, `{"name":"blocked","email":"blocked@test.sk","password":"secret1"}`)
	injector.SetPasswordLoginPolicy(fakeLoginPolicy{deniedEmail: "blocked@test.sk"})
	t.Cleanup(func() { injector.SetPasswordLoginPolicy(authpolicy.Free{}) })

	blocked := Request(t, app, "POST", "/api/public/login", `{"email":"blocked@test.sk","password":"secret1"}`, "")
	require.Equal(t, http.StatusUnauthorized, blocked.StatusCode)
	wrong := Request(t, app, "POST", "/api/public/login", `{"email":"test@test.sk","password":"wrong-one"}`, "")
	require.Equal(t, readBody(t, wrong), readBody(t, blocked))
	require.Equal(t, http.StatusOK, loginStatus(t, app, "test@test.sk", "kreslo"))
}

func TestLogin_RecordsLastLogin(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	res := Request(t, app, "GET", "/api/private/admin/user", "", admin)
	var users []model.User
	require.NoError(t, json.NewDecoder(res.Body).Decode(&users))
	for _, u := range users {
		if u.Email == "test@test.sk" {
			require.NotNil(t, u.LastLoginAt)
			return
		}
	}
	t.Fatal("bootstrap admin missing")
}

func TestAdminEmailEdit_IsNormalizedAndCaseInsensitivelyUnique(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "first-edit@test.sk", "second-edit@test.sk", "second.new@test.sk")
	t.Cleanup(func() { deleteUsers(app, "first-edit@test.sk", "second-edit@test.sk", "second.new@test.sk") })
	createUserAsAdmin(t, app, admin, `{"name":"first","email":"first-edit@test.sk","password":"secret1"}`)
	createUserAsAdmin(t, app, admin, `{"name":"second","email":"second-edit@test.sk","password":"secret1"}`)
	idSecond := idOfUser(t, app, admin, "second-edit@test.sk")

	res := patchUser(t, app, admin, idSecond, `{"name":"second","email":"FIRST-edit@test.sk"}`)
	require.Equal(t, http.StatusConflict, res.StatusCode)

	res = patchUser(t, app, admin, idSecond, `{"name":"second","email":"Second.New@Test.sk"}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "second.new@test.sk", adminUser(t, app, admin, idSecond).Email)
}
