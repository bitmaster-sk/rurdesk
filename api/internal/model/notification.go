package model

import (
	"encoding/json"
	"time"
)

type Notification struct {
	IdNotification int64           `json:"idNotification" db:"id_notification"`
	IdUser         int64           `json:"-" db:"id_user"`
	Type           string          `json:"type" db:"type"`
	IdProject      *int64          `json:"idProject,omitempty" db:"id_project"`
	ProjectName    string          `json:"projectName,omitempty" db:"project_name"`
	ProjectColor   string          `json:"projectColor,omitempty" db:"project_color"`
	ActorName      string          `json:"actorName,omitempty" db:"actor_name"`
	ActorAvatarBg  string          `json:"actorAvatarBg,omitempty" db:"actor_avatar_bg"`
	RefType        string          `json:"refType,omitempty" db:"ref_type"`
	RefId          string          `json:"refId,omitempty" db:"ref_id"`
	RefTitle       string          `json:"refTitle,omitempty" db:"ref_title"`
	RefPublicId    *int64          `json:"refPublicId,omitempty" db:"ref_public_id"`
	Body           json.RawMessage `json:"body,omitempty" db:"body"`
	IsRead         bool            `json:"isRead" db:"is_read"`
	CreatedAt      time.Time       `json:"createdAt" db:"created_at"`
}

type CreateNotificationReq struct {
	IdUser        int64
	Type          string
	IdProject     *int64
	ProjectName   string
	ProjectColor  string
	ActorName     string
	ActorAvatarBg string
	RefType       string
	RefId         string
	RefTitle      string
	RefPublicId   *int64
	Body          any
	Source        string // "bot" when the acting user has is_bot=true
}

type NotificationListFilter struct {
	IdUser     int64
	IdProject  *int64
	OnlyUnread bool
	Limit      int
	Offset     int
}

type MarkAllReadReq struct {
	IdProject *int64 `json:"idProject"`
}
