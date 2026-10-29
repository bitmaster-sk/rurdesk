package model

type ReleaseIssue struct {
	IdIssuePublic int64  `json:"idIssuePublic"`
	Title         string `json:"title"`
}

type ReleaseSection struct {
	Name   *string        `json:"name"`
	Issues []ReleaseIssue `json:"issues"`
}

type ReleasePromptContext struct {
	Name       string           `json:"name"`
	Tag        string           `json:"tag"`
	Sections   []ReleaseSection `json:"sections"`
	OpenIssues []ReleaseIssue   `json:"openIssues"`
}
