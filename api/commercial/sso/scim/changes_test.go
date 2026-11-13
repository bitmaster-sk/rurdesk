package scim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func patch(t *testing.T, operations string) Changes {
	changes, err := ParsePatch([]byte(`{"schemas":["` + SchemaPatchOp + `"],"Operations":` + operations + `}`))
	require.NoError(t, err)
	return changes
}

func TestParsePatch_PathForms(t *testing.T) {
	changes := patch(t, `[
		{"op":"Replace","path":"active","value":"False"},
		{"op":"replace","path":"userName","value":"jana@acme.test"},
		{"op":"replace","path":"displayName","value":"Jana Nováková"},
		{"op":"add","path":"name.givenName","value":"Jana"},
		{"op":"add","path":"name.familyName","value":"Nováková"},
		{"op":"replace","path":"name.formatted","value":"J. Nováková"},
		{"op":"replace","path":"externalId","value":"ext-1"},
		{"op":"replace","path":"emails[type eq \"work\"].value","value":"Jana@Acme.test"},
		{"op":"replace","path":"title","value":"ignored"},
		{"op":"remove","path":"phoneNumbers"}
	]`)

	assert.Equal(t, false, *changes.Active)
	assert.Equal(t, "jana@acme.test", *changes.UserName)
	assert.Equal(t, "Jana Nováková", *changes.DisplayName)
	assert.Equal(t, "Jana", *changes.GivenName)
	assert.Equal(t, "Nováková", *changes.FamilyName)
	assert.Equal(t, "J. Nováková", *changes.Formatted)
	assert.Equal(t, "ext-1", *changes.ExternalId)
	assert.Equal(t, "Jana@Acme.test", *changes.WorkEmail)
	assert.True(t, changes.IsDeactivation())
}

func TestParsePatch_WithoutPath(t *testing.T) {
	changes := patch(t, `[
		{"op":"Replace","value":{"active":true,"name.givenName":"Jana","name":{"familyName":"Nová"}}},
		{"op":"Add","value":{"emails":[{"value":"jana@acme.test","type":"work"}]}}
	]`)

	assert.True(t, *changes.Active)
	name, ok := changes.Name()
	assert.True(t, ok)
	assert.Equal(t, "Jana Nová", name)
	address, ok := changes.Email()
	assert.True(t, ok)
	assert.Equal(t, "jana@acme.test", address)
}

func TestParsePatch_ActiveAsTextOrBoolean(t *testing.T) {
	for raw, expected := range map[string]bool{`true`: true, `false`: false, `"True"`: true, `"FALSE"`: false} {
		changes := patch(t, `[{"op":"replace","path":"active","value":`+raw+`}]`)
		assert.Equal(t, expected, *changes.Active, raw)
	}
}

func TestParsePatch_LaterOperationsWin(t *testing.T) {
	changes := patch(t, `[
		{"op":"replace","path":"active","value":false},
		{"op":"replace","path":"active","value":true}
	]`)

	assert.False(t, changes.IsDeactivation())
}

func TestParsePatch_RejectsBrokenOperations(t *testing.T) {
	for _, operations := range []string{
		`[{"op":"move","path":"active","value":true}]`,
		`[{"op":"replace","path":"active","value":"maybe"}]`,
		`[{"op":"replace","value":"not an object"}]`,
	} {
		_, err := ParsePatch([]byte(`{"Operations":` + operations + `}`))
		assert.Error(t, err, operations)
	}
}

func TestChanges_Email(t *testing.T) {
	text := func(value string) *string { return &value }
	tests := []struct {
		name     string
		changes  Changes
		expected string
	}{
		{name: "primary wins", changes: Changes{HasEmails: true, Emails: []Email{{Value: "home@x.test", Type: "home"}, {Value: "Main@Acme.test", Primary: true}}, UserName: text("upn@acme.test")}, expected: "main@acme.test"},
		{name: "work before the first", changes: Changes{HasEmails: true, Emails: []Email{{Value: "home@x.test", Type: "home"}, {Value: "work@acme.test", Type: "Work"}}}, expected: "work@acme.test"},
		{name: "first without type", changes: Changes{HasEmails: true, Emails: []Email{{Value: "first@acme.test"}}}, expected: "first@acme.test"},
		{name: "user name without emails", changes: Changes{UserName: text(" UPN@Acme.test ")}, expected: "upn@acme.test"},
		{name: "work e-mail path", changes: Changes{WorkEmail: text("w@acme.test"), UserName: text("upn@acme.test")}, expected: "w@acme.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, ok := tt.changes.Email()
			assert.True(t, ok)
			assert.Equal(t, tt.expected, address)
		})
	}
	_, ok := Changes{}.Email()
	assert.False(t, ok)
}

func TestChanges_Name(t *testing.T) {
	text := func(value string) *string { return &value }
	tests := []struct {
		changes  Changes
		expected string
		ok       bool
	}{
		{changes: Changes{DisplayName: text("Jana N."), Formatted: text("Jana Nováková")}, expected: "Jana N.", ok: true},
		{changes: Changes{Formatted: text("Jana Nováková"), GivenName: text("J")}, expected: "Jana Nováková", ok: true},
		{changes: Changes{GivenName: text("Jana"), FamilyName: text("Nováková")}, expected: "Jana Nováková", ok: true},
		{changes: Changes{FamilyName: text("Nováková")}, expected: "Nováková", ok: true},
		{changes: Changes{DisplayName: text(" ")}, ok: false},
		{changes: Changes{IsPartial: true, FamilyName: text("Nováková")}, ok: false},
		{changes: Changes{IsPartial: true, GivenName: text("Jana")}, ok: false},
		{changes: Changes{IsPartial: true, GivenName: text("Jana"), FamilyName: text("Nováková")}, expected: "Jana Nováková", ok: true},
		{changes: Changes{IsPartial: true, DisplayName: text("Jana N."), FamilyName: text("Nováková")}, expected: "Jana N.", ok: true},
	}
	for _, tt := range tests {
		name, ok := tt.changes.Name()
		assert.Equal(t, tt.ok, ok)
		assert.Equal(t, tt.expected, name)
	}
}

func TestParseResource(t *testing.T) {
	changes, err := ParseResource([]byte(`{
		"schemas":["` + SchemaUser + `"],
		"externalId":"ext-9",
		"userName":"upn@acme.test",
		"active":true,
		"displayName":"Jana",
		"emails":[{"primary":true,"type":"work","value":"jana@acme.test"}],
		"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User":{"department":"R&D"}
	}`))
	require.NoError(t, err)

	address, _ := changes.Email()
	assert.Equal(t, "jana@acme.test", address)
	assert.Equal(t, "ext-9", *changes.ExternalId)
	assert.True(t, *changes.Active)
}
