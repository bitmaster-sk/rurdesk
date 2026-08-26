package model

type AgentRunStageSkills struct {
	Name       string  `json:"name"`
	IdsSkill   []int64 `json:"idsSkill"`
	Dispatched bool    `json:"dispatched"`
}

type UpdateAgentRunStageSkillsReq struct {
	Stage    string  `json:"stage"    binding:"required,oneof=brainstorming design implementation_plan implementation"`
	IdsSkill []int64 `json:"idsSkill"`
}
