package model

import "time"

type Issue struct {
	IdIssue          int64      `json:"idIssue" db:"id_issue"`
	IdIssuePublic    int64      `json:"idIssuePublic" db:"id_issue_public"`
	IdProject        int64      `json:"idProject" db:"id_project"`
	IdState          *int64     `json:"idState" db:"id_state"`
	IdSeverity       *int64     `json:"idSeverity" db:"id_severity"`
	Title            string     `json:"title" db:"title"`
	Description      string     `json:"description" db:"description"`
	CreateAt         time.Time  `json:"createAt" db:"create_at"`
	UpdateAt         time.Time  `json:"updateAt" db:"update_at"`
	CreateBy         int64      `json:"createBy" db:"create_by"`
	UpdateBy         int64      `json:"updateBy" db:"update_by"`
	AssignedTo       *int64     `json:"assignedTo" db:"assigned_to"`
	Tracked          int64      `json:"tracked" db:"tracked"`
	Estimated        int64      `json:"estimated" db:"estimated"`
	ScheduledAt      *time.Time `json:"scheduledAt" db:"scheduled_at"`
	QualityScore     *int       `json:"qualityScore" db:"quality_score"`
	IdempotencyKey   *string    `json:"-" db:"-"`
	IdGitIntegration *int64     `json:"idGitIntegration,omitempty" db:"id_git_integration"`
	MrId             *string    `json:"mrId,omitempty"             db:"mr_id"`
	RelationCount    int        `json:"relationCount"              db:"relation_count"`
	GanttRank        *string    `json:"ganttRank"                 db:"gantt_rank"`
	IdSprint         *int64     `json:"idSprint"                  db:"id_sprint"`
	Points           *int       `json:"points"                    db:"points"`
	CarryoverCount   int        `json:"carryoverCount"            db:"carryover_count"`
}

type LoadIssuesFilter struct {
	IdProject          int64
	IdsIssue           []int64
	IdsIssuePublic     []int64
	IdsSeverity        []int64
	SeverityUnset      bool
	IdsState           []int64
	StateUnset         bool
	IdsAssignedTo      []int64
	AssignedToUnset    bool
	IdSprint           *int64 // filter to a single sprint (kanban sprint scope)
	SprintUnset        bool   // when true, filter to id_sprint IS NULL (the Backlog tab)
	Title              string
	CreateAtFrom       time.Time
	CreateAtTo         time.Time
	UpdateAtFrom       time.Time
	UpdateAtTo         time.Time
	CreateAtWithin     time.Duration
	UpdateAtWithin     time.Duration
	ScheduledAtFrom    time.Time
	ScheduledAtTo      time.Time
	ScheduledAtUnset   bool    // when true, filter to scheduled_at IS NULL
	AssignedToNull     bool    // when true, filter to assigned_to IS NULL (unassigned-only; for swimlane cell paging)
	Search             *string // full-text search via tsvector (takes precedence over Title)
	ExcludeFinalStates bool    // exclude issues in states where final = true
	Limit              *int64  // max rows; nil = no LIMIT
	Offset             *int64
	Cursor             *string // keyset pagination cursor (opaque); takes precedence over Offset
	Order              *Order
}
