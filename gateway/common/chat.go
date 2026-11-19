package common

// Chat turn modes and the complete_stage message kinds that answer them,
// mirroring api/internal/constants/agent_conversation.go.
const (
	ChatModeReply            = "reply"
	ChatModeTaskDraft        = "task_draft"
	ChatMessageKindReply     = "chat_reply"
	ChatMessageKindTaskDraft = "task_draft"
)

// ChatContext is what a chat turn knows about its conversation. HeadCommit is
// the commit the previous turn saw; WorktreeHead is the commit of this turn's
// worktree, set once the workspace is ready (empty without a repo).
type ChatContext struct {
	IdConversation     int64
	Mode               string
	Projects           []ChatProject
	HeadCommit         string
	WorktreeHead       string
	IsHistoryTruncated bool
	// DraftFromMessage is the agent answer a task draft turn writes its task about.
	DraftFromMessage *ChatDraftFromMessage
}

// ChatDraftFromMessage is the quoted agent answer of a task draft turn.
type ChatDraftFromMessage struct {
	IdMessage int64
	Body      string
}

type ChatProject struct {
	IdProject int64
	Name      string
	IsMain    bool
}
