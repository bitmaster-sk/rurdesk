package model

type State struct {
	IdState   int64  `json:"idState" db:"id_state"`
	IdProject int64  `json:"idProject" db:"id_project"`
	Name      string `json:"name" db:"name"`
	Start     bool   `json:"start" db:"start"`
	Final     bool   `json:"final" db:"final"`
	Protected bool   `json:"protected" db:"protected"`
	OrderRank int    `json:"orderRank" db:"order_rank"`
}

// StateUsage reports everything that still points at a state within one project.
type StateUsage struct {
	Issues           int  `json:"issues"`
	IsProjectDefault bool `json:"isProjectDefault"`
	AgentPhases      int  `json:"agentPhases"`
	// mapping count across ALL projects; guard input only, never serialized
	Mappings int `json:"-"`
}

type CreateStateReq struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name" binding:"required,max=20"`
	Start     bool   `json:"start"`
	Final     bool   `json:"final"`
}

type EditStateReq struct {
	IdState   int64  `json:"idState"`
	IdProject int64  `json:"idProject"`
	Name      string `json:"name" binding:"required,max=20"`
	Start     bool   `json:"start"`
	Final     bool   `json:"final"`
	OrderRank int    `json:"orderRank"`
}
