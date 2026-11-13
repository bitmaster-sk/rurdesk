package links

import (
	"net/http"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

var (
	ErrIdentityNotFound = errs.NewErrWithStatus("SSO_IDENTITY_NOT_FOUND", "the account has no SSO identity", "error.sso_identity_not_found", http.StatusNotFound)
	ErrTargetInvalid    = errs.NewErr("SSO_IDENTITY_TARGET_INVALID", "the target must be another active person account that is not a break-glass account", "error.sso_identity_target_invalid")
	ErrIdentityExists   = errs.NewErrWithStatus("SSO_IDENTITY_EXISTS", "the target already has an identity from this identity provider", "error.sso_identity_exists", http.StatusConflict)
)

type IdentityRes struct {
	IdUserIdentity int64     `json:"idUserIdentity" db:"id_user_identity"`
	Issuer         string    `json:"issuer" db:"issuer"`
	Subject        string    `json:"subject" db:"subject"`
	CreateAt       time.Time `json:"createAt" db:"create_at"`
}

type MoveReq struct {
	IdTargetUser int64 `json:"idTargetUser" binding:"required,min=1"`
}
