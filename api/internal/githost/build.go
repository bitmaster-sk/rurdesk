package githost

import "github.com/bitmaster-sk/rurdesk/api/internal/model"

// BuildFromIntegration decrypts the integration's stored token and constructs the
// matching GitHost adapter. Shared by the git-integration read paths and the
// agent-run PR-creation path so token handling lives in one place.
func BuildFromIntegration(integration *model.GitIntegration) (GitHost, error) {
	key, err := LoadEncryptionKey()
	if err != nil {
		return nil, err
	}
	token, err := Decrypt(key, integration.TokenNonce, integration.AccessTokenEnc)
	if err != nil {
		return nil, err
	}
	return NewGitHost(integration.HostType, integration.BaseUrl, integration.RepoPath, string(token))
}
