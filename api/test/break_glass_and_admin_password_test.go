package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func patchUser(t *testing.T, app *issue.Application, token string, idUser int64, body string) *http.Response {
	return Request(t, app, "PATCH", fmt.Sprintf("/api/private/admin/user/%d", idUser), body, token)
}

func adminUser(t *testing.T, app *issue.Application, token string, idUser int64) model.User {
	res := Request(t, app, "GET", "/api/private/admin/user", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var users []model.User
	require.NoError(t, json.NewDecoder(res.Body).Decode(&users))
	for _, u := range users {
		if u.IdUser == idUser {
			return u
		}
	}
	t.Fatalf("user %d not found", idUser)
	return model.User{}
}

func setPassword(t *testing.T, app *issue.Application, token string, idUser int64, password string) *http.Response {
	return Request(t, app, "PUT", fmt.Sprintf("/api/private/admin/user/%d/password", idUser),
		fmt.Sprintf(`{"password":%q}`, password), token)
}

func TestBreakGlass_OnlyForAnActiveAdminWithPasswordAndNoIdentity(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "bg-plain@test.sk", "bg-admin@test.sk")
	t.Cleanup(func() { deleteUsers(app, "bg-plain@test.sk", "bg-admin@test.sk") })
	createUserAsAdmin(t, app, admin, `{"name":"bg plain","email":"bg-plain@test.sk","password":"secret1"}`)
	createUserAsAdmin(t, app, admin, `{"name":"bg admin","email":"bg-admin@test.sk","password":"secret1","isAdmin":true}`)
	idPlain := idOfUser(t, app, admin, "bg-plain@test.sk")
	idAdmin := idOfUser(t, app, admin, "bg-admin@test.sk")

	res := patchUser(t, app, admin, idPlain, `{"isBreakGlass":true}`)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Contains(t, readBody(t, res), "BREAK_GLASS_NOT_ALLOWED")

	require.Equal(t, http.StatusOK, patchUser(t, app, admin, idAdmin, `{"isBreakGlass":true}`).StatusCode)
	require.True(t, adminUser(t, app, admin, idAdmin).IsBreakGlass)

	require.Equal(t, http.StatusOK, patchUser(t, app, admin, idAdmin, `{"isAdmin":false}`).StatusCode)
	require.False(t, adminUser(t, app, admin, idAdmin).IsBreakGlass, "demotion clears the flag")

	require.Equal(t, http.StatusOK, patchUser(t, app, admin, idAdmin, `{"isAdmin":true,"isBreakGlass":true}`).StatusCode)
	require.True(t, adminUser(t, app, admin, idAdmin).IsBreakGlass)
	require.Equal(t, http.StatusOK, patchUser(t, app, admin, idAdmin, `{"isActive":false}`).StatusCode)
	require.False(t, adminUser(t, app, admin, idAdmin).IsBreakGlass, "disabling clears the flag")

	require.Equal(t, http.StatusOK, patchUser(t, app, admin, idAdmin, `{"isActive":true}`).StatusCode)
	_, err := app.Pool.Exec(context.Background(),
		"INSERT INTO users.user_identity (id_user, issuer, subject) VALUES ($1, 'https://idp', 'bg-sub')", idAdmin)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, patchUser(t, app, admin, idAdmin, `{"isBreakGlass":true}`).StatusCode)

	_, err = app.Pool.Exec(context.Background(), "DELETE FROM users.user_identity WHERE id_user = $1", idAdmin)
	require.NoError(t, err)
	_, err = app.Pool.Exec(context.Background(), "UPDATE users.user SET password = NULL WHERE id_user = $1", idAdmin)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, patchUser(t, app, admin, idAdmin, `{"isBreakGlass":true}`).StatusCode)
}

func TestAdminSetPassword(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "reset-me@test.sk")
	t.Cleanup(func() { deleteUsers(app, "reset-me@test.sk") })
	userToken := createUserAsAdmin(t, app, admin, `{"name":"reset me","email":"reset-me@test.sk","password":"secret1"}`)
	idUser := idOfUser(t, app, admin, "reset-me@test.sk")

	require.Equal(t, http.StatusOK, setPassword(t, app, admin, idUser, "brand-new").StatusCode)
	require.Equal(t, http.StatusUnauthorized, Request(t, app, "GET", "/api/private/user", "", userToken).StatusCode)
	require.Equal(t, http.StatusOK, loginStatus(t, app, "reset-me@test.sk", "brand-new"))

	_, err := app.Pool.Exec(context.Background(), "UPDATE users.user SET password = NULL WHERE id_user = $1", idUser)
	require.NoError(t, err)
	require.True(t, adminUser(t, app, admin, idUser).IsPasswordless)
	require.Equal(t, http.StatusOK, setPassword(t, app, admin, idUser, "after-sso").StatusCode)
	require.False(t, adminUser(t, app, admin, idUser).IsPasswordless)
	require.Equal(t, http.StatusOK, loginStatus(t, app, "reset-me@test.sk", "after-sso"))

	idSelf := idOfUser(t, app, admin, "test@test.sk")
	require.Equal(t, http.StatusOK, setPassword(t, app, admin, idSelf, "kreslo").StatusCode)
	require.Equal(t, http.StatusOK, Request(t, app, "GET", "/api/private/user", "", admin).StatusCode,
		"the acting session survives")

	require.Equal(t, http.StatusBadRequest, setPassword(t, app, admin, idUser, "abc").StatusCode)
	require.Equal(t, http.StatusNotFound, setPassword(t, app, admin, 999999999, "secret1").StatusCode)
}

func TestAdminSetPassword_RejectsAgents(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	res := Request(t, app, "POST", "/api/private/admin/user", `{"name":"pw agent","isAgent":true}`, admin)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.AdminCreateUserRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&created))
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", created.IdUser)
	})

	res = setPassword(t, app, admin, created.IdUser, "secret1")
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Contains(t, readBody(t, res), "AGENT_PASSWORD")
}
