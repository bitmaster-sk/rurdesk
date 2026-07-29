package errs

import (
	"errors"
	"net/http"
)

// Error is a typed application error carrying a code, message, i18n translate key,
// and HTTP status so controllers can respond consistently.
type Error struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	TranslateKey string `json:"translateKey,omitempty"`
	status       int
}

func (e *Error) Error() string { return e.Message }

func (e *Error) HttpStatus() int { return e.status }

// WithTranslateKey returns a copy of the error with the given translate key set.
// Use this to attach a dynamic key without mutating the shared sentinel.
func (e *Error) WithTranslateKey(key string) *Error {
	cp := *e
	cp.TranslateKey = key
	return &cp
}

func newErr(code, message, translateKey string, status int) *Error {
	return &Error{Code: code, Message: message, TranslateKey: translateKey, status: status}
}

// NewErr creates a new application error with StatusUnprocessableEntity by default.
// Use for domain-level errors that don't fit existing sentinels.
func NewErr(code, message, translateKey string) *Error {
	return newErr(code, message, translateKey, http.StatusUnprocessableEntity)
}

var (
	ErrBadRequest                     = newErr("BAD_REQUEST", "bad request", "error.bad_request", http.StatusBadRequest)
	ErrUnauthorized                   = newErr("UNAUTHORIZED", "unauthorized", "error.unauthorized", http.StatusUnauthorized)
	ErrForbidden                      = newErr("FORBIDDEN", "forbidden", "error.forbidden", http.StatusForbidden)
	ErrNotFound                       = newErr("NOT_FOUND", "not found", "error.not_found", http.StatusNotFound)
	ErrInternal                       = newErr("INTERNAL_ERROR", "internal server error", "error.internal", http.StatusInternalServerError)
	ErrConflict                       = newErr("CONFLICT", "conflict", "error.conflict", http.StatusConflict)
	ErrStateInUse                     = newErr("STATE_IN_USE", "state is still in use (issues, project default or agent phases); pass migrateTo=<id> or migrateTo=null", "error.state_in_use", http.StatusConflict)
	ErrSeverityInUse                  = newErr("SEVERITY_IN_USE", "severity is still in use (issues or project default); pass migrateTo=<id> or migrateTo=null", "error.severity_in_use", http.StatusConflict)
	ErrCycle                          = newErr("CYCLE", "relation would create a cycle", "error.relation_cycle", http.StatusUnprocessableEntity)
	ErrAiUnavailable                  = newErr("AI_UNAVAILABLE", "AI service unavailable", "error.ai_unavailable", http.StatusServiceUnavailable)
	ErrAiNotConfigured                = newErr("AI_NOT_CONFIGURED", "no AI model configured", "error.ai_not_configured", http.StatusServiceUnavailable)
	ErrAiInvalidResponse              = newErr("AI_INVALID_RESPONSE", "AI returned an invalid response", "error.ai_invalid_response", http.StatusUnprocessableEntity)
	ErrRateLimited                    = newErr("RATE_LIMITED", "too many requests", "error.rate_limited", http.StatusTooManyRequests)
	ErrInvalidStateMigrationTarget    = newErr("INVALID_MIGRATION_TARGET", "migration target must be a different state of the same project", "error.invalid_state_migration_target", http.StatusUnprocessableEntity)
	ErrInvalidSeverityMigrationTarget = newErr("INVALID_MIGRATION_TARGET", "migration target must be a different severity of the same project", "error.invalid_severity_migration_target", http.StatusUnprocessableEntity)
)

// As reports whether err is (or wraps) an *Error, storing it in target. It
// delegates to errors.As so a sentinel stays matchable after being wrapped with %w.
func As(err error, target **Error) bool {
	return errors.As(err, target)
}

// FromStatus returns the sentinel *Error for a bare HTTP status, so a handler that
// set only a status (no typed error) still yields a translatable body. Unmapped
// statuses fall back to ErrInternal.
func FromStatus(status int) *Error {
	switch status {
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusServiceUnavailable:
		return ErrAiUnavailable
	default:
		return ErrInternal
	}
}
