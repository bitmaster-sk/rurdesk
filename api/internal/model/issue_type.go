package model

type IssueType struct {
	IdIssueType int64  `json:"idIssueType" db:"id_issue_type"`
	IdProject   int64  `json:"idProject" db:"id_project"`
	Name        string `json:"name" db:"name"`
	Protected   bool   `json:"protected" db:"protected"`
	OrderRank   int    `json:"orderRank" db:"order_rank"`
}

type IssueTypeUsage struct {
	Issues           int  `json:"issues"`
	IsProjectDefault bool `json:"isProjectDefault"`
}

type CreateIssueTypeReq struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name" binding:"required,max=20"`
}

type EditIssueTypeReq struct {
	IdIssueType int64  `json:"idIssueType"`
	IdProject   int64  `json:"idProject"`
	Name        string `json:"name" binding:"required,max=20"`
	OrderRank   int    `json:"orderRank"`
}
