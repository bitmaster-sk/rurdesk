package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const (
	apiKeyCacheTTL         = 5 * time.Minute
	apiKeyNegativeCacheTTL = 60 * time.Second
	apiKeyCachePrefix      = "auth:apikey:"
	rateLimitKeyPrefix     = "ratelimit:apikey:"
	lastUsedChannelSize    = 1000
)

// Namespaces the per-user create lock away from other advisory lock keys.
const userApiKeyLockKey int64 = 3 << 48

var ErrRateLimited = fmt.Errorf("rate limit exceeded")

// ErrApiKeyLimitReached means the user already holds the maximum number of
// API keys allowed by the app settings.
var ErrApiKeyLimitReached = errors.New("user api key limit reached")

type ApiKeyService struct {
	apiKeyRepo *repository.ApiKeyRepository
	lockRepo   *repository.AdvisoryLockRepository
	cache      *redis.Client
	pool       *pgxpool.Pool
	lastUsedCh chan string
	stopCh     chan struct{}
}

func NewApiKeyService(
	apiKeyRepo *repository.ApiKeyRepository,
	lockRepo *repository.AdvisoryLockRepository,
	cache *redis.Client,
	pool *pgxpool.Pool,
) *ApiKeyService {
	svc := &ApiKeyService{
		apiKeyRepo: apiKeyRepo,
		lockRepo:   lockRepo,
		cache:      cache,
		pool:       pool,
		lastUsedCh: make(chan string, lastUsedChannelSize),
		stopCh:     make(chan struct{}),
	}
	go svc.drainLastUsed()
	return svc
}

// Shutdown signals the last-used drain goroutine to exit. Buffered hashes are
// dropped — last_used_at is best-effort telemetry, not a correctness signal.
func (s *ApiKeyService) Shutdown() {
	close(s.stopCh)
}

// HashApiKey returns the hex-encoded SHA-256 hash of a raw API key.
// Used by both the service and the auth middleware.
func HashApiKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

func generateRawKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *ApiKeyService) CreateAgentApiKey(ctx context.Context, idUser int64, req *model.CreateAgentApiKeyReq) (*model.CreateApiKeyRes, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("generating agent api key: %w", err)
	}
	keyHash := HashApiKey(rawKey)
	key, err := s.apiKeyRepo.Insert(ctx, idUser, req.Name, keyHash, req.ExpiresAt, req.RateLimitOverride, true)
	if err != nil {
		return nil, fmt.Errorf("inserting agent api key: %w", err)
	}
	return &model.CreateApiKeyRes{ApiKey: *key, RawKey: rawKey}, nil
}

// GetAgentApiKey returns an agent's single API key, or nil when none exists.
func (s *ApiKeyService) GetAgentApiKey(ctx context.Context, idUser int64) (*model.ApiKey, error) {
	return s.apiKeyRepo.LoadAgentApiKey(ctx, idUser)
}

// RegenerateAgentApiKey rotates an agent's key in place, invalidating the old one
// immediately, and returns the new one-time raw key. ErrApiKeyNotFound if the
// agent has none yet.
func (s *ApiKeyService) RegenerateAgentApiKey(ctx context.Context, idUser int64) (*model.CreateApiKeyRes, error) {
	existing, err := s.apiKeyRepo.LoadAgentApiKey(ctx, idUser)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, repository.ErrApiKeyNotFound
	}
	return s.rotate(ctx, idUser, existing.IdApiKey, true)
}

// RevokeAgentApiKey deletes an agent's single key and purges its cached session.
// Absent key is not an error: revoking what is already gone is a no-op.
func (s *ApiKeyService) RevokeAgentApiKey(ctx context.Context, idUser int64) error {
	existing, err := s.apiKeyRepo.LoadAgentApiKey(ctx, idUser)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	return s.revoke(ctx, idUser, existing.IdApiKey, true)
}

// CreateUserApiKey mints a personal access token unless the user is already at
// limit, in which case it returns ErrApiKeyLimitReached. The advisory lock is what
// makes the limit hold: without it two concurrent creates both read a count below
// the limit and both insert.
func (s *ApiKeyService) CreateUserApiKey(
	ctx context.Context,
	idUser int64,
	limit int,
	req *model.CreateUserApiKeyReq,
) (*model.CreateApiKeyRes, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("generating user api key: %w", err)
	}

	var key *model.ApiKey
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.lockRepo.Lock(ctx, userApiKeyLockKey+idUser); err != nil {
			return err
		}
		count, err := s.apiKeyRepo.CountUserApiKeys(ctx, idUser)
		if err != nil {
			return err
		}
		if count >= limit {
			return ErrApiKeyLimitReached
		}
		key, err = s.apiKeyRepo.Insert(ctx, idUser, req.Name, HashApiKey(rawKey), req.ExpiresAt, nil, false)
		if err != nil {
			return fmt.Errorf("inserting user api key: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &model.CreateApiKeyRes{ApiKey: *key, RawKey: rawKey}, nil
}

func (s *ApiKeyService) ListUserApiKeys(ctx context.Context, idUser int64) ([]model.ApiKey, error) {
	return s.apiKeyRepo.LoadUserApiKeys(ctx, idUser)
}

func (s *ApiKeyService) RegenerateUserApiKey(ctx context.Context, idUser, idApiKey int64) (*model.CreateApiKeyRes, error) {
	return s.rotate(ctx, idUser, idApiKey, false)
}

func (s *ApiKeyService) RevokeUserApiKey(ctx context.Context, idUser, idApiKey int64) error {
	return s.revoke(ctx, idUser, idApiKey, false)
}

func (s *ApiKeyService) rotate(ctx context.Context, idUser, idApiKey int64, isAgent bool) (*model.CreateApiKeyRes, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}
	oldHash, key, err := s.apiKeyRepo.UpdateApiKeyHash(ctx, idUser, idApiKey, isAgent, HashApiKey(rawKey))
	if err != nil {
		return nil, err
	}
	if oldHash != "" {
		_ = s.cache.Del(ctx, apiKeyCachePrefix+oldHash).Err()
	}
	return &model.CreateApiKeyRes{ApiKey: *key, RawKey: rawKey}, nil
}

