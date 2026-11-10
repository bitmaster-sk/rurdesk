package model

// The max=10000 tag must stay equal to constants.MaxAgentTurns.
type AgentStageOptions struct {
	MaxTurns *int    `json:"maxTurns,omitempty" binding:"omitempty,min=1,max=10000"`
	Effort   *string `json:"effort,omitempty"   binding:"omitempty,oneof=low medium high max"`
}

type AgentStageDefault struct {
	MaxTurns int                `json:"maxTurns"`
	Effort   AgentEffortDefault `json:"effort"`
}

type AgentEffortDefault struct {
	Effort *string `json:"effort"`
	Source string  `json:"source"`
}
