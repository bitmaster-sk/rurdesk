package test

import (
	"context"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const resolverIssuer = "https://idp.acme.test"

var resolverRules = login.Rules{AllowedDomains: []string{"acme.test"}}

func newResolver(t *testing.T) (*issue.Application, *login.Resolver) {
	app := Setup(t)
	resetSso(t, app)
	return app, login.NewResolver(app.Pool, injector.GetUserRepository(), newGroupSync(app))
}

func insertSsoUser(t *testing.T, email string, changes ...func(*model.User)) *model.User {
	user := &model.User{Name: "Jana Nováková", Email: email, Password: "hash", ColorAvatarBg: "#aabbcc"}
	for _, change := range changes {
		change(user)
	}
	inserted, err := injector.GetUserRepository().InsertUser(context.Background(), user)
	require.NoError(t, err)
	return inserted
}

func resolverIdentity(subject, email string) identity.Identity {
	return identity.Identity{Issuer: resolverIssuer, Subject: subject, Email: email, Name: "Jana Nováková"}
}

func identityCount(t *testing.T, app *issue.Application, idUser int64) int {
	var count int
	err := app.Pool.QueryRow(context.Background(), `SELECT count(*) FROM users.user_identity WHERE id_user = $1`, idUser).Scan(&count)
	require.NoError(t, err)
	return count
}

func TestSsoResolver_LinksAnExistingAccountRegardlessOfCase(t *testing.T) {
	app, resolver := newResolver(t)
	email := uniqueSsoEmail("link")
	existing := insertSsoUser(t, strings.ToUpper(email[:1])+email[1:])
	id := resolverIdentity("sub-link-"+email, email)

	outcome, err := resolver.Resolve(context.Background(), id, resolverRules)
	require.NoError(t, err)
	assert.Equal(t, login.ActionLink, outcome.Action)
	assert.Equal(t, existing.IdUser, outcome.User.IdUser)
	assert.Equal(t, 1, identityCount(t, app, existing.IdUser))

	again, err := resolver.Resolve(context.Background(), id, resolverRules)
	require.NoError(t, err)
	assert.Equal(t, login.ActionSignIn, again.Action)
	assert.Equal(t, existing.IdUser, again.User.IdUser)
}

func TestSsoResolver_CreatesAPasswordlessAccountWithAutoCreate(t *testing.T) {
	app, resolver := newResolver(t)
	email := uniqueSsoEmail("jit")
	rules := login.Rules{AllowedDomains: []string{"acme.test"}, IsAutoCreate: true}

	outcome, err := resolver.Resolve(context.Background(), resolverIdentity("sub-"+email, email), rules)

	require.NoError(t, err)
	assert.Equal(t, login.ActionCreate, outcome.Action)
	assert.Equal(t, email, outcome.User.Email)
	assert.Equal(t, "Jana Nováková", outcome.User.Name)
	assert.True(t, outcome.User.IsPasswordless)
	assert.True(t, outcome.User.IsActive)
	assert.NotEmpty(t, outcome.User.ColorAvatarBg)
	assert.Equal(t, 1, identityCount(t, app, outcome.User.IdUser))
}

func TestSsoResolver_RejectsAndChangesNothing(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, app *issue.Application, email string)
		reason login.Reason
	}{
		{name: "auto-create off", setup: func(*testing.T, *issue.Application, string) {}, reason: login.ReasonAccountNotProvisioned},
		{name: "agent", setup: func(t *testing.T, _ *issue.Application, email string) {
			insertSsoUser(t, email, func(u *model.User) { u.IsAgent = true })
		}, reason: login.ReasonAccountNotLinkable},
		{name: "identity from the same issuer", setup: func(t *testing.T, app *issue.Application, email string) {
			user := insertSsoUser(t, email)
			_, err := app.Pool.Exec(context.Background(), `INSERT INTO users.user_identity (id_user, issuer, subject) VALUES ($1, $2, 'other-subject')`, user.IdUser, resolverIssuer)
			require.NoError(t, err)
		}, reason: login.ReasonAccountNotLinkable},
		{name: "disabled account", setup: func(t *testing.T, _ *issue.Application, email string) {
			user := insertSsoUser(t, email)
			require.NoError(t, injector.GetUserRepository().SetActive(context.Background(), user.IdUser, false))
		}, reason: login.ReasonAccountDisabled},
		{name: "two accounts differing in case", setup: func(t *testing.T, _ *issue.Application, email string) {
			insertSsoUser(t, email)
			insertSsoUser(t, strings.ToUpper(email))
		}, reason: login.ReasonEmailAmbiguous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, resolver := newResolver(t)
			email := uniqueSsoEmail("reject")
			tt.setup(t, app, email)

			outcome, err := resolver.Resolve(context.Background(), resolverIdentity("sub-"+email, email), resolverRules)

			require.NoError(t, err)
			assert.Equal(t, login.ActionReject, outcome.Action)
			assert.Equal(t, tt.reason, outcome.Reason)
			var identities int
			require.NoError(t, app.Pool.QueryRow(context.Background(), `SELECT count(*) FROM users.user_identity WHERE subject = $1`, "sub-"+email).Scan(&identities))
			assert.Zero(t, identities)
		})
	}
}

func TestSsoResolver_UpdatesTheProfileOnSignIn(t *testing.T) {
	_, resolver := newResolver(t)
	ctx := context.Background()
	email := uniqueSsoEmail("profile")
	user := insertSsoUser(t, email)
	subject := "sub-" + email
	_, err := resolver.Resolve(ctx, resolverIdentity(subject, email), resolverRules)
	require.NoError(t, err)

	newEmail := uniqueSsoEmail("profile-new")
	renamed := resolverIdentity(subject, newEmail)
	renamed.Name = "Jana Malá"
	outcome, err := resolver.Resolve(ctx, renamed, resolverRules)
	require.NoError(t, err)
	assert.Equal(t, "Jana Malá", outcome.User.Name)
	assert.Equal(t, newEmail, outcome.User.Email)

	taken := insertSsoUser(t, uniqueSsoEmail("taken"))
	collision := resolverIdentity(subject, taken.Email)
	outcome, err = resolver.Resolve(ctx, collision, resolverRules)
	require.NoError(t, err)
	assert.Equal(t, newEmail, outcome.User.Email)

	stored, err := injector.GetUserRepository().LoadUser(ctx, user.IdUser)
	require.NoError(t, err)
	assert.Equal(t, newEmail, stored.Email)
}

func TestSsoResolver_PreviewChangesNothing(t *testing.T) {
	app, resolver := newResolver(t)
	email := uniqueSsoEmail("preview")
	user := insertSsoUser(t, email)

	outcome, err := resolver.Preview(context.Background(), resolverIdentity("sub-"+email, email), resolverRules)

	require.NoError(t, err)
	assert.Equal(t, login.ActionLink, outcome.Action)
	assert.Zero(t, identityCount(t, app, user.IdUser))
}
