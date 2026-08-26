package model

import "encoding/json"

type User struct {
	IdUser        int64  `json:"idUser"        db:"id_user"`
	Name          string `json:"name"          db:"name"`
	Email         string `json:"email"         db:"email"`
	ColorAvatarBg string `json:"colorAvatarBg" db:"color_avatar_bg"`
	Password      string `json:"-"             db:"password"`
	IsBot         bool   `json:"isBot"         db:"is_bot"`
	IsAdmin       bool   `json:"isAdmin"       db:"is_admin"`
}

func (u *User) MarshalBinary() ([]byte, error) {
	return json.Marshal(u)
}

func (u *User) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, &u)
}

type LoginReq struct {
	Email                      string `json:"email" binding:"required,email,max=250"`
	Password                   string `json:"password" binding:"required,min=5,max=100"`
	HasExtendedSessionLifetime bool   `json:"hasExtendedSessionLifetime"`
}

type RegisterReq struct {
	Name     string `json:"name" binding:"required,min=5,max=250"`
	Email    string `json:"email" binding:"required,email,max=250"`
	Password string `json:"password" binding:"required,min=5,max=100"`
}

type UpdateUserReq struct {
	Name string `json:"name" binding:"required,min=1,max=250"`
	// Optional: when present, overrides the avatar background colour. Omitted → unchanged.
	ColorAvatarBg *string `json:"colorAvatarBg" binding:"omitempty,hexcolor"`
}

type ChangePasswordReq struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=5,max=100"`
	NewPassword     string `json:"newPassword" binding:"required,min=5,max=100"`
}

// AdminCreateUserReq is the body for POST /admin/user. For bots, Email/Password are ignored.
type AdminCreateUserReq struct {
	Name      string `json:"name"      binding:"required,min=1,max=250"`
	Email     string `json:"email"     binding:"omitempty,email,max=250"`
	Password  string `json:"password"  binding:"omitempty,min=5,max=100"`
	IsBot     bool   `json:"isBot"`
	IsAdmin   bool   `json:"isAdmin"`
	IdProject *int64 `json:"idProject"`
	Role      Role   `json:"role"`
	// Optional: when present, seeds the avatar background colour instead of a random one.
	ColorAvatarBg *string `json:"colorAvatarBg" binding:"omitempty,hexcolor"`
}

// AdminCreateUserRes returns the created user. RawKey is set only for bots (shown once).
type AdminCreateUserRes struct {
	User
	RawKey string `json:"rawKey,omitempty"`
}

// AdminUpdateUserReq is the PATCH /admin/user/:idUser body. Every field is an
// optional pointer, serving both the isAdmin toggle and the edit form — only
// present fields apply, and an explicit false for IsAdmin differs from omitted.
type AdminUpdateUserReq struct {
	Name          *string `json:"name"          binding:"omitempty,min=1,max=250"`
	Email         *string `json:"email"         binding:"omitempty,email,max=250"`
	IsAdmin       *bool   `json:"isAdmin"`
	ColorAvatarBg *string `json:"colorAvatarBg" binding:"omitempty,hexcolor"`
}
