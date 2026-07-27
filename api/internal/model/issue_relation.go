package model

import "time"

// Relation type constants, used across repository, controller, and binding validation.
// binding tags (oneof=...) can't reference constants — keep them in sync manually.
const (
	RelationTypeHierarchy  = "hierarchy"
	RelationTypeSchedule   = "schedule"
	RelationTypeDuplicates = "duplicates"
	RelationTypeRelatesTo  = "relates_to"

	RelationSubTypeFinishToStart  = "finish_to_start"
	RelationSubTypeStartToStart   = "start_to_start"
	RelationSubTypeFinishToFinish = "finish_to_finish"
	RelationSubTypeStartToFinish  = "start_to_finish"

	RelationDirectionOutbound = "outbound"
	RelationDirectionInbound  = "inbound"
)

type IssueRelation struct {
	IdIssueRelation int64     `json:"idIssueRelation" db:"id_issue_relation"`
	IdProject       int64     `json:"idProject"       db:"id_project"`
	IdIssueFrom     int64     `json:"idIssueFrom"     db:"id_issue_from"`
	IdIssueTo       int64     `json:"idIssueTo"       db:"id_issue_to"`
	RelationType    string    `json:"relationType"    db:"relation_type"`
	RelationSubType *string   `json:"relationSubType" db:"relation_sub_type"`
	LagMinutes      *int64    `json:"lagMinutes"      db:"lag_minutes"`
	CreatedAt       time.Time `json:"createdAt"       db:"created_at"`
	CreatedBy       int64     `json:"createdBy"       db:"created_by"`
}

type ReadIssueRelationRes struct {
	IdIssueRelation int64            `json:"idIssueRelation"`
	RelationType    string           `json:"relationType"`
	RelationSubType *string          `json:"relationSubType"`
	LagMinutes      *int64           `json:"lagMinutes"`
	Direction       string           `json:"direction"`
	Label           string           `json:"label"`
	InverseLabel    string           `json:"inverseLabel"`
	From            IssueRelationRef `json:"from"`
	To              IssueRelationRef `json:"to"`
	CreatedAt       time.Time        `json:"createdAt"`
	CreatedBy       int64            `json:"createdBy"`
}

type IssueRelationRef struct {
	IdIssuePublic int64     `json:"idIssuePublic"`
	Title         string    `json:"title"`
	IdSeverity    *int64    `json:"idSeverity"`
	IdState       *int64    `json:"idState"`
	AssignedTo    *int64    `json:"assignedTo"`
	UpdateAt      time.Time `json:"updateAt"`
	QualityScore  *int64    `json:"qualityScore"`
}

// LoadRelationsFilter drives LoadRelations. IdsProject is required;
// IdsIssue is optional — omit for all relations in the projects.
type LoadRelationsFilter struct {
	IdsProject []int64
	IdsIssue   []int64
}

type CreateIssueRelationReq struct {
	IdIssuePublicTo int64   `json:"idIssuePublicTo" binding:"required"`
	RelationType    string  `json:"relationType"    binding:"required,oneof=hierarchy schedule duplicates relates_to"`
	RelationSubType *string `json:"relationSubType" binding:"omitempty,oneof=finish_to_start start_to_start finish_to_finish start_to_finish"`
	LagMinutes      *int64  `json:"lagMinutes"      binding:"omitempty"`
}
