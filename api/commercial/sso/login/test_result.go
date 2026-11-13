package login

import "github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"

type TestError string

const (
	TestErrorIdp                TestError = "idp_error"
	TestErrorTokenInvalid       TestError = "token_invalid"
	TestErrorSubjectMissing     TestError = "subject_missing"
	TestErrorSubjectUnsupported TestError = "subject_unsupported"
	TestErrorDraftChanged       TestError = "draft_changed"
	TestErrorSsoUnavailable     TestError = "sso_unavailable"
	TestErrorInternal           TestError = "internal"
)

type TestResult struct {
	IsSuccess      bool      `json:"isSuccess"`
	Error          TestError `json:"error,omitempty"`
	Detail         string    `json:"detail,omitempty"`
	Subject        string    `json:"subject"`
	Email          string    `json:"email"`
	EmailVerified  *bool     `json:"emailVerified"`
	Name           string    `json:"name"`
	Groups         []string  `json:"groups"`
	HasGroupsClaim bool      `json:"hasGroupsClaim"`
	Action         Action    `json:"action,omitempty"`
	Reason         Reason    `json:"reason,omitempty"`
	Warnings       []Warning `json:"warnings"`
}

func SucceededTest(id identity.Identity, outcome Outcome, rules Rules) TestResult {
	groups := id.Groups
	if groups == nil {
		groups = []string{}
	}
	return TestResult{
		IsSuccess:      true,
		Subject:        id.Subject,
		Email:          id.Email,
		EmailVerified:  id.EmailVerified,
		Name:           id.Name,
		Groups:         groups,
		HasGroupsClaim: id.HasGroupsClaim,
		Action:         outcome.Action,
		Reason:         outcome.Reason,
		Warnings:       Warnings(id, rules),
	}
}

func FailedTest(code TestError, detail string) TestResult {
	return TestResult{Error: code, Detail: detail, Groups: []string{}, Warnings: []Warning{}}
}
