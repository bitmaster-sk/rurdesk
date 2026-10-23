package model

type Issue struct {
	IdIssuePublic int64  `json:"idIssuePublic"`
	Title         string `json:"title"`
	Description   string `json:"description"`
}
