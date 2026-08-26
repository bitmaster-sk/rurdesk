package model

// Counters are global; named issues are scoped to the requested project so a
// dropdown cannot leak issue numbers from a project the caller cannot read.
type AgentOverview struct {
	IdUserBot            int64            `json:"idUserBot"`
	IsBusy               bool             `json:"isBusy"`
	Current              *AgentCurrentRun `json:"current"`
	QueueCount           int              `json:"queueCount"`
	QueuedIdsIssuePublic []int64          `json:"queuedIdsIssuePublic"`
	CompletedToday       int              `json:"completedToday"`
	Tokens7d             int64            `json:"tokens7d"`
	AvgRunDurationMs7d   *int64           `json:"avgRunDurationMs7d"`
	FailedAttempts7d     int              `json:"failedAttempts7d"`
}

type AgentCurrentRun struct {
	IdIssuePublic int64  `json:"idIssuePublic"`
	Stage         string `json:"stage"`
}
