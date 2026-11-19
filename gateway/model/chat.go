package model

// ChatContext describes the conversation of a chat turn. HeadCommit is the
// commit the previous turn's worktree was at, empty before the first one.
type ChatContext struct {
	IdConversation int64         `json:"idConversation"`
	Mode           string        `json:"mode"`
	Projects       []ChatProject `json:"projects"`
	HeadCommit     string        `json:"headCommit"`
	// DraftFromMessage is set in task draft mode when the draft is about one agent answer.
	DraftFromMessage *ChatDraftFromMessage `json:"draftFromMessage"`
}

type ChatDraftFromMessage struct {
	IdMessage int64  `json:"idMessage"`
	Body      string `json:"body"`
}

type ChatProject struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name"`
	IsMain    bool   `json:"isMain"`
}

// ChatMessage is one reviewThread entry of a chat turn.
type ChatMessage struct {
	Author    string `json:"author"`
	Kind      string `json:"kind"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}
