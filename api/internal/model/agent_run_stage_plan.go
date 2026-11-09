package model

type AgentRunStagePlanEntry struct {
	Name       string  `json:"name"`
	IdsSkill   []int64 `json:"idsSkill"`
	Model      *string `json:"model"`
	MaxTurns   *int    `json:"maxTurns"`
	Effort     *string `json:"effort"`
	Skip       bool    `json:"skip"`
	Dispatched bool    `json:"dispatched"`
}

type AgentRunStagePlanRes struct {
	Stages                []AgentRunStagePlanEntry     `json:"stages"`
	Models                []string                     `json:"models"`
	ModelDefault          *string                      `json:"modelDefault"`
	MaxTurnsLimit         *int                         `json:"maxTurnsLimit"`
	DefaultsByStage       map[string]AgentStageDefault `json:"defaultsByStage"`
	ModelsWithoutThinking []string                     `json:"modelsWithoutThinking"`
}

type UpdateAgentRunStagePlanEntryReq struct {
	Stage    string  `json:"stage"    binding:"required,oneof=brainstorming design implementation_plan implementation"`
	IdsSkill []int64 `json:"idsSkill"`
	Model    string  `json:"model"`
	MaxTurns int     `json:"maxTurns"`
	Effort   string  `json:"effort"   binding:"omitempty,oneof=low medium high max"`
}
