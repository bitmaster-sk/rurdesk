package model

// IssueParticipant represents a user attached to an issue for notification scoping.
type IssueParticipant struct {
	IdUser                  int64  `json:"idUser"                  db:"id_user"`
	Name                    string `json:"name"                    db:"name"`
	ColorAvatarBg           string `json:"colorAvatarBg"           db:"color_avatar_bg"`
	IsBot                   bool   `json:"isBot"                   db:"is_bot"`
	Source                  string `json:"source"                  db:"source"`
	HasNotificationsEnabled bool   `json:"hasNotificationsEnabled" db:"has_notifications_enabled"`
}

// AddParticipantReq is the request body for manually adding a project member as a participant.
type AddParticipantReq struct {
	IdUser int64 `json:"idUser" binding:"required"`
}

// SetParticipantNotificationsReq is the request body for toggling the caller's own notifications.
type SetParticipantNotificationsReq struct {
	Enabled bool `json:"enabled"`
}
