package model

import "time"

// QualityDimensions holds per-dimension quality scores (each 0–100).
type QualityDimensions struct {
	Clarity       int `json:"clarity"`
	Completeness  int `json:"completeness"`
	Actionability int `json:"actionability"`
	Scope         int `json:"scope"`
	Metadata      int `json:"metadata"`
}

// QualitySuggestion represents a single AI quality suggestion.
type QualitySuggestion struct {
	Type        string `json:"type"`
	Explanation string `json:"explanation"`
	NewValue    string `json:"newValue,omitempty"`
}

// IssueQuality is the DB model for issues.issue_quality.
type IssueQuality struct {
	IdIssue     int64     `db:"id_issue"`
	Score       int       `db:"score"`
	ContentHash string    `db:"content_hash"`
	CheckedAt   time.Time `db:"checked_at"`
	CheckedBy   int64     `db:"checked_by"`
}

// QualityCheckReq is the HTTP request body for a quality check.
type QualityCheckReq struct {
	Title       string `json:"title" binding:"required,max=100"`
	Description string `json:"description"`
}

// QualityCheckRes is the HTTP response for a quality check.
type QualityCheckRes struct {
	Score       int                 `json:"score"`
	Dimensions  QualityDimensions   `json:"dimensions"`
	Problems    []string            `json:"problems"`
	Suggestions []QualitySuggestion `json:"suggestions"`
	CheckedAt   time.Time           `json:"checkedAt"`
	FromCache   bool                `json:"fromCache"`
}