func (s *ApiKeyService) revoke(ctx context.Context, idUser, idApiKey int64, isAgent bool) error {
	keyHash, err := s.apiKeyRepo.DeleteApiKey(ctx, idUser, idApiKey, isAgent)
	if err != nil {
		return fmt.Errorf("revoking api key: %w", err)
	}
	_ = s.cache.Del(ctx, apiKeyCachePrefix+keyHash).Err()
	return nil
}

// KeyHashesByUser exposes the stored hashes of a user's keys for cache purging on
// user deletion. The raw keys cannot be derived from these hashes.
func (s *ApiKeyService) KeyHashesByUser(ctx context.Context, idUser int64) ([]string, error) {
	return s.apiKeyRepo.LoadHashesByUser(ctx, idUser)
}

// PurgeCache drops cached auth sessions for the given key hashes. Best-effort: a
// missed delete only means the snapshot lives out its short TTL.
func (s *ApiKeyService) PurgeCache(ctx context.Context, hashes []string) {
	for _, hash := range hashes {
		_ = s.cache.Del(ctx, apiKeyCachePrefix+hash).Err()
	}
}

// LookupSession resolves a raw API key to an ApiKeySession.
// Returns nil, nil when the key is unknown or expired (caller must 401).
func (s *ApiKeyService) LookupSession(ctx context.Context, rawKey string) (*model.ApiKeySession, error) {
	hash := HashApiKey(rawKey)
	cacheKey := apiKeyCachePrefix + hash

	var session model.ApiKeySession
	if err := s.cache.Get(ctx, cacheKey).Scan(&session); err == nil {
		if session.User.IdUser == 0 {
			return nil, nil // negative cache hit
		}
		// Re-check expiry on every hit: guards the clock-skew window so an
		// expired key can never authenticate off a stale snapshot.
		if session.ExpiresAt != nil && !session.ExpiresAt.After(time.Now()) {
			_ = s.cache.Del(ctx, cacheKey).Err()
			return nil, nil
		}
		// Fire-and-forget last_used_at update
		select {
		case s.lastUsedCh <- hash:
		default:
		}
		return &session, nil
	}

	dbSession, err := s.apiKeyRepo.LoadByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("looking up api key: %w", err)
	}
	if dbSession == nil {
		// Store negative cache entry so repeated unknown-key requests don't hit DB
		_ = s.cache.Set(ctx, cacheKey, model.ApiKeySession{}, apiKeyNegativeCacheTTL).Err()
		return nil, nil
	}

	// Cap the cache TTL so the snapshot never outlives the key's expiry; clamp
	// defensively even though LoadByHash already filtered out expired keys.
	ttl := apiKeyCacheTTL
	if dbSession.ExpiresAt != nil {
		if remaining := time.Until(*dbSession.ExpiresAt); remaining < ttl {
			ttl = remaining
		}
	}
	if ttl > 0 {
		_ = s.cache.Set(ctx, cacheKey, *dbSession, ttl).Err()
	}
	select {
	case s.lastUsedCh <- hash:
	default:
	}
	return dbSession, nil
}

// CheckRateLimit returns ErrRateLimited if the key exceeded its per-minute quota.
// Fails open when Redis is unreachable — better to serve traffic than block all
// authenticated calls on a cache outage — but logs so the incident stays visible.
func (s *ApiKeyService) CheckRateLimit(ctx context.Context, idApiKey int64, limitPerMin int) error {
	windowKey := fmt.Sprintf("%s%d:%d", rateLimitKeyPrefix, idApiKey, time.Now().Unix()/60)
	count, err := s.cache.Incr(ctx, windowKey).Result()
	if err != nil {
		log.Warn().Err(err).Int64("idApiKey", idApiKey).Msg("rate limit check failed, failing open")
		return nil
	}
	if count == 1 {
		_ = s.cache.Expire(ctx, windowKey, 120*time.Second).Err()
	}
	if count > int64(limitPerMin) {
		return ErrRateLimited
	}
	return nil
}

func (s *ApiKeyService) drainLastUsed() {
	for {
		select {
		case <-s.stopCh:
			return
		case hash := <-s.lastUsedCh:
			if err := s.apiKeyRepo.UpdateLastUsedByHash(context.Background(), hash); err != nil {
				log.Warn().Err(err).Msg("failed to update last_used_at for api key")
			}
		}
	}
}
