package agent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

const emptyRepoBaseBranch = "main"

var ErrProjectHasNoGitIntegration = errors.New("the project has no git integration; add one in the project settings so the agent knows which repository to work in")

// Stamped only at the first dispatch: every stage and the PR must stay on the same repo and base branch.
func (d *Dispatcher) agentRunGitRepo(ctx context.Context, run *model.AgentRun) (*model.AgentRunGitRepo, error) {
	integration, err := d.agentRunGitIntegration(ctx, run)
	if err != nil {
		return nil, err
	}
	token, err := githost.DecryptToken(integration)
	if err != nil {
		return nil, fmt.Errorf("decrypting the git integration token: %w", err)
	}

	baseBranch := ""
	if run.BaseBranch != nil {
		baseBranch = *run.BaseBranch
	}
	if baseBranch == "" || run.IdGitIntegration == nil {
		if baseBranch, err = integrationBaseBranch(ctx, integration, token); err != nil {
			return nil, err
		}
		if err := d.agentRunRepo.SetGitIntegrationAndBaseBranch(ctx, run.IdRun, integration.IdGitIntegration, baseBranch); err != nil {
			return nil, err
		}
		run.IdGitIntegration = &integration.IdGitIntegration
		run.BaseBranch = &baseBranch
	}

	host, err := hostOfBaseUrl(integration.BaseUrl)
	if err != nil {
		return nil, err
	}
	encryptedToken, err := d.encryptGitTokenForGateway(ctx, run, token)
	if err != nil {
		return nil, err
	}
	return &model.AgentRunGitRepo{
		CloneUrl:       cloneUrl(integration.BaseUrl, integration.RepoPath),
		Host:           host,
		RepoPath:       strings.Trim(integration.RepoPath, "/"),
		BaseBranch:     baseBranch,
		EncryptedToken: encryptedToken,
	}, nil
}

func (d *Dispatcher) agentRunGitIntegration(ctx context.Context, run *model.AgentRun) (*model.GitIntegration, error) {
	if run.IdGitIntegration != nil {
		integration, err := d.gitIntRepo.LoadByID(ctx, *run.IdGitIntegration, run.IdProject)
		if err != nil {
			return nil, fmt.Errorf("loading git integration %d: %w", *run.IdGitIntegration, err)
		}
		if integration == nil {
			return nil, fmt.Errorf("git integration %d of the run no longer exists; restart the run", *run.IdGitIntegration)
		}
		return integration, nil
	}
	integration, err := d.gitIntRepo.LoadByProject(ctx, run.IdProject)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, ErrProjectHasNoGitIntegration
	}
	return integration, nil
}

func (d *Dispatcher) encryptGitTokenForGateway(ctx context.Context, run *model.AgentRun, token string) (string, error) {
	gateway, err := d.agentGwRepo.LoadByAgentUser(ctx, run.IdUserAgent)
	if err != nil {
		return "", fmt.Errorf("loading gateway: %w", err)
	}
	if gateway == nil {
		return "", fmt.Errorf("no gateway configured for agent %d", run.IdUserAgent)
	}
	return EncryptGitToken(gateway.WebhookSecret, token)
}

func integrationBaseBranch(ctx context.Context, integration *model.GitIntegration, token string) (string, error) {
	if integration.BaseBranch != nil && *integration.BaseBranch != "" {
		return *integration.BaseBranch, nil
	}
	host, err := githost.NewGitHost(integration.HostType, integration.BaseUrl, integration.RepoPath, token)
	if err != nil {
		return "", fmt.Errorf("building git host: %w", err)
	}
	branch, err := host.DefaultBranch(ctx)
	if errors.Is(err, githost.ErrNoDefaultBranch) {
		// The gateway seeds an empty repository with this branch.
		return emptyRepoBaseBranch, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolving the default branch of %s (set Base branch on the git integration to skip this): %w", integration.RepoPath, err)
	}
	return branch, nil
}

func cloneUrl(baseUrl, repoPath string) string {
	return strings.TrimRight(baseUrl, "/") + "/" + strings.Trim(repoPath, "/") + ".git"
}

func hostOfBaseUrl(baseUrl string) (string, error) {
	parsed, err := url.Parse(baseUrl)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("git integration base URL %q has no host", baseUrl)
	}
	return parsed.Host, nil
}
