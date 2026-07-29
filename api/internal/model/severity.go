package model

type Severity struct {
	IdSeverity int64  `json:"idSeverity" db:"id_severity"`
	IdProject  int64  `json:"idProject" db:"id_project"`
	Title      string `json:"title" db:"title"`
	Color      string `json:"color" db:"color"`
	Protected  bool   `json:"protected" db:"protected"`
	OrderRank  int    `json:"orderRank" db:"order_rank"`
}

// SeverityUsage mirrors StateUsage for severities (no agent-phase mapping).
type SeverityUsage struct {
	Issues           int  `json:"issues"`
	IsProjectDefault bool `json:"isProjectDefault"`
	// Guard input only — see StateUsage.Mappings.
	Mappings int `json:"-"`
}

type CreateSeverityReq struct {
	IdProject int64  `json:"idProject"`
	Title     string `json:"title" binding:"required,max=20"`
	Color     string `json:"color" binding:"required,max=20"`
}

type EditSeverityReq struct {
	IdSeverity int64  `json:"idSeverity"`
	IdProject  int64  `json:"idProject"`
	Title      string `json:"title" binding:"required,max=20"`
	Color      string `json:"color" binding:"required,max=20"`
	OrderRank  int    `json:"orderRank"`
}
