package login

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/stretchr/testify/assert"
)

func TestNextProfile(t *testing.T) {
	rules := Rules{AllowedDomains: []string{"acme.test"}}
	user := newUser()

	tests := []struct {
		name         string
		id           identity.Identity
		isEmailTaken bool
		expected     Profile
	}{
		{name: "new name and e-mail", id: newIdentity(func(id *identity.Identity) { id.Name = "Jana Malá"; id.Email = "jana.mala@acme.test" }), expected: Profile{Name: "Jana Malá", Email: "jana.mala@acme.test"}},
		{name: "empty name keeps the old one", id: newIdentity(func(id *identity.Identity) { id.Name = "  " }), expected: Profile{Name: "Jana Nováková", Email: "jana@acme.test"}},
		{name: "e-mail outside the domains", id: newIdentity(func(id *identity.Identity) { id.Email = "jana@other.test" }), expected: Profile{Name: "Jana Nováková", Email: "jana@acme.test"}},
		{name: "e-mail of another account", id: newIdentity(func(id *identity.Identity) { id.Email = "eva@acme.test" }), isEmailTaken: true, expected: Profile{Name: "Jana Nováková", Email: "jana@acme.test"}},
		{name: "unverified e-mail", id: newIdentity(func(id *identity.Identity) { id.Email = "jana2@acme.test"; id.EmailVerified = verified(false) }), expected: Profile{Name: "Jana Nováková", Email: "jana@acme.test"}},
		{name: "missing e-mail", id: newIdentity(func(id *identity.Identity) { id.Email = "" }), expected: Profile{Name: "Jana Nováková", Email: "jana@acme.test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NextProfile(tt.id, user, rules, tt.isEmailTaken))
		})
	}
}

func TestNewUserName(t *testing.T) {
	assert.Equal(t, "Jana Nováková", NewUserName(newIdentity()))
	assert.Equal(t, "jana", NewUserName(newIdentity(func(id *identity.Identity) { id.Name = "" })))

	long := NewUserName(newIdentity(func(id *identity.Identity) { id.Name = strings.Repeat("ž", 300) }))
	assert.Equal(t, 250, utf8.RuneCountInString(long))
}
