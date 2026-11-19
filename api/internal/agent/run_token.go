package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/jackc/pgx/v5"
)

// MintRunToken returns a fresh run token and the hash stored on the run. Only the hash is persisted; the raw token goes to the gateway.
func MintRunToken() (string, []byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("generating run token: %w", err)
	}
	raw := extctx.RunTokenPrefix + hex.EncodeToString(secret)
	return raw, HashRunToken(raw), nil
}

func HashRunToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// RunTokenAuth resolves a run token to the agent and the projects its turn may read.
type RunTokenAuth struct {
	runRepo          *repository.AgentRunRepository
	conversationRepo *repository.AgentConversationRepository
	userRepo         *repository.UserRepository
	conversationSvc  *service.AgentConversationService
}

func NewRunTokenAuth(
	runRepo *repository.AgentRunRepository,
	conversationRepo *repository.AgentConversationRepository,
	userRepo *repository.UserRepository,
	conversationSvc *service.AgentConversationService,
) *RunTokenAuth {
	return &RunTokenAuth{
		runRepo:          runRepo,
		conversationRepo: conversationRepo,
		userRepo:         userRepo,
		conversationSvc:  conversationSvc,
	}
}

// LookupRunToken returns nil, nil, nil when the token does not authenticate (unknown, expired,
// no active task, or an agent or owner that can no longer act). The scope is recomputed on every request.
func (a *RunTokenAuth) LookupRunToken(ctx context.Context, rawToken string) (*model.User, *extctx.RunScope, error) {
	run, err := a.runRepo.LoadByRunTokenHash(ctx, HashRunToken(rawToken))
	if err != nil {
		return nil, nil, fmt.Errorf("looking up run token: %w", err)
	}
	if run == nil {
		return nil, nil, nil
	}
	conversation, err := a.conversationRepo.LoadConversationByRun(ctx, run.IdRun)
	if err != nil {
		return nil, nil, fmt.Errorf("loading conversation of run %d: %w", run.IdRun, err)
	}
	if conversation == nil || conversation.IdUserAgent != run.IdUserAgent {
		return nil, nil, nil
	}

	agentUser, err := a.loadActiveUser(ctx, run.IdUserAgent)
	if err != nil || agentUser == nil || !agentUser.IsAgent {
		return nil, nil, err
	}
	owner, err := a.loadActiveUser(ctx, conversation.IdUser)
	if err != nil || owner == nil {
		return nil, nil, err
	}

	idsProject, err := a.conversationSvc.LoadReadableProjects(ctx, conversation)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving projects of conversation %d: %w", conversation.IdConversation, err)
	}
	// A chat turn only reads.
	return agentUser, &extctx.RunScope{
		IdRun:      run.IdRun,
		IdsProject: idsProject,
		MaxRole:    model.RoleViewer,
	}, nil
}

// loadActiveUser returns nil for a missing or disabled user.
func (a *RunTokenAuth) loadActiveUser(ctx context.Context, idUser int64) (*model.User, error) {
	user, err := a.userRepo.LoadUser(ctx, idUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading user %d: %w", idUser, err)
	}
	if !user.IsActive {
		return nil, nil
	}
	return user, nil
}
