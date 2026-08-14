package model

import "time"

type Sprint struct {
	IdSprint  int64     `json:"idSprint"  db:"id_sprint"`
	IdProject int64     `json:"idProject" db:"id_project"`
	Name      string    `json:"name"      db:"name"`
	StartAt   time.Time `json:"startAt"   db:"start_at"`
	EndAt     time.Time `json:"endAt"     db:"end_at"`
	State     string    `json:"state"     db:"state"`
}

type CreateSprintReq struct {
	Name    string     `json:"name"    binding:"omitempty,max=60"`
	StartAt *time.Time `json:"startAt"`
	EndAt   *time.Time `json:"endAt"`
}

type EditSprintReq struct {
	Name    string    `json:"name"    binding:"required,max=60"`
	StartAt time.Time `json:"startAt" binding:"required"`
	EndAt   time.Time `json:"endAt"   binding:"required"`
}

type AssignSprintReq struct {
	IdSprint *int64 `json:"idSprint"` // explicit null clears the assignment
}

type SprintStats struct {
	TotalPoints    int `json:"totalPoints"`
	DonePoints     int `json:"donePoints"` // velocity: points in a final state
	StartPoints    int `json:"startPoints"`
	ProgressPoints int `json:"progressPoints"`
	TotalIssues    int `json:"totalIssues"`
	DoneIssues     int `json:"doneIssues"`
	StartIssues    int `json:"startIssues"`
	ProgressIssues int `json:"progressIssues"`
	PointedIssues  int `json:"pointedIssues"`

	RolledOverIssues    *int `json:"rolledOverIssues,omitempty"`
	FrozenTotalPoints   *int `json:"frozenTotalPoints,omitempty"`
	FrozenDonePoints    *int `json:"frozenDonePoints,omitempty"`
	FrozenTotalIssues   *int `json:"frozenTotalIssues,omitempty"`
	FrozenDoneIssues    *int `json:"frozenDoneIssues,omitempty"`
	FrozenPointedIssues *int `json:"frozenPointedIssues,omitempty"`
}

type SprintBurndown struct {
	IdSprint int64               `json:"idSprint"`
	StartAt  time.Time           `json:"startAt"`
	EndAt    time.Time           `json:"endAt"`
	State    string              `json:"state"`
	Days     []SprintBurndownDay `json:"days"`
}

type SprintBurndownDay struct {
	Day             time.Time `json:"day"`
	TotalPoints     *int      `json:"totalPoints"`
	DonePoints      *int      `json:"donePoints"`
	RemainingPoints *int      `json:"remainingPoints"`
	TotalIssues     *int      `json:"totalIssues"`
	DoneIssues      *int      `json:"doneIssues"`
	RemainingIssues *int      `json:"remainingIssues"`
	Snapshot        bool      `json:"snapshot"`
}

type SprintSnapshot struct {
	IdSprint      int64     `json:"idSprint"      db:"id_sprint"`
	Day           time.Time `json:"day"           db:"day"`
	TotalPoints   int       `json:"totalPoints"   db:"total_points"`
	DonePoints    int       `json:"donePoints"    db:"done_points"`
	TotalIssues   int       `json:"totalIssues"   db:"total_issues"`
	DoneIssues    int       `json:"doneIssues"    db:"done_issues"`
	PointedIssues int       `json:"pointedIssues" db:"pointed_issues"`
}
