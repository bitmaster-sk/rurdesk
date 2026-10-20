package model

type AgentRunStagePlanEntry struct {
	Name       string  `json:"name"`
	IdsSkill   []int64 `json:"idsSkill"`
	Model      *string `json:"model"`
	MaxTurns   *int    `json:"maxTurns"`
	Skip       bool    `json:"skip"`
	Dispatched bool    `json:"dispatched"`
}

type AgentRunStagePlanRes struct {
	Stages            []AgentRunStagePlanEntry `json:"stages"`
	Models            []string                 `json:"models"`
	ModelDefault      *string                  `json:"modelDefault"`
	MaxTurnsPlan      int                      `json:"maxTurnsPlan"`
	MaxTurnsImplement int                      `json:"maxTurnsImplement"`
	MaxTurnsLimit     *int                     `json:"maxTurnsLimit"`
}

type UpdateAgentRunStagePlanEntryReq struct {
	Stage    string  `json:"stage"    binding:"required,oneof=brainstorming design implementation_plan implementation"`
	IdsSkill []int64 `json:"idsSkill"`
	Model    string  `json:"model"`
	MaxTurns int     `json:"maxTurns"`
}
