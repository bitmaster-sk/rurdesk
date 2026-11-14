package groups

import (
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

var (
	ErrMappingExists   = errs.NewErrWithStatus("SSO_MAPPING_EXISTS", "this group is already mapped to the team", "error.sso_mapping_exists", http.StatusConflict)
	ErrMappingNotFound = errs.NewErrWithStatus("SSO_MAPPING_NOT_FOUND", "group mapping not found", "error.sso_mapping_not_found", http.StatusNotFound)
	ErrTeamNotFound    = errs.NewErr("SSO_TEAM_NOT_FOUND", "team not found", "error.sso_team_not_found")
)

type MappingRes struct {
	IdSsoGroupMapping int64  `json:"idSsoGroupMapping" db:"id_sso_group_mapping"`
	GroupValue        string `json:"groupValue" db:"group_value"`
	IdTeam            int64  `json:"idTeam" db:"id_team"`
	TeamName          string `json:"teamName" db:"team_name"`
}

type InsertMappingReq struct {
	GroupValue string `json:"groupValue" binding:"required,max=500"`
	IdTeam     int64  `json:"idTeam" binding:"required,min=1"`
}
