package model

// Counters are global; named issues are scoped to the requested project so a
// dropdown cannot leak issue numbers from a project the caller cannot read.
type AgentOverview struct {
	IdUserAgent          int64            `json:"idUserAgent"`
	IsBusy               bool             `json:"isBusy"`
	Current              *AgentCurrentRun `json:"current"`
	QueueCount           int              `json:"queueCount"`
	QueuedIdsIssuePublic []int64          `json:"queuedIdsIssuePublic"`
	CompletedToday       int              `json:"completedToday"`
	Tokens7d             int64            `json:"tokens7d"`
	AvgRunDurationMs7d   *int64           `json:"avgRunDurationMs7d"`
	FailedAttempts7d     int              `json:"failedAttempts7d"`
	Models               []string         `json:"models"`
	ModelDefault         *string          `json:"modelDefault"`
	MaxTurnsPlan         int              `json:"maxTurnsPlan"`
	MaxTurnsImplement    int              `json:"maxTurnsImplement"`
	MaxTurnsLimit        *int             `json:"maxTurnsLimit"`
}

type AgentCurrentRun struct {
	IdIssuePublic *int64  `json:"idIssuePublic"`
	ReleaseName   *string `json:"releaseName"`
	Stage         string  `json:"stage"`
}
