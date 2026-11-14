package login

import "github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"

type Warning string

const (
	WarningEmailMissing       Warning = "email_missing"
	WarningDomainNotAllowed   Warning = "domain_not_allowed"
	WarningGoogleHdMissing    Warning = "google_hd_missing"
	WarningEmailNotVerified   Warning = "email_not_verified"
	WarningGroupsClaimMissing Warning = "groups_claim_missing"
	WarningGroupsOverage      Warning = "groups_overage"
	WarningSubjectUnstable    Warning = "saml_subject_unstable"
)

func Warnings(id identity.Identity, rules Rules) []Warning {
	warnings := []Warning{}
	switch {
	case id.Email == "":
		warnings = append(warnings, WarningEmailMissing)
	case !isDomainAllowed(domainOf(id.Email), rules):
		warnings = append(warnings, WarningDomainNotAllowed)
	}
	if id.Issuer == GoogleIssuer && id.HostedDomain == "" {
		warnings = append(warnings, WarningGoogleHdMissing)
	}
	if isUnverified(id) {
		warnings = append(warnings, WarningEmailNotVerified)
	}
	if id.IsSubjectUnstable {
		warnings = append(warnings, WarningSubjectUnstable)
	}
	switch {
	case id.HasGroupsOverage:
		warnings = append(warnings, WarningGroupsOverage)
	case !id.HasGroupsClaim:
		warnings = append(warnings, WarningGroupsClaimMissing)
	}
	return warnings
}
