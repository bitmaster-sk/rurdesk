package model

import "time"

const (
	HostTypeGitHub = "github"
	HostTypeGitLab = "gitlab"
	HostTypeGitea  = "gitea"
)

var ValidHostTypes = map[string]bool{
	HostTypeGitHub: true,
	HostTypeGitLab: true,
	HostTypeGitea:  true,
}

type GitIntegration struct {
	IdGitIntegration int64     `json:"-" db:"id_git_integration"`
	IdProject        int64     `json:"-" db:"id_project"`
	Name             string    `json:"-" db:"name"`
	HostType         string    `json:"-" db:"host_type"`
	BaseUrl          string    `json:"-" db:"base_url"`
	RepoPath         string    `json:"-" db:"repo_path"`
	AccessTokenEnc   []byte    `json:"-" db:"access_token_enc"`
	TokenNonce       []byte    `json:"-" db:"token_nonce"`
	CreatedAt        time.Time `json:"-" db:"created_at"`
	UpdatedAt        time.Time `json:"-" db:"updated_at"`
}

type GitIntegrationRes struct {
	IdGitIntegration int64     `json:"idGitIntegration"`
	IdProject        int64     `json:"idProject"`
	Name             string    `json:"name"`
	HostType         string    `json:"hostType"`
	BaseUrl          string    `json:"baseUrl"`
	RepoPath         string    `json:"repoPath"`
	HasToken         bool      `json:"hasToken"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (g *GitIntegration) ToRes() *GitIntegrationRes {
	return &GitIntegrationRes{
		IdGitIntegration: g.IdGitIntegration,
		IdProject:        g.IdProject,
		Name:             g.Name,
		HostType:         g.HostType,
		BaseUrl:          g.BaseUrl,
		RepoPath:         g.RepoPath,
		HasToken:         len(g.AccessTokenEnc) > 0,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
	}
}

type CreateGitIntegrationReq struct {
	Name        string `json:"name"        binding:"required,max=100"`
	HostType    string `json:"hostType"    binding:"required,oneof=github gitlab gitea"`
	BaseUrl     string `json:"baseUrl"     binding:"required,url"`
	RepoPath    string `json:"repoPath"    binding:"required,max=500"`
	AccessToken string `json:"accessToken" binding:"required,min=1,max=500"`
}

type UpdateGitIntegrationReq struct {
	Name        string `json:"name"        binding:"required,max=100"`
	HostType    string `json:"hostType"    binding:"required,oneof=github gitlab gitea"`
	BaseUrl     string `json:"baseUrl"     binding:"required,url"`
	RepoPath    string `json:"repoPath"    binding:"required,max=500"`
	AccessToken string `json:"accessToken" binding:"omitempty,min=1,max=500"`
}
