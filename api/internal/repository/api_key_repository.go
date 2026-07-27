package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultHumanRateLimit = 120
const defaultBotRateLimit = 600

// ErrApiKeyNotFound is returned when a bot has no API key to regenerate or revoke.
var ErrApiKeyNotFound = errors.New("api key not found")

type ApiKeyRepository struct {
	pool *pgxpool.Pool
}

func NewApiKeyRepository(pool *pgxpool.Pool) *ApiKeyRepository {
	return &ApiKeyRepository{pool: pool}
}

func (r *ApiKeyRepository) Insert(
	ctx context.Context,
	idUser int64,
	name, keyHash string,
	expiresAt *time.Time,
	rateLimitOverride *int,
) (*model.ApiKey, error) {
	key := &model.ApiKey{}
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO users.api_key (id_user, name, key_hash, expires_at, rate_limit_override)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at`,
		idUser, name, keyHash, expiresAt, rateLimitOverride,
	).Scan(
		&key.IdApiKey, &key.IdUser, &key.Name, &key.RateLimitOverride,
		&key.CreatedAt, &key.ExpiresAt, &key.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting api key: %w", err)
	}
	return key, nil
}

// LoadOneByUser returns the user's single API key, or nil, nil when none exists.
// The unique index on id_user guarantees at most one row.
func (r *ApiKeyRepository) LoadOneByUser(ctx context.Context, idUser int64) (*model.ApiKey, error) {
	key := &model.ApiKey{}
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at
		FROM users.api_key
		WHERE id_user = $1`,
		idUser,
	).Scan(
		&key.IdApiKey, &key.IdUser, &key.Name, &key.RateLimitOverride,
		&key.CreatedAt, &key.ExpiresAt, &key.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading api key by user: %w", err)
	}
	return key, nil
}

// UpdateHashByUser rotates a bot's key in place: swaps the stored hash,
// resets last_used_at, keeps id/name/created_at. Returns the previous
// key_hash so the caller can purge the old cached session. ErrApiKeyNotFound
// if the bot has none.
func (r *ApiKeyRepository) UpdateHashByUser(ctx context.Context, idUser int64, keyHash string) (string, *model.ApiKey, error) {
	key := &model.ApiKey{}
	var oldHash string
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		WITH old AS (SELECT key_hash FROM users.api_key WHERE id_user = $1)
		UPDATE users.api_key
		SET key_hash = $2, last_used_at = NULL
		WHERE id_user = $1
		RETURNING id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at,
		          (SELECT key_hash FROM old)`,
		idUser, keyHash,
	).Scan(
		&key.IdApiKey, &key.IdUser, &key.Name, &key.RateLimitOverride,
		&key.CreatedAt, &key.ExpiresAt, &key.LastUsedAt, &oldHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrApiKeyNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("updating api key hash: %w", err)
	}
	return oldHash, key, nil
}

// LoadByHash joins users.user so a single DB round-trip populates the auth session.
// Returns nil, nil when no active key matches (not found or expired).
func (r *ApiKeyRepository) LoadByHash(ctx context.Context, hash string) (*model.ApiKeySession, error) {
	var session model.ApiKeySession
	var rateLimitOverride *int
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT k.id_api_key, k.rate_limit_override, k.expires_at,
		       u.id_user, u.name, u.email, u.color_avatar_bg, u.is_bot
		FROM users.api_key k
		JOIN users.user u ON u.id_user = k.id_user
		WHERE k.key_hash = $1
		  AND (k.expires_at IS NULL OR k.expires_at > (NOW() AT TIME ZONE 'utc'))`,
		hash,
	).Scan(
		&session.IdApiKey, &rateLimitOverride, &session.ExpiresAt,
		&session.User.IdUser, &session.User.Name, &session.User.Email,
		&session.User.ColorAvatarBg, &session.User.IsBot,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("loading api key by hash: %w", err)
	}
	if rateLimitOverride != nil {
		session.RateLimitPerMin = *rateLimitOverride
	} else if session.User.IsBot {
		session.RateLimitPerMin = defaultBotRateLimit
	} else {
		session.RateLimitPerMin = defaultHumanRateLimit
	}
	return &session, nil
}

// DeleteByUser removes a bot's single key and returns its key_hash for cache
// invalidation. Returns ("", nil) when the bot has no key.
func (r *ApiKeyRepository) DeleteByUser(ctx context.Context, idUser int64) (string, error) {
	var keyHash string
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx,
		`DELETE FROM users.api_key WHERE id_user = $1 RETURNING key_hash`,
		idUser,
	).Scan(&keyHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("deleting api key by user: %w", err)
	}
	return keyHash, nil
}

func (r *ApiKeyRepository) UpdateLastUsedByHash(ctx context.Context, hash string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx,
		`UPDATE users.api_key SET last_used_at = (NOW() AT TIME ZONE 'utc') WHERE key_hash = $1`, hash)
	if err != nil {
		return fmt.Errorf("updating api key last used: %w", err)
	}
	return nil
}

// LoadHashesByUser returns a user's stored key hashes, to purge the Redis
// auth cache on user deletion (DB rows cascade, but a cached session would
// otherwise stay valid up to its 5-minute TTL).
func (r *ApiKeyRepository) LoadHashesByUser(ctx context.Context, idUser int64) ([]string, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx,
		`SELECT key_hash FROM users.api_key WHERE id_user = $1`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying api key hashes: %w", err)
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("scanning api key hash: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating api key hashes: %w", err)
	}
	return hashes, nil
}
