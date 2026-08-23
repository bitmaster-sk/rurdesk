package model

type Project struct {
	IdProject          int64  `json:"idProject" db:"id_project"`
	Name               string `json:"name" db:"name"`
	Color              string `json:"color" db:"color"`
	IdStateDefault     *int64 `json:"idStateDefault" db:"id_state_default"`
	IdSeverityDefault  *int64 `json:"idSeverityDefault" db:"id_severity_default"`
	IdIssueTypeDefault *int64 `json:"idIssueTypeDefault" db:"id_issue_type_default"`
}

type CreateProjectReq struct {
	Name   string `json:"name" binding:"required,max=250"`
	Color  string `json:"color" binding:"omitempty,max=32"`
	IdTeam *int64 `json:"idTeam"`
}

type EditProjectReq struct {
	IdProject          int64  `json:"idProject" binding:"required"`
	Name               string `json:"name" binding:"required,max=250"`
	Color              string `json:"color" binding:"omitempty,max=32"`
	IdTeam             *int64 `json:"idTeam"`
	IdStateDefault     *int64 `json:"idStateDefault"    binding:"omitempty"`
	IdSeverityDefault  *int64 `json:"idSeverityDefault" binding:"omitempty"`
	IdIssueTypeDefault *int64 `json:"idIssueTypeDefault" binding:"omitempty"`
}
