package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapClaims(t *testing.T) {
	id := MapClaims(map[string]any{
		"iss":            "https://idp.acme.test",
		"sub":            "sub-1",
		"email":          " Jana@ACME.test ",
		"email_verified": "true",
		"hd":             "ACME.test",
		"given_name":     "Jana",
		"family_name":    "Nováková",
		"groups":         []any{"dev", "", 5, "ops"},
	}, "email", "groups")

	assert.Equal(t, "https://idp.acme.test", id.Issuer)
	assert.Equal(t, "sub-1", id.Subject)
	assert.Equal(t, "jana@acme.test", id.Email)
	assert.Equal(t, true, *id.EmailVerified)
	assert.Equal(t, "acme.test", id.HostedDomain)
	assert.Equal(t, "Jana Nováková", id.Name)
	assert.Equal(t, []string{"dev", "ops"}, id.Groups)
	assert.True(t, id.HasGroupsClaim)
	assert.False(t, id.HasGroupsOverage)
}

func TestMapClaims_ConfigurableClaimsAndEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
		check  func(t *testing.T, claims map[string]any)
	}{
		{name: "email claim name", claims: map[string]any{"preferred_username": "Jana@Acme.test", "email": "other@acme.test"}, check: func(t *testing.T, claims map[string]any) {
			assert.Equal(t, "jana@acme.test", MapClaims(claims, "preferred_username", "groups").Email)
		}},
		{name: "groups claim name and single value", claims: map[string]any{"roles": "admin"}, check: func(t *testing.T, claims map[string]any) {
			id := MapClaims(claims, "email", "roles")
			assert.Equal(t, []string{"admin"}, id.Groups)
			assert.True(t, id.HasGroupsClaim)
		}},
		{name: "empty groups list", claims: map[string]any{"groups": []any{}}, check: func(t *testing.T, claims map[string]any) {
			id := MapClaims(claims, "email", "groups")
			assert.Equal(t, []string{}, id.Groups)
			assert.True(t, id.HasGroupsClaim)
		}},
		{name: "missing groups", claims: map[string]any{}, check: func(t *testing.T, claims map[string]any) {
			id := MapClaims(claims, "email", "groups")
			assert.Nil(t, id.Groups)
			assert.False(t, id.HasGroupsClaim)
			assert.Nil(t, id.EmailVerified)
		}},
		{name: "entra overage", claims: map[string]any{"_claim_names": map[string]any{"groups": "src1"}}, check: func(t *testing.T, claims map[string]any) {
			id := MapClaims(claims, "email", "groups")
			assert.True(t, id.HasGroupsOverage)
			assert.False(t, id.HasGroupsClaim)
		}},
		{name: "email_verified as false string", claims: map[string]any{"email_verified": "False"}, check: func(t *testing.T, claims map[string]any) {
			assert.Equal(t, false, *MapClaims(claims, "email", "groups").EmailVerified)
		}},
		{name: "name claim wins", claims: map[string]any{"name": "J. N.", "given_name": "Jana"}, check: func(t *testing.T, claims map[string]any) {
			assert.Equal(t, "J. N.", MapClaims(claims, "email", "groups").Name)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.claims)
		})
	}
}

func TestFillFromUserinfo_FillsOnlyMissingData(t *testing.T) {
	id := MapClaims(map[string]any{"sub": "sub-1", "name": "Jana"}, "email", "groups")

	filled := FillFromUserinfo(id, map[string]any{
		"sub":            "sub-1",
		"name":           "Other",
		"email":          "Jana@Acme.test",
		"email_verified": true,
		"groups":         []any{"dev"},
	}, "email", "groups")

	assert.Equal(t, "Jana", filled.Name)
	assert.Equal(t, "jana@acme.test", filled.Email)
	assert.Equal(t, true, *filled.EmailVerified)
	assert.Equal(t, []string{"dev"}, filled.Groups)
	assert.True(t, filled.HasGroupsClaim)
}
