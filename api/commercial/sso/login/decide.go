package login

import (
	"slices"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type Action string

const (
	ActionSignIn Action = "sign_in"
	ActionLink   Action = "link"
	ActionCreate Action = "create"
	ActionReject Action = "reject"
)

type Reason string

const (
	ReasonEmailMissing          Reason = "email_missing"
	ReasonDomainNotAllowed      Reason = "domain_not_allowed"
	ReasonEmailNotVerified      Reason = "email_not_verified"
	ReasonEmailAmbiguous        Reason = "email_ambiguous"
	ReasonAccountNotLinkable    Reason = "account_not_linkable"
	ReasonAccountDisabled       Reason = "account_disabled"
	ReasonAccountNotProvisioned Reason = "account_not_provisioned"
)

const GoogleIssuer = "https://accounts.google.com"

type Rules struct {
	AllowedDomains []string
	IsAutoCreate   bool
	AdminGroup     string
}

type Candidate struct {
	User              *model.User
	HasIssuerIdentity bool
}

type Candidates struct {
	Linked  *model.User
	ByEmail []Candidate
}

type Outcome struct {
	Action Action
	Reason Reason
	User   *model.User
}

func Decide(id identity.Identity, candidates Candidates, rules Rules) Outcome {
	if candidates.Linked != nil {
		if !candidates.Linked.IsActive {
			return reject(ReasonAccountDisabled)
		}
		return Outcome{Action: ActionSignIn, User: candidates.Linked}
	}
	if reason, isRejected := emailRejection(id, rules); isRejected {
		return reject(reason)
	}
	switch len(candidates.ByEmail) {
	case 0:
		if rules.IsAutoCreate {
			return Outcome{Action: ActionCreate}
		}
		return reject(ReasonAccountNotProvisioned)
	case 1:
		return decideLink(candidates.ByEmail[0])
	default:
		return reject(ReasonEmailAmbiguous)
	}
}

func decideLink(candidate Candidate) Outcome {
	user := candidate.User
	if user.IsAgent || user.IsBreakGlass || candidate.HasIssuerIdentity {
		return reject(ReasonAccountNotLinkable)
	}
	if !user.IsActive {
		return reject(ReasonAccountDisabled)
	}
	return Outcome{Action: ActionLink, User: user}
}

func emailRejection(id identity.Identity, rules Rules) (Reason, bool) {
	switch {
	case id.Email == "":
		return ReasonEmailMissing, true
	case !isDomainAllowed(domainOf(id.Email), rules):
		return ReasonDomainNotAllowed, true
	case id.Issuer == GoogleIssuer && !isDomainAllowed(id.HostedDomain, rules):
		return ReasonDomainNotAllowed, true
	case isUnverified(id):
		return ReasonEmailNotVerified, true
	}
	return "", false
}

func reject(reason Reason) Outcome {
	return Outcome{Action: ActionReject, Reason: reason}
}

func isUnverified(id identity.Identity) bool {
	return id.EmailVerified != nil && !*id.EmailVerified
}

func domainOf(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	return email[at+1:]
}

func isDomainAllowed(domain string, rules Rules) bool {
	return domain != "" && slices.Contains(rules.AllowedDomains, domain)
}
