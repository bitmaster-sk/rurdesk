package oidc

import (
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/email"
)

func MapClaims(claims map[string]any, emailClaim, groupsClaim string) identity.Identity {
	id := identity.Identity{
		Issuer:           stringClaim(claims, "iss"),
		Subject:          stringClaim(claims, "sub"),
		Email:            email.Normalize(stringClaim(claims, emailClaim)),
		EmailVerified:    boolClaim(claims, "email_verified"),
		HostedDomain:     strings.ToLower(stringClaim(claims, "hd")),
		Name:             nameOf(claims),
		HasGroupsOverage: hasGroupsOverage(claims, groupsClaim),
	}
	id.Groups, id.HasGroupsClaim = groupsOf(claims, groupsClaim)
	return id
}

func FillFromUserinfo(id identity.Identity, claims map[string]any, emailClaim, groupsClaim string) identity.Identity {
	extra := MapClaims(claims, emailClaim, groupsClaim)
	if id.Email == "" && extra.Email != "" {
		id.Email = extra.Email
		if extra.EmailVerified != nil {
			id.EmailVerified = extra.EmailVerified
		}
	}
	if id.Name == "" {
		id.Name = extra.Name
	}
	if id.HostedDomain == "" {
		id.HostedDomain = extra.HostedDomain
	}
	if !id.HasGroupsClaim && extra.HasGroupsClaim {
		id.Groups, id.HasGroupsClaim, id.HasGroupsOverage = extra.Groups, true, false
	}
	return id
}

func needsUserinfo(id identity.Identity) bool {
	return id.Email == "" || id.Name == "" || !id.HasGroupsClaim
}

func stringClaim(claims map[string]any, name string) string {
	value, _ := claims[name].(string)
	return strings.TrimSpace(value)
}

func boolClaim(claims map[string]any, name string) *bool {
	switch value := claims[name].(type) {
	case bool:
		return &value
	case string:
		isTrue := strings.EqualFold(strings.TrimSpace(value), "true")
		return &isTrue
	}
	return nil
}

func nameOf(claims map[string]any) string {
	if name := stringClaim(claims, "name"); name != "" {
		return name
	}
	return strings.TrimSpace(stringClaim(claims, "given_name") + " " + stringClaim(claims, "family_name"))
}

func groupsOf(claims map[string]any, groupsClaim string) ([]string, bool) {
	switch value := claims[groupsClaim].(type) {
	case []any:
		groups := make([]string, 0, len(value))
		for _, item := range value {
			if group, isString := item.(string); isString && group != "" {
				groups = append(groups, group)
			}
		}
		return groups, true
	case string:
		if value == "" {
			return []string{}, true
		}
		return []string{value}, true
	}
	return nil, false
}

func hasGroupsOverage(claims map[string]any, groupsClaim string) bool {
	names, _ := claims["_claim_names"].(map[string]any)
	_, isPresent := names[groupsClaim]
	return isPresent
}
