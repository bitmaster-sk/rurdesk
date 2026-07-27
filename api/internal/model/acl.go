package model

// Role represents a project membership role.
type Role string

const (
	RoleViewer Role = "viewer"
	RoleMember Role = "member"
	RoleOwner  Role = "owner"
)

// RoleRank returns a role's numeric rank (higher = more privilege).
// A function rather than an exported map, to prevent accidental mutation.
func RoleRank(role Role) int {
	switch role {
	case RoleOwner:
		return 2
	case RoleMember:
		return 1
	default:
		return 0
	}
}

// AroUser is the Access Request Object for a user in a project context.
// Embeds User so standard fields are included without polluting the User model.
type AroUser struct {
	User
	Role     Role    `json:"role"`
	IsDirect bool    `json:"isDirect"`
	IdsTeams []int64 `json:"idsTeams"`
}

// AroTeam is the Access Request Object for a team in a project context.
type AroTeam struct {
	Team
	Role        Role `json:"role"`
	MemberCount int  `json:"memberCount"`
}

// ProjectMembersRes is the response envelope for GET /project/:id/member.
type ProjectMembersRes struct {
	Users []AroUser `json:"users"`
	Teams []AroTeam `json:"teams"`
}

// UserRoleRes is the response for GET /project/:id/user-role.
type UserRoleRes struct {
	Role Role `json:"role"`
}

// IsValidRole reports whether role is one of the three known roles.
// Used for DTO validation, since binding:"required" doesn't validate string enums.
func IsValidRole(role Role) bool {
	return role == RoleViewer || role == RoleMember || role == RoleOwner
}

// AddProjectUserReq is the request body for POST /project/:id/member/user.
type AddProjectUserReq struct {
	IdUser int64 `json:"idUser" binding:"required"`
	Role   Role  `json:"role"   binding:"required"`
}

// UpdateProjectUserRoleReq is the request body for PATCH /project/:id/member/user/:idUser.
type UpdateProjectUserRoleReq struct {
	Role Role `json:"role" binding:"required"`
}

// AddProjectTeamReq is the request body for POST /project/:id/member/team.
type AddProjectTeamReq struct {
	IdTeam int64 `json:"idTeam" binding:"required"`
	Role   Role  `json:"role"   binding:"required"`
}

// UpdateProjectTeamRoleReq is the request body for PATCH /project/:id/member/team/:idTeam.
type UpdateProjectTeamRoleReq struct {
	Role Role `json:"role" binding:"required"`
}
