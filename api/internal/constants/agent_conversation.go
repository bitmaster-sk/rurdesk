package constants

import "time"

// Authors and kinds of agent.conversation_message rows.
const (
	ConversationAuthorUser   = "user"
	ConversationAuthorAgent  = "agent"
	ConversationAuthorSystem = "system"

	ConversationMessageText          = "text"
	ConversationMessageTaskDraft     = "task_draft"
	ConversationMessageError         = "error"
	ConversationMessageCodeRefreshed = "code_refreshed"
)

// Modes of a chat turn: an ordinary reply, or a task draft the owner asked for.
const (
	ChatModeReply     = "reply"
	ChatModeTaskDraft = "task_draft"
)

// A pending task draft: the next turn to start is the draft, or the draft waits
// for the reply turn that is already queued or running.
const (
	TaskDraftNextTurn  = "next_turn"
	TaskDraftAfterTurn = "after_turn"
)

// FailReasonGatewayRestartExhausted marks the attempt after which a chat turn gave
// up on restarts, so the next turn starts a fresh restart count.
const FailReasonGatewayRestartExhausted = "gateway_restart_exhausted"

// complete_stage message kinds of a chat turn, mapped to the stored message kind.
const (
	ChatMessageKindReply     = "chat_reply"
	ChatMessageKindTaskDraft = "task_draft"
)

var ChatMessageKinds = []string{ChatMessageKindReply, ChatMessageKindTaskDraft}

// ChatTurnMaxRestarts is how many gateway restarts in a row one turn survives before it ends with an error.
const ChatTurnMaxRestarts = 3

// RunTokenLifetime bounds a run token even when its turn never ends cleanly.
const RunTokenLifetime = 6 * time.Hour

// ChatHistoryMaxBytes caps the conversation history sent with a turn, far below
// the gateway's 1 MiB webhook limit.
const ChatHistoryMaxBytes = 512 * 1024

// ChatMessageMaxBytes caps one message body in that history; a longer body is cut with a marker.
const ChatMessageMaxBytes = 64 * 1024

const ChatTitleMaxRunes = 80
