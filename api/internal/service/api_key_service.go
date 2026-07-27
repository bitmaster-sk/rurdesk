package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

const (
	apiKeyCacheTTL         = 5 * time.Minute
	apiKeyNegativeCacheTTL = 60 * time.Second
	apiKeyCachePrefix      = "auth:apikey:"
	rateLimitKeyPrefix     = "ratelimit:apikey:"
	lastUsedChannelSize    = 1000
)

var ErrRateLimited = fmt.Errorf("rate limit exceeded")

type ApiKeyService struct {
	apiKeyRepo *repository.ApiKeyRepository
	cache      *redis.Client
	lastUsedCh chan string
	stopCh     chan struct{}
}

func NewApiKeyService(apiKeyRepo *repository.ApiKeyRepository, cache *redis.Client) *ApiKeyService {
	svc := &ApiKeyService{
		apiKeyRepo: apiKeyRepo,
		cache:      cache,
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

func (s *ApiKeyService) Create(ctx context.Context, idUser int64, req *model.CreateApiKeyReq) (*model.CreateApiKeyRes, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}
	keyHash := HashApiKey(rawKey)
	key, err := s.apiKeyRepo.Insert(ctx, idUser, req.Name, keyHash, req.ExpiresAt, req.RateLimitOverride)
	if err != nil {
		return nil, fmt.Errorf("inserting api key: %w", err)
	}
	return &model.CreateApiKeyRes{ApiKey: *key, RawKey: rawKey}, nil
}

// GetByUser returns a bot's single API key, or nil when none exists.
func (s *ApiKeyService) GetByUser(ctx context.Context, idUser int64) (*model.ApiKey, error) {
	return s.apiKeyRepo.LoadOneByUser(ctx, idUser)
}

// Regenerate rotates a bot's key in place, invalidating the old one immediately,
// and returns the new one-time raw key. ErrApiKeyNotFound if the bot has none yet.
func (s *ApiKeyService) Regenerate(ctx context.Context, idUser int64) (*model.CreateApiKeyRes, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("generating api key: %w", err)
	}
	oldHash, key, err := s.apiKeyRepo.UpdateHashByUser(ctx, idUser, HashApiKey(rawKey))
	if err != nil {
		return nil, err
	}
	if oldHash != "" {
		_ = s.cache.Del(ctx, apiKeyCachePrefix+oldHash).Err()
	}
	return &model.CreateApiKeyRes{ApiKey: *key, RawKey: rawKey}, nil
}

// RevokeByUser deletes a bot's single key and purges its cached session.
func (s *ApiKeyService) RevokeByUser(ctx context.Context, idUser int64) error {
	keyHash, err := s.apiKeyRepo.DeleteByUser(ctx, idUser)
	if err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}
	if keyHash != "" {
		_ = s.cache.Del(ctx, apiKeyCachePrefix+keyHash).Err()
	}
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
