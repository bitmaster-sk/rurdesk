package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/email"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/password"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidPassword is returned when the current password does not match
// (or the user cannot use password auth at all, e.g. agents).
var ErrInvalidPassword = errors.New("invalid password")

const bootstrapLockKey int64 = 2 << 48

type UserService struct {
	userRepo    *repository.UserRepository
	lockRepo    *repository.AdvisoryLockRepository
	cache       *redis.Client
	pool        *pgxpool.Pool
	loginPolicy func() authpolicy.PasswordLoginPolicy
}

func NewUserService(repo *repository.UserRepository, lockRepo *repository.AdvisoryLockRepository, cache *redis.Client, pool *pgxpool.Pool, loginPolicy func() authpolicy.PasswordLoginPolicy) *UserService {
	return &UserService{userRepo: repo, lockRepo: lockRepo, cache: cache, pool: pool, loginPolicy: loginPolicy}
}

func (s *UserService) Register(ctx context.Context, user *model.User) (*model.User, error) {
	return s.userRepo.InsertUser(ctx, user)
}

func (s *UserService) RegisterFirst(ctx context.Context, user *model.User) (*model.User, bool, error) {
	var created bool
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.lockRepo.Lock(ctx, bootstrapLockKey); err != nil {
			return err
		}
		count, err := s.userRepo.CountUsers(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if _, err := s.userRepo.InsertUser(ctx, user); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil || !created {
		return nil, false, err
	}
	return user, true, nil
}

func (s *UserService) LoadByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.userRepo.LoadUserByEmail(ctx, email)
}

func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	return s.userRepo.CountUsers(ctx)
}

func (s *UserService) ListUsers(ctx context.Context) ([]*model.User, error) {
	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	idsWithIdentity, err := s.userRepo.LoadIdsWithIdentity(ctx)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		user.HasSsoIdentity = slices.Contains(idsWithIdentity, user.IdUser)
	}
	return users, nil
}

func (s *UserService) Login(ctx context.Context, rawEmail, plainPassword string, hasExtendedSessionLifetime bool) (string, error) {
	user, err := s.loadForLogin(ctx, rawEmail)
	// Hashing on every rejection keeps unknown, disabled, passwordless and
	// policy-blocked accounts as slow as a wrong password, so none of them leaks.
	if err != nil || !user.IsActive || user.IsPasswordless || !s.loginPolicy().Allow(ctx, user) {
		_ = password.Compare(password.Dummy(), plainPassword)
		return "", ErrInvalidPassword
	}
	if err := password.Compare(user.Password, plainPassword); err != nil {
		return "", ErrInvalidPassword
	}
	return s.CreateSession(ctx, user, hasExtendedSessionLifetime)
}

func (s *UserService) loadForLogin(ctx context.Context, rawEmail string) (*model.User, error) {
	user, err := s.userRepo.LoadUserByEmail(ctx, rawEmail)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}
	folded, err := s.userRepo.LoadUsersByEmailFold(ctx, email.Normalize(rawEmail))
	if err != nil {
		return nil, err
	}
	if len(folded) != 1 {
		return nil, repository.ErrUserNotFound
	}
	return folded[0], nil
}

func (s *UserService) CreateSession(ctx context.Context, user *model.User, isExtended bool) (string, error) {
	lifetime := constants.SessionLifetime
	if isExtended {
		lifetime = constants.SessionLifetimeExtended
	}
	if err := s.userRepo.TouchLastLogin(ctx, user.IdUser); err != nil {
		return "", err
	}
	token := uuid.New().String()
	if err := s.cache.Set(ctx, token, user, lifetime).Err(); err != nil {
		return "", err
	}
	// Index the session under the user so demote/delete can invalidate it. Stale
	// members (logged-out or expired tokens) are harmless — deleting them is a no-op.
	idxKey := sessionIndexKey(user.IdUser)
	if err := s.cache.SAdd(ctx, idxKey, token).Err(); err != nil {
		return "", err
	}
	if err := s.extendSessionIndexExpiry(ctx, idxKey, lifetime); err != nil {
		return "", err
	}
	return token, nil
}

