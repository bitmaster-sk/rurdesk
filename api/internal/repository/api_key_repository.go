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
const defaultAgentRateLimit = 600

// ErrApiKeyNotFound is returned when the targeted API key does not exist.
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
	isAgent bool,
) (*model.ApiKey, error) {
	key := &model.ApiKey{}
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO users.api_key (id_user, name, key_hash, expires_at, rate_limit_override, is_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at`,
		idUser, name, keyHash, expiresAt, rateLimitOverride, isAgent,
	).Scan(
		&key.IdApiKey, &key.IdUser, &key.Name, &key.RateLimitOverride,
		&key.CreatedAt, &key.ExpiresAt, &key.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting api key: %w", err)
	}
	return key, nil
}

// LoadAgentApiKey returns the agent's single API key, or nil, nil when none
// exists. The partial unique index on id_user guarantees at most one row.
func (r *ApiKeyRepository) LoadAgentApiKey(ctx context.Context, idUser int64) (*model.ApiKey, error) {
	key := &model.ApiKey{}
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at
		FROM users.api_key
		WHERE id_user = $1 AND is_agent`,
		idUser,
	).Scan(
		&key.IdApiKey, &key.IdUser, &key.Name, &key.RateLimitOverride,
		&key.CreatedAt, &key.ExpiresAt, &key.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading agent api key: %w", err)
	}
	return key, nil
}

func (r *ApiKeyRepository) LoadUserApiKeys(ctx context.Context, idUser int64) ([]model.ApiKey, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at
		FROM users.api_key
		WHERE id_user = $1 AND NOT is_agent
		ORDER BY created_at DESC, id_api_key DESC`,
		idUser)
	if err != nil {
		return nil, fmt.Errorf("querying user api keys: %w", err)
	}
	defer rows.Close()

	keys := make([]model.ApiKey, 0)
	for rows.Next() {
		var key model.ApiKey
		if err := rows.Scan(
			&key.IdApiKey, &key.IdUser, &key.Name, &key.RateLimitOverride,
			&key.CreatedAt, &key.ExpiresAt, &key.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user api key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user api keys: %w", err)
	}
	return keys, nil
}

func (r *ApiKeyRepository) CountUserApiKeys(ctx context.Context, idUser int64) (int, error) {
	var count int
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx,
		`SELECT count(*) FROM users.api_key WHERE id_user = $1 AND NOT is_agent`, idUser,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting user api keys: %w", err)
	}
	return count, nil
}

// UpdateApiKeyHash rotates a key in place: swaps the stored hash, resets
// last_used_at, keeps id/name/created_at. Returns the previous key_hash so the
// caller can purge the old cached session. ErrApiKeyNotFound when no row matches.
//
// Ownership and is_agent are both part of the WHERE clause: dropping either would
// let a user rotate another user's key, or reach an agent key from a user route.
func (r *ApiKeyRepository) UpdateApiKeyHash(
	ctx context.Context,
	idUser, idApiKey int64,
	isAgent bool,
	keyHash string,
) (string, *model.ApiKey, error) {
	key := &model.ApiKey{}
	var oldHash string
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		WITH old AS (
			SELECT key_hash FROM users.api_key
			WHERE id_api_key = $1 AND id_user = $2 AND is_agent = $3
		)
		UPDATE users.api_key
		SET key_hash = $4, last_used_at = NULL
		WHERE id_api_key = $1 AND id_user = $2 AND is_agent = $3
		RETURNING id_api_key, id_user, name, rate_limit_override, created_at, expires_at, last_used_at,
		          (SELECT key_hash FROM old)`,
		idApiKey, idUser, isAgent, keyHash,
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

// DeleteApiKey removes a key and returns its key_hash for cache invalidation.
// ErrApiKeyNotFound when no row matches.
//
// Ownership and is_agent are both part of the WHERE clause: dropping either would
// let a user revoke another user's key, or reach an agent key from a user route.
func (r *ApiKeyRepository) DeleteApiKey(ctx context.Context, idUser, idApiKey int64, isAgent bool) (string, error) {
	var keyHash string
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx,
		`DELETE FROM users.api_key
		 WHERE id_api_key = $1 AND id_user = $2 AND is_agent = $3
		 RETURNING key_hash`,
		idApiKey, idUser, isAgent,
	).Scan(&keyHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrApiKeyNotFound
	}
	if err != nil {
		return "", fmt.Errorf("deleting api key: %w", err)
	}
	return keyHash, nil
}

// LoadByHash joins users.user so a single DB round-trip populates the auth session.
// Returns nil, nil when no active key matches (not found or expired).
//
// is_admin is deliberately not selected, so a key never carries admin rights.
// Selecting it would let a leaked admin token mint an agent and its key, defeating
// the rule that a token cannot create tokens.
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
		session.RateLimitPerMin = defaultAgentRateLimit
	} else {
		session.RateLimitPerMin = defaultHumanRateLimit
	}
	return &session, nil
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
