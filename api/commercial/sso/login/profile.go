package login

import (
	"strings"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

const maxNameRunes = 250

type Profile struct {
	Name  string
	Email string
}

func NextProfile(id identity.Identity, user *model.User, rules Rules, isEmailTaken bool) Profile {
	profile := Profile{Name: user.Name, Email: user.Email}
	if name := cleanName(id.Name); name != "" {
		profile.Name = name
	}
	if canApplyEmail(id, rules, isEmailTaken) {
		profile.Email = id.Email
	}
	return profile
}

func NewUserName(id identity.Identity) string {
	if name := cleanName(id.Name); name != "" {
		return name
	}
	local, _, _ := strings.Cut(id.Email, "@")
	return cleanName(local)
}

func canApplyEmail(id identity.Identity, rules Rules, isEmailTaken bool) bool {
	return id.Email != "" && !isEmailTaken && !isUnverified(id) && isDomainAllowed(domainOf(id.Email), rules)
}

func cleanName(raw string) string {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) > maxNameRunes {
		name = string([]rune(name)[:maxNameRunes])
	}
	return name
}
