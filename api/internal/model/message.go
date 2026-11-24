package model

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

var ErrInvalidAnchor = errors.New("invalid anchor")

type MessageRecipientType int64

const (
	TeammateRecipientType MessageRecipientType = 1
	TeamRecipientType     MessageRecipientType = 2
	ProjectRecipientType  MessageRecipientType = 3
	IssueRecipientType    MessageRecipientType = 4
)

type Message struct {
	IdMessage              int64                 `json:"idMessage"`
	IdMessageRecipientType MessageRecipientType  `json:"idMessageRecipientType"`
	IdRecipient            int64                 `json:"idRecipient"`
	Message                string                `json:"message"`
	MessageKind            constants.MessageKind `json:"messageKind"`
	CreatedAt              time.Time             `json:"createdAt"`
	UpdatedAt              *time.Time            `json:"updatedAt"`
	Creator                *User                 `json:"creator"`
	IsRead                 bool                  `json:"isRead"`
	Version                int                   `json:"version"`
	Anchor                 *MessageAnchor        `json:"anchor,omitempty"`
}

const MaxAnchorQuoteLength = 2000

type MessageAnchor struct {
	IdParentMessage int64   `json:"idParentMessage"`
	ParentVersion   int     `json:"parentVersion"`
	AnchorLineStart int     `json:"anchorLineStart"`
	AnchorLineEnd   int     `json:"anchorLineEnd"`
	IsOutdated      bool    `json:"isOutdated"`
	AnchorText      string  `json:"anchorText"`
	Quote           *string `json:"quote,omitempty"`
	IdThreadMessage *int64  `json:"idThreadMessage,omitempty"`
}

type CreateMessageReq struct {
	IdRecipient            int64                `json:"idRecipient" binding:"required"`
	IdMessageRecipientType MessageRecipientType `json:"idMessageRecipientType" binding:"required,lte=4,gte=1"`
	Message                string               `json:"message" binding:"required"`
	IdParentMessage        *int64               `json:"idParentMessage,omitempty"`
	AnchorLineStart        *int                 `json:"anchorLineStart,omitempty"`
	AnchorLineEnd          *int                 `json:"anchorLineEnd,omitempty"`
	AnchorQuote            *string              `json:"anchorQuote,omitempty"`
	IdThreadMessage        *int64               `json:"idThreadMessage,omitempty"`
}

func (req *CreateMessageReq) Validate() error {
	hasParent := req.IdParentMessage != nil
	hasStart := req.AnchorLineStart != nil
	hasEnd := req.AnchorLineEnd != nil
	hasQuote := req.AnchorQuote != nil

	allSet := hasParent && hasStart && hasEnd
	noneSet := !hasParent && !hasStart && !hasEnd

	if !allSet && !noneSet {
		return ErrInvalidAnchor
	}
	if req.IdThreadMessage != nil && (!noneSet || hasQuote) {
		return ErrInvalidAnchor
	}
	if hasQuote {
		if !allSet {
			return ErrInvalidAnchor
		}
		quote := strings.TrimSpace(*req.AnchorQuote)
		if quote == "" || utf8.RuneCountInString(quote) > MaxAnchorQuoteLength {
			return ErrInvalidAnchor
		}
	}
	if allSet {
		if *req.AnchorLineStart < 1 {
			return ErrInvalidAnchor
		}
		if *req.AnchorLineEnd < *req.AnchorLineStart {
			return ErrInvalidAnchor
		}
	}
	return nil
}

type UpdateMessageReq struct {
	Message string `json:"message" binding:"required"`
}

type GetMessagesReq struct {
	IdRecipient            int64                `json:"idRecipient"`
	IdMessageRecipientType MessageRecipientType `json:"idMessageRecipientType"`
	Read                   *bool                `json:"read"`
}

type SetReadMessageReq struct {
	IdRecipient            int64                `json:"idRecipient" binding:"required"`
	IdMessageRecipientType MessageRecipientType `json:"idMessageRecipientType" binding:"required,lte=4,gte=1"`
}
