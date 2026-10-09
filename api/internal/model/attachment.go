package model

import (
	"time"

	"github.com/google/uuid"
)

type AttachmentScope struct {
	IdIssue   *int64 `json:"idIssue"   db:"id_issue"`
	IdProject *int64 `json:"idProject" db:"id_project"`
	IdTeam    *int64 `json:"idTeam"    db:"id_team"`
	IdUserTo  *int64 `json:"idUserTo"  db:"id_user_to"`
}

func AttachmentScopeOf(recipientType MessageRecipientType, idRecipient int64) (AttachmentScope, bool) {
	switch recipientType {
	case TeammateRecipientType:
		return AttachmentScope{IdUserTo: &idRecipient}, true
	case TeamRecipientType:
		return AttachmentScope{IdTeam: &idRecipient}, true
	case ProjectRecipientType:
		return AttachmentScope{IdProject: &idRecipient}, true
	case IssueRecipientType:
		return AttachmentScope{IdIssue: &idRecipient}, true
	default:
		return AttachmentScope{}, false
	}
}

func (s AttachmentScope) Recipient() (MessageRecipientType, int64, bool) {
	switch {
	case s.IdUserTo != nil:
		return TeammateRecipientType, *s.IdUserTo, true
	case s.IdTeam != nil:
		return TeamRecipientType, *s.IdTeam, true
	case s.IdProject != nil:
		return ProjectRecipientType, *s.IdProject, true
	case s.IdIssue != nil:
		return IssueRecipientType, *s.IdIssue, true
	default:
		return 0, 0, false
	}
}

func (s AttachmentScope) HasScope(other AttachmentScope) bool {
	recipientType, idRecipient, ok := s.Recipient()
	otherType, otherIdRecipient, otherOk := other.Recipient()
	return ok && otherOk && recipientType == otherType && idRecipient == otherIdRecipient
}

type Attachment struct {
	IdAttachment uuid.UUID `json:"idAttachment" db:"id_attachment"`
	FileName     string    `json:"fileName"     db:"file_name"`
	MimeType     string    `json:"mimeType"     db:"mime_type"`
	Size         int64     `json:"size"         db:"size"`
	AttachmentScope
	CreateBy *int64    `json:"createBy" db:"create_by"`
	CreateAt time.Time `json:"createAt" db:"create_at"`
}

type AttachmentRes struct {
	IdAttachment uuid.UUID `json:"idAttachment"`
	FileName     string    `json:"fileName"`
	MimeType     string    `json:"mimeType"`
	Size         int64     `json:"size"`
}
