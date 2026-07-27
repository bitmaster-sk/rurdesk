package model

// ProjectBuilderGenerateReq is the request body for the generate endpoint.
type ProjectBuilderGenerateReq struct {
	Description string `json:"description" binding:"required,min=10"`
	IdState     *int64 `json:"idState"`
	IdSeverity  *int64 `json:"idSeverity"`
}

// ProjectBuilderGenerateRes is returned by the generate endpoint; the client also holds this shape in staging.
type ProjectBuilderGenerateRes struct {
	Issues  []ProjectBuilderIssue `json:"issues"`
	Summary string                `json:"summary"`
}

// ProjectBuilderAcceptReq is the accept endpoint's request body: the client's (possibly edited) staging state.
type ProjectBuilderAcceptReq struct {
	Issues []ProjectBuilderIssue `json:"issues" binding:"required"`
}

// ProjectBuilderAcceptRes contains the full issue objects created on accept.
type ProjectBuilderAcceptRes struct {
	Issues []*Issue `json:"issues"`
}

// ProjectBuilderIssue is the shared issue shape used in the generate response and accept request.
type ProjectBuilderIssue struct {
	Ref                string                   `json:"ref"`
	Title              string                   `json:"title" binding:"required,max=100"`
	Description        string                   `json:"description"`
	EstimatedMinutes   int64                    `json:"estimatedMinutes"`
	IdState            *int64                   `json:"idState"`
	IdSeverity         *int64                   `json:"idSeverity"`
	HierarchyParentRef string                   `json:"hierarchyParentRef"`
	ScheduleRelations  []ProjectBuilderRelation `json:"scheduleRelations"`
}

// ProjectBuilderRelation describes a schedule relation from one issue to another.
type ProjectBuilderRelation struct {
	Ref  string `json:"ref"`
	Type string `json:"type"`
}
