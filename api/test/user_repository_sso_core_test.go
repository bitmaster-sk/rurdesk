package test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_PasswordlessAndFoldedEmail(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	repo := injector.GetUserRepository()
	cleanup := func() {
		_, _ = app.Pool.Exec(ctx, "DELETE FROM users.user WHERE lower(email) IN ('sso-only@test.sk', 'legacy@test.sk')")
	}
	cleanup()
	t.Cleanup(cleanup)

	created, err := repo.InsertUser(ctx, &model.User{Email: "sso-only@test.sk", Name: "sso only", ColorAvatarBg: "#123456"})
	require.NoError(t, err)

	loaded, err := repo.LoadUser(ctx, created.IdUser)
	require.NoError(t, err)
	require.True(t, loaded.IsPasswordless)
	require.Equal(t, "", loaded.Password)
	require.Nil(t, loaded.LastLoginAt)

	require.NoError(t, repo.TouchLastLogin(ctx, created.IdUser))
	loaded, err = repo.LoadUser(ctx, created.IdUser)
	require.NoError(t, err)
	require.NotNil(t, loaded.LastLoginAt)

	_, err = app.Pool.Exec(ctx, "INSERT INTO users.user (email, password, name, color_avatar_bg) VALUES ('Legacy@Test.sk', 'x', 'legacy', '#123456')")
	require.NoError(t, err)

	folded, err := repo.LoadUsersByEmailFold(ctx, "LEGACY@test.SK")
	require.NoError(t, err)
	require.Len(t, folded, 1)
	require.False(t, folded[0].IsPasswordless)

	exists, err := repo.EmailExists(ctx, "legacy@test.sk")
	require.NoError(t, err)
	require.True(t, exists)

	taken, err := repo.EmailTakenByOther(ctx, "legacy@test.sk", created.IdUser)
	require.NoError(t, err)
	require.True(t, taken)
	taken, err = repo.EmailTakenByOther(ctx, "sso-only@test.sk", created.IdUser)
	require.NoError(t, err)
	require.False(t, taken)

	hasIdentity, err := repo.HasIdentity(ctx, created.IdUser)
	require.NoError(t, err)
	require.False(t, hasIdentity)
	_, err = app.Pool.Exec(ctx, "INSERT INTO users.user_identity (id_user, issuer, subject) VALUES ($1, 'https://idp', 'sub-1')", created.IdUser)
	require.NoError(t, err)
	hasIdentity, err = repo.HasIdentity(ctx, created.IdUser)
	require.NoError(t, err)
	require.True(t, hasIdentity)

	require.NoError(t, repo.SetBreakGlass(ctx, created.IdUser, true))
	loaded, err = repo.LoadUser(ctx, created.IdUser)
	require.NoError(t, err)
	require.True(t, loaded.IsBreakGlass)
}
