package handler

import (
	"errors"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

var (
	ErrInvalidCode         = errs.NewErr("SSO_INVALID_CODE", "the sign-in code is invalid, used or expired", "error.sso_invalid_code")
	ErrUserDisabled        = errs.NewErr("SSO_USER_DISABLED", "the account cannot sign in", "error.sso_user_disabled")
	ErrDiscoveryFailed     = errs.NewErr("SSO_DISCOVERY_FAILED", "the identity provider could not be reached", "error.sso_discovery_failed")
	ErrMetadataUnreachable = errs.NewErr("SSO_METADATA_UNREACHABLE", "the identity provider metadata could not be downloaded", "error.sso_metadata_unreachable")

	errNotLicensed = errors.New("sso is not licensed")
	errNotEnabled  = errors.New("sso is not enabled")
)

const (
	errorSsoUnavailable = "sso_unavailable"
	errorInvalidState   = "invalid_state"
	errorIdp            = "idp_error"
	errorInternal       = "internal"

	errorSamlSubjectUnsupported = "saml_subject_unsupported"
)
