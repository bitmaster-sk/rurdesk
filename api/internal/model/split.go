package model

// ProposedIssue represents a single child issue proposed by the AI split operation.
type ProposedIssue struct {
	Title            string `json:"title" binding:"required,max=100"`
	Description      string `json:"description"`
	EstimatedMinutes *int64 `json:"estimatedMinutes,omitempty"`
}

// SplitPreviewReq is the request body for the split preview endpoint.
type SplitPreviewReq struct {
	Hint string `json:"hint"`
}

// SplitPreviewRes contains the proposed child issues from a split preview.
type SplitPreviewRes struct {
	Children []ProposedIssue `json:"children"`
}

// SplitAcceptReq is the request body for accepting a split operation.
type SplitAcceptReq struct {
	Children []ProposedIssue `json:"children" binding:"required,min=1"`
}

// SplitAcceptRes contains the full Issue objects created from the split.
type SplitAcceptRes struct {
	Children []*Issue `json:"children"`
}
