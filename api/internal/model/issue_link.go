package model

type IssueLinkOption struct {
	IdProject     int64  `json:"idProject" db:"id_project"`
	ProjectName   string `json:"projectName" db:"project_name"`
	IdIssuePublic int64  `json:"idIssuePublic" db:"id_issue_public"`
	Title         string `json:"title" db:"title"`
}

type IssueLinkRef struct {
	IdProject     int64 `json:"idProject" binding:"required"`
	IdIssuePublic int64 `json:"idIssuePublic" binding:"required"`
}

type IssueLinkInfoReq struct {
	Refs []IssueLinkRef `json:"refs" binding:"required,max=100,dive"`
}

type IssueLinkIssue struct {
	IdProject     int64  `json:"idProject" db:"id_project"`
	IdIssuePublic int64  `json:"idIssuePublic" db:"id_issue_public"`
	Title         string `json:"title" db:"title"`
	IdState       *int64 `json:"idState" db:"id_state"`
	IdSeverity    *int64 `json:"idSeverity" db:"id_severity"`
	IdIssueType   *int64 `json:"idIssueType" db:"id_issue_type"`
	AssignedTo    *int64 `json:"assignedTo" db:"assigned_to"`
}

type IssueLinkProject struct {
	IdProject int64  `json:"idProject"`
	Name      string `json:"name"`
}

type IssueLinkUser struct {
	IdUser         int64   `json:"idUser"`
	Name           string  `json:"name"`
	ColorAvatarBg  string  `json:"colorAvatarBg"`
	AvatarInitials *string `json:"avatarInitials,omitempty"`
}

type IssueLinkInfoRes struct {
	Issues     []*IssueLinkIssue  `json:"issues"`
	Projects   []IssueLinkProject `json:"projects"`
	States     []*State           `json:"states"`
	Severities []*Severity        `json:"severities"`
	IssueTypes []*IssueType       `json:"issueTypes"`
	Users      []IssueLinkUser    `json:"users"`
}
