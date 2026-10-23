package model

type Message struct {
	Message     string `json:"message"`
	MessageKind string `json:"messageKind"`
	CreatedAt   string `json:"createdAt"`
	Creator     *User  `json:"creator"`
}
