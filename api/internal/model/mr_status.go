package model

// MrStatusNotice is the WebSocket payload for an open manual MR that changed
// CI/approval/head while it is still open. It mirrors githost.Status with the
// identifiers clients need to route it to the right issue.
type MrStatusNotice struct {
	IdIssue          int64  `json:"idIssue"`
	IdGitIntegration int64  `json:"idGitIntegration"`
	IdMr             string `json:"idMr"`
	State            string `json:"state"`
	Approved         bool   `json:"approved"`
	CiStatus         string `json:"ciStatus"`
	WebUrl           string `json:"webUrl"`
	HeadSHA          string `json:"headSha"`
}
