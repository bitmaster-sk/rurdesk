package login

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
)

func newUser(changes ...func(*model.User)) *model.User {
	user := &model.User{IdUser: 7, Name: "Jana Nováková", Email: "jana@acme.test", IsActive: true}
	for _, change := range changes {
		change(user)
	}
	return user
}

func verified(isVerified bool) *bool {
	return &isVerified
}

func newIdentity(changes ...func(*identity.Identity)) identity.Identity {
	id := identity.Identity{Issuer: "https://idp.acme.test", Subject: "sub-1", Email: "jana@acme.test", Name: "Jana Nováková"}
	for _, change := range changes {
		change(&id)
	}
	return id
}

func TestDecide(t *testing.T) {
	rules := Rules{AllowedDomains: []string{"acme.test"}}
	autoCreate := Rules{AllowedDomains: []string{"acme.test"}, IsAutoCreate: true}
	match := func(user *model.User) Candidates { return Candidates{ByEmail: []Candidate{{User: user}}} }
	google := func(id *identity.Identity) { id.Issuer = GoogleIssuer }

	tests := []struct {
		name       string
		id         identity.Identity
		candidates Candidates
		rules      Rules
		action     Action
		reason     Reason
	}{
		{name: "linked identity skips the domain check", id: newIdentity(func(id *identity.Identity) { id.Email = "jana@other.test" }), candidates: Candidates{Linked: newUser()}, rules: rules, action: ActionSignIn},
		{name: "linked identity of a disabled account", id: newIdentity(), candidates: Candidates{Linked: newUser(func(u *model.User) { u.IsActive = false })}, rules: rules, action: ActionReject, reason: ReasonAccountDisabled},
		{name: "missing e-mail", id: newIdentity(func(id *identity.Identity) { id.Email = "" }), candidates: match(newUser()), rules: rules, action: ActionReject, reason: ReasonEmailMissing},
		{name: "domain not allowed", id: newIdentity(func(id *identity.Identity) { id.Email = "jana@other.test" }), rules: autoCreate, action: ActionReject, reason: ReasonDomainNotAllowed},
		{name: "empty domain list", id: newIdentity(), rules: Rules{IsAutoCreate: true}, action: ActionReject, reason: ReasonDomainNotAllowed},
		{name: "google without hd", id: newIdentity(google), candidates: match(newUser()), rules: rules, action: ActionReject, reason: ReasonDomainNotAllowed},
		{name: "google with a foreign hd", id: newIdentity(google, func(id *identity.Identity) { id.HostedDomain = "other.test" }), candidates: match(newUser()), rules: rules, action: ActionReject, reason: ReasonDomainNotAllowed},
		{name: "google with an allowed hd", id: newIdentity(google, func(id *identity.Identity) { id.HostedDomain = "acme.test" }), candidates: match(newUser()), rules: rules, action: ActionLink},
		{name: "e-mail not verified", id: newIdentity(func(id *identity.Identity) { id.EmailVerified = verified(false) }), candidates: match(newUser()), rules: rules, action: ActionReject, reason: ReasonEmailNotVerified},
		{name: "e-mail verified", id: newIdentity(func(id *identity.Identity) { id.EmailVerified = verified(true) }), candidates: match(newUser()), rules: rules, action: ActionLink},
		{name: "more matches", id: newIdentity(), candidates: Candidates{ByEmail: []Candidate{{User: newUser()}, {User: newUser(func(u *model.User) { u.IdUser = 8 })}}}, rules: rules, action: ActionReject, reason: ReasonEmailAmbiguous},
		{name: "agent", id: newIdentity(), candidates: match(newUser(func(u *model.User) { u.IsAgent = true })), rules: rules, action: ActionReject, reason: ReasonAccountNotLinkable},
		{name: "break-glass", id: newIdentity(), candidates: match(newUser(func(u *model.User) { u.IsBreakGlass = true })), rules: rules, action: ActionReject, reason: ReasonAccountNotLinkable},
		{name: "identity from the same issuer", id: newIdentity(), candidates: Candidates{ByEmail: []Candidate{{User: newUser(), HasIssuerIdentity: true}}}, rules: rules, action: ActionReject, reason: ReasonAccountNotLinkable},
		{name: "disabled match", id: newIdentity(), candidates: match(newUser(func(u *model.User) { u.IsActive = false })), rules: rules, action: ActionReject, reason: ReasonAccountDisabled},
		{name: "no match with auto-create", id: newIdentity(), rules: autoCreate, action: ActionCreate},
		{name: "no match without auto-create", id: newIdentity(), rules: rules, action: ActionReject, reason: ReasonAccountNotProvisioned},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := Decide(tt.id, tt.candidates, tt.rules)

			assert.Equal(t, tt.action, outcome.Action)
			assert.Equal(t, tt.reason, outcome.Reason)
		})
	}
}

func TestDecide_ReturnsTheUserToSignInOrLink(t *testing.T) {
	user := newUser()
	rules := Rules{AllowedDomains: []string{"acme.test"}}

	assert.Same(t, user, Decide(newIdentity(), Candidates{Linked: user}, rules).User)
	assert.Same(t, user, Decide(newIdentity(), Candidates{ByEmail: []Candidate{{User: user}}}, rules).User)
}