func (s *UserService) extendSessionIndexExpiry(ctx context.Context, idxKey string, lifetime time.Duration) error {
	currentTTL, err := s.cache.TTL(ctx, idxKey).Result()
	if err != nil {
		return err
	}
	if currentTTL > lifetime {
		return nil
	}
	return s.cache.Expire(ctx, idxKey, lifetime).Err()
}

func (s *UserService) Logout(ctx context.Context, token string) error {
	return s.cache.Del(ctx, token).Err()
}

func (s *UserService) Update(ctx context.Context, token string, user model.User, name, colorAvatarBg string) error {
	if err := s.userRepo.UpdateUser(ctx, user.IdUser, name); err != nil {
		return fmt.Errorf("update user db: %w", err)
	}
	if err := s.userRepo.UpdateAvatarColor(ctx, user.IdUser, colorAvatarBg); err != nil {
		return fmt.Errorf("update avatar colour db: %w", err)
	}
	user.Name = name
	user.ColorAvatarBg = colorAvatarBg
	return s.cache.Set(ctx, token, &user, redis.KeepTTL).Err()
}

// ChangePassword verifies the current password and replaces it, then invalidates every
// other session of the user (keepToken — the caller's current session — stays alive) so
// a leaked pre-change token cannot outlive the change. Agents are rejected: API keys only.
func (s *UserService) ChangePassword(ctx context.Context, idUser int64, currentPassword, newPassword, keepToken string) error {
	user, err := s.userRepo.LoadUser(ctx, idUser)
	if err != nil {
		return fmt.Errorf("loading user %d: %w", idUser, err)
	}
	if user.IsAgent || user.IsPasswordless {
		return ErrInvalidPassword
	}
	if password.Compare(user.Password, currentPassword) != nil {
		return ErrInvalidPassword
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, idUser, hash); err != nil {
		return err
	}
	return s.InvalidateUserSessionsExcept(ctx, idUser, keepToken)
}

func (s *UserService) SetPassword(ctx context.Context, idUser int64, plain, keepToken string) error {
	hash, err := password.Hash(plain)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, idUser, hash); err != nil {
		return err
	}
	return s.InvalidateUserSessionsExcept(ctx, idUser, keepToken)
}

// InvalidateUserSessions deletes every indexed session of a user. Called on admin rights
// changes (promote/demote → force re-login with a fresh snapshot) and user deletion, so
// the 24h Redis session snapshot cannot outlive the change. Sessions predating the index
// aren't covered — they expire naturally.
func (s *UserService) InvalidateUserSessions(ctx context.Context, idUser int64) error {
	idxKey := sessionIndexKey(idUser)
	tokens, err := s.cache.SMembers(ctx, idxKey).Result()
	if err != nil {
		return err
	}
	if len(tokens) > 0 {
		if err := s.cache.Del(ctx, tokens...).Err(); err != nil {
			return err
		}
	}
	return s.cache.Del(ctx, idxKey).Err()
}

// InvalidateUserSessionsExcept deletes every indexed session of a user except keepToken
// (kept alive, left in the index). Used by self-service password change: the acting
// session survives, all others are booted. An empty keepToken invalidates every session.
func (s *UserService) InvalidateUserSessionsExcept(ctx context.Context, idUser int64, keepToken string) error {
	idxKey := sessionIndexKey(idUser)
	tokens, err := s.cache.SMembers(ctx, idxKey).Result()
	if err != nil {
		return err
	}
	toDelete := make([]string, 0, len(tokens))
	members := make([]any, 0, len(tokens))
	for _, token := range tokens {
		if token != keepToken {
			toDelete = append(toDelete, token)
			members = append(members, token)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}
	if err := s.cache.Del(ctx, toDelete...).Err(); err != nil {
		return err
	}
	return s.cache.SRem(ctx, idxKey, members...).Err()
}

func sessionIndexKey(idUser int64) string {
	return fmt.Sprintf("%s%d", constants.SessionIndexPrefix, idUser)
}
