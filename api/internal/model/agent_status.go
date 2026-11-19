package model

// AgentStatus is what a conversation shows about its agent. IdIssuePublic, IdProject and Stage
// describe the stage it runs only when the owner can read its project; QueuePosition is set while
// this conversation's turn waits (1 = next); IdTask is this conversation's running turn, for replaying its thinking.
type AgentStatus struct {
	State         string  `json:"state"`
	IdIssuePublic *int64  `json:"idIssuePublic"`
	IdProject     *int64  `json:"idProject"`
	Stage         *string `json:"stage"`
	QueuePosition *int    `json:"queuePosition"`
	IdTask        *int64  `json:"idTask"`
}

// AgentActiveTask is the stage an agent runs right now. IdProject is 0 for a chat
// without a project.
type AgentActiveTask struct {
	IdTask        int64
	IdRun         int64
	Kind          string
	IdProject     int64
	IdIssuePublic *int64
	Stage         string
}
