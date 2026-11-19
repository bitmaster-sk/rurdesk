package model

import "time"

// AgentConversation is a user's private chat with an agent. IdProject is the main
// project (its repo is the agent's code context); IdsProjectContext are extra
// projects whose tasks and wiki the agent may read. Model is what the
// agent answers with; nil is the gateway's default model.
type AgentConversation struct {
	IdConversation     int64      `json:"idConversation"     db:"id_conversation"`
	IdUser             int64      `json:"idUser"             db:"id_user"`
	IdUserAgent        int64      `json:"idUserAgent"        db:"id_user_agent"`
	IdProject          *int64     `json:"idProject"          db:"id_project"`
	IdsProjectContext  []int64    `json:"idsProjectContext"  db:"ids_project_context"`
	IdRun              *int64     `json:"idRun"              db:"id_run"`
	Title              string     `json:"title"              db:"title"`
	HeadCommit         *string    `json:"headCommit"         db:"head_commit"`
	IsLocked           bool       `json:"isLocked"           db:"is_locked"`
	TaskDraftRequest   *string    `json:"taskDraftRequest"   db:"task_draft_request"`
	IdTaskDraftMessage *int64     `json:"idTaskDraftMessage" db:"task_draft_message_id"`
	ReadAt             *time.Time `json:"readAt"             db:"read_at"`
	PinnedAt           *time.Time `json:"pinnedAt"           db:"pinned_at"`
	Model              *string    `json:"model"              db:"model"`
	LastAgentMessageAt *time.Time `json:"lastAgentMessageAt" db:"last_agent_message_at"`
	CreatedAt          time.Time  `json:"createdAt"          db:"created_at"`
	UpdatedAt          time.Time  `json:"updatedAt"          db:"updated_at"`
}

// AgentConversationMessage is one entry of a conversation. HasThinking tells the
// client whether the agent turn behind it (IdTask) recorded thinking to replay.
type AgentConversationMessage struct {
	IdMessage      int64     `json:"idMessage"      db:"id_message"`
	IdConversation int64     `json:"idConversation" db:"id_conversation"`
	Author         string    `json:"author"         db:"author"`
	Kind           string    `json:"kind"           db:"kind"`
	Body           string    `json:"body"           db:"body"`
	IdUser         *int64    `json:"idUser"         db:"id_user"`
	IdTask         *int64    `json:"idTask"         db:"id_task"`
	HasThinking    bool      `json:"hasThinking"    db:"has_thinking"`
	CreatedAt      time.Time `json:"createdAt"      db:"created_at"`
}

// CreateAgentConversationReq: an empty Model means the default.
type CreateAgentConversationReq struct {
	IdUserAgent int64   `json:"idUserAgent" binding:"required"`
	IdProject   *int64  `json:"idProject"`
	Model       *string `json:"model"`
}

// UpdateAgentConversationSettingsReq replaces the model; null or empty means the default.
type UpdateAgentConversationSettingsReq struct {
	Model *string `json:"model"`
}

// AgentConversationAgent is an agent the owner can chat with, with the models it offers.
type AgentConversationAgent struct {
	User
	AgentModelChoices
}

// CreateAgentConversationTaskDraftReq names the agent answer the task draft is about.
type CreateAgentConversationTaskDraftReq struct {
	IdMessage int64 `json:"idMessage" binding:"required"`
}

// SetAgentConversationPinnedReq pins (true) or unpins (false) a conversation; a pointer so that a missing flag is rejected.
type SetAgentConversationPinnedReq struct {
	IsPinned *bool `json:"isPinned" binding:"required"`
}

type CreateAgentConversationMessageReq struct {
	Message string `json:"message" binding:"required,max=20000"`
}

// AgentConversationListItem is a conversation in the owner's list. IsUnread: the
// agent wrote after the owner last read it. TokensUsed and CostUsd sum every turn; CostUsd is nil
// while no turn has a cost, CostEstimated when any priced turn is estimated.
type AgentConversationListItem struct {
	AgentConversation
	IsUnread      bool     `json:"isUnread"      db:"is_unread"`
	TokensUsed    int      `json:"tokensUsed"    db:"tokens_used"`
	CostUsd       *float64 `json:"costUsd"       db:"cost_usd"`
	CostEstimated bool     `json:"costEstimated" db:"cost_estimated"`
}

// AgentConversationProject is a project attached to a conversation: the main one
// or a context one.
type AgentConversationProject struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name"`
	IsMain    bool   `json:"isMain"`
}

type AgentConversationDetail struct {
	Conversation *AgentConversation         `json:"conversation"`
	Projects     []AgentConversationProject `json:"projects"`
	Agent        *User                      `json:"agent"`
	AgentStatus  AgentStatus                `json:"agentStatus"`
	AgentModels  AgentModelChoices          `json:"agentModels"`
	Stats        *RunStatsRes               `json:"stats"`
}

type AddAgentConversationProjectReq struct {
	IdProject int64 `json:"idProject" binding:"required"`
}

// AgentConversationTurnNotice tells the owner that the agent took up a turn of the conversation.
type AgentConversationTurnNotice struct {
	IdConversation int64 `json:"idConversation"`
	IdTask         int64 `json:"idTask"`
}

type AgentConversationUnreadCountRes struct {
	Count int `json:"count"`
}
