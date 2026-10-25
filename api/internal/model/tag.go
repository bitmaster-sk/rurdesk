package model

type Tag struct {
	IdTag      int64  `json:"idTag" db:"id_tag"`
	IdProject  int64  `json:"idProject" db:"id_project"`
	Name       string `json:"name" db:"name"`
	Color      string `json:"color" db:"color"`
	OrderRank  int    `json:"orderRank" db:"order_rank"`
	IssueCount int    `json:"issueCount" db:"issue_count"`
}

type CreateTagReq struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name" binding:"required,max=40"`
	Color     string `json:"color" binding:"required,max=20"`
}

type EditTagReq struct {
	IdTag     int64  `json:"idTag"`
	IdProject int64  `json:"idProject"`
	Name      string `json:"name" binding:"required,max=40"`
	Color     string `json:"color" binding:"required,max=20"`
	OrderRank int    `json:"orderRank"`
}
