package model

type Message struct {
	IdMessage   int64          `json:"idMessage"`
	Message     string         `json:"message"`
	MessageKind string         `json:"messageKind"`
	CreatedAt   string         `json:"createdAt"`
	Creator     *User          `json:"creator"`
	Anchor      *MessageAnchor `json:"anchor"`
}

// MessageAnchor marks a comment that reacts to part of an earlier message.
type MessageAnchor struct {
	IdParentMessage int64   `json:"idParentMessage"`
	AnchorLineStart int     `json:"anchorLineStart"`
	AnchorLineEnd   int     `json:"anchorLineEnd"`
	IsOutdated      bool    `json:"isOutdated"`
	AnchorText      string  `json:"anchorText"`
	Quote           *string `json:"quote"`
}
