package model

type AgentRunGitRepo struct {
	CloneUrl       string `json:"cloneUrl"`
	Host           string `json:"host"`
	RepoPath       string `json:"repoPath"`
	BaseBranch     string `json:"baseBranch"`
	EncryptedToken string `json:"encryptedToken"`
}
