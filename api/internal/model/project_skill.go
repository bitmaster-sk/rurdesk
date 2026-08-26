package model

type ProjectSkill struct {
	IdProject int64  `json:"idProject" db:"id_project"`
	IdSkill   int64  `json:"idSkill"   db:"id_skill"`
	Stage     string `json:"stage"     db:"stage"`
}

type UpdateProjectSkillReq struct {
	IdSkill int64  `json:"idSkill" binding:"required"`
	Stage   string `json:"stage"   binding:"required,oneof=brainstorming design implementation_plan implementation"`
}
