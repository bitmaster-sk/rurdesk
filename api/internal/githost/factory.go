package githost

import (
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// NewGitHost creates a GitHost adapter for the given host type.
func NewGitHost(hostType, baseUrl, repoPath, token string) (GitHost, error) {
	switch hostType {
	case model.HostTypeGitHub:
		return NewGitHubHost(baseUrl, repoPath, token), nil
	case model.HostTypeGitLab:
		return NewGitLabHost(baseUrl, repoPath, token), nil
	case model.HostTypeGitea:
		return NewGiteaHost(baseUrl, repoPath, token), nil
	default:
		return nil, fmt.Errorf("unsupported host type: %s", hostType)
	}
}
