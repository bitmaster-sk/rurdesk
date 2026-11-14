package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/deactivation"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/stretchr/testify/require"
)

func TestDeactivation_SystemActorMayOrphanAProjectOwner(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "sole-owner@test.sk")
	ownerToken := createUserAsAdmin(t, app, admin, `{"name":"sole owner","email":"sole-owner@test.sk","password":"secret1"}`)
	idProject := createProject(t, app, ownerToken, "orphan-check")
	idOwner := idOfUser(t, app, admin, "sole-owner@test.sk")
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", idProject)
		deleteUsers(app, "sole-owner@test.sk")
	})
	svc := injector.GetDeactivationService()
	ctx := context.Background()

	_, err := svc.SetActive(ctx, idOwner, false, deactivation.Options{})
	require.ErrorContains(t, err, "last active owner")

	changed, err := svc.SetActive(ctx, idOwner, false, deactivation.Options{SkipLastOwnerGuard: true})
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, http.StatusUnauthorized, Request(t, app, "GET", "/api/private/user", "", ownerToken).StatusCode)
}

func TestDeactivation_LastAdminStaysGuardedForTheSystemActor(t *testing.T) {
	app := Setup(t)
	idAdmin := idOfUser(t, app, Token(t, app), "test@test.sk")
	t.Cleanup(func() {
		_, _ = app.Pool.Exec(context.Background(), "UPDATE users.user SET is_active = TRUE WHERE id_user = $1", idAdmin)
	})

	_, err := injector.GetDeactivationService().SetActive(context.Background(), idAdmin, false, deactivation.Options{SkipLastOwnerGuard: true})
	require.ErrorIs(t, err, errs.ErrLastAdmin)
}

type fakeChangeGuard struct{ deniedKind authpolicy.ChangeKind }

var _ authpolicy.UserChangeGuard = fakeChangeGuard{}

func (guard fakeChangeGuard) Check(_ context.Context, change authpolicy.UserChange) error {
	if change.Kind == guard.deniedKind {
		return errs.NewErr("GUARDED", "guarded", "error.guarded")
	}
	return nil
}

func TestDeactivation_UserChangeGuardBlocksAdminDisableDeleteAndDemote(t *testing.T) {
	app := Setup(t)
	admin := Token(t, app)
	deleteUsers(app, "guarded@test.sk")
	t.Cleanup(func() { deleteUsers(app, "guarded@test.sk") })
	createUserAsAdmin(t, app, admin, `{"name":"guarded","email":"guarded@test.sk","password":"secret1","isAdmin":true}`)
	idUser := idOfUser(t, app, admin, "guarded@test.sk")
	t.Cleanup(func() { injector.SetUserChangeGuard(authpolicy.Free{}) })
	userURL := fmt.Sprintf("/api/private/admin/user/%d", idUser)

	injector.SetUserChangeGuard(fakeChangeGuard{deniedKind: authpolicy.ChangeRemoveAdmin})
	res := Request(t, app, "PATCH", userURL, `{"isAdmin":false}`, admin)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Contains(t, readBody(t, res), "GUARDED")

	injector.SetUserChangeGuard(fakeChangeGuard{deniedKind: authpolicy.ChangeDeactivate})
	res = Request(t, app, "PATCH", userURL, `{"isActive":false}`, admin)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Contains(t, readBody(t, res), "GUARDED")

	injector.SetUserChangeGuard(fakeChangeGuard{deniedKind: authpolicy.ChangeDelete})
	res = Request(t, app, "DELETE", userURL, "", admin)
	require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	require.Contains(t, readBody(t, res), "GUARDED")

	injector.SetUserChangeGuard(authpolicy.Free{})
	require.Equal(t, http.StatusOK, Request(t, app, "PATCH", userURL, `{"isActive":false}`, admin).StatusCode)
}
