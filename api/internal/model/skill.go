package model

import "time"

// IsBuiltin and IsEdited are derived, not stored.
type Skill struct {
	IdSkill         int64     `json:"idSkill"     db:"id_skill"`
	Name            string    `json:"name"        db:"name"`
	Description     string    `json:"description" db:"description"`
	Content         string    `json:"content"     db:"content"`
	BuiltinKey      *string   `json:"-"           db:"builtin_key"`
	BuiltinChecksum *string   `json:"-"          db:"builtin_checksum"`
	IsBuiltin       bool      `json:"isBuiltin"   db:"-"`
	IsEdited        bool      `json:"isEdited"    db:"-"`
	CreatedAt       time.Time `json:"createdAt"   db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt"   db:"updated_at"`
}

type CreateSkillReq struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	Content     string `json:"content"     binding:"required"`
}

// omitempty skips an absent or null field but not an empty string, so a PATCH
// cannot clear a required field.
type UpdateSkillReq struct {
	Name        *string `json:"name"        binding:"omitempty,min=1"`
	Description *string `json:"description"`
	Content     *string `json:"content"     binding:"omitempty,min=1"`
}
