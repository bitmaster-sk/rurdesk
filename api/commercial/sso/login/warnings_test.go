package login

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/stretchr/testify/assert"
)

func TestWarnings(t *testing.T) {
	rules := Rules{AllowedDomains: []string{"acme.test"}}
	withGroups := func(id *identity.Identity) { id.HasGroupsClaim = true }

	tests := []struct {
		name     string
		id       identity.Identity
		expected []Warning
	}{
		{name: "none", id: newIdentity(withGroups), expected: []Warning{}},
		{name: "missing e-mail", id: newIdentity(withGroups, func(id *identity.Identity) { id.Email = "" }), expected: []Warning{WarningEmailMissing}},
		{name: "foreign domain", id: newIdentity(withGroups, func(id *identity.Identity) { id.Email = "jana@other.test" }), expected: []Warning{WarningDomainNotAllowed}},
		{name: "google without hd", id: newIdentity(withGroups, func(id *identity.Identity) { id.Issuer = GoogleIssuer }), expected: []Warning{WarningGoogleHdMissing}},
		{name: "unverified e-mail", id: newIdentity(withGroups, func(id *identity.Identity) { id.EmailVerified = verified(false) }), expected: []Warning{WarningEmailNotVerified}},
		{name: "missing groups claim", id: newIdentity(), expected: []Warning{WarningGroupsClaimMissing}},
		{name: "unstable saml subject", id: newIdentity(withGroups, func(id *identity.Identity) { id.IsSubjectUnstable = true }), expected: []Warning{WarningSubjectUnstable}},
		{name: "groups overage", id: newIdentity(func(id *identity.Identity) { id.HasGroupsOverage = true }), expected: []Warning{WarningGroupsOverage}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Warnings(tt.id, rules))
		})
	}
}

func TestTestResults_HaveNonNilLists(t *testing.T) {
	succeeded := SucceededTest(newIdentity(), Outcome{Action: ActionCreate}, Rules{AllowedDomains: []string{"acme.test"}})
	assert.True(t, succeeded.IsSuccess)
	assert.NotNil(t, succeeded.Groups)
	assert.Equal(t, ActionCreate, succeeded.Action)

	failed := FailedTest(TestErrorTokenInvalid, "bad signature")
	assert.False(t, failed.IsSuccess)
	assert.NotNil(t, failed.Groups)
	assert.NotNil(t, failed.Warnings)
	assert.Equal(t, "bad signature", failed.Detail)
}
