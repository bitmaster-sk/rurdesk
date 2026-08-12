package model

import "time"

type SprintVelocity struct {
	IdSprint   int64     `json:"idSprint"   db:"id_sprint"`
	Name       string    `json:"name"       db:"name"`
	EndAt      time.Time `json:"endAt"      db:"end_at"`
	DonePoints int       `json:"donePoints" db:"done_points"`
	DoneIssues int       `json:"doneIssues" db:"done_issues"`
	Frozen     bool      `json:"frozen"     db:"frozen"`
}
