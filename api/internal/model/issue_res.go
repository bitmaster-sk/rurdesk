package model

// IssuesPageRes is the flat-list envelope. NextCursor == nil means no further pages.
type IssuesPageRes struct {
	Items      []*Issue `json:"items"`
	NextCursor *string  `json:"nextCursor"`
	Total      int      `json:"total"`
}

// IssueGroupRes is one group of the grouped (kanban) representation.
type IssueGroupRes struct {
	Key        map[string]any `json:"key"`
	Items      []*Issue       `json:"items"`
	Total      int            `json:"total"`
	NextCursor *string        `json:"nextCursor"`
}
