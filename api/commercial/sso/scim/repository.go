package scim

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	accountColumns = `id_user, name, email, is_active, scim_external_id`
	visible        = `NOT is_agent AND NOT is_break_glass AND scim_deleted_at IS NULL`
)

type candidate struct {
	IdUser       int64   `db:"id_user"`
	IsAgent      bool    `db:"is_agent"`
	IsBreakGlass bool    `db:"is_break_glass"`
	ExternalId   *string `db:"scim_external_id"`
}

type storedToken struct {
	Hash      *string `db:"token_hash"`
	IsEnabled bool    `db:"is_scim_enabled"`
}

type TokenInfo struct {
	CreateAt   time.Time  `db:"create_at"`
	LastUsedAt *time.Time `db:"last_used_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) loadVisible(ctx context.Context, idUser int64) (*Account, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+accountColumns+` FROM users.user WHERE id_user = $1 AND `+visible+`
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying scim user: %w", err)
	}
	account, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting scim user: %w", err)
	}
	return account, nil
}

func (r *Repository) list(ctx context.Context, filter *Filter, page Page) ([]Account, int64, error) {
	condition, args := visible, []any{}
	if filter != nil {
		switch filter.Attribute {
		case AttributeUserName:
			condition += ` AND lower(email) = lower($1)`
		case AttributeExternalId:
			condition += ` AND scim_external_id = $1`
		}
		args = append(args, filter.Value)
	}
	db := extctx.GetDb(ctx, r.pool)
	var total int64
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users.user WHERE `+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting scim users: %w", err)
	}
	limit := fmt.Sprintf(" ORDER BY id_user LIMIT %d OFFSET %d", page.Count, page.StartIndex-1)
	rows, err := db.Query(ctx, `SELECT `+accountColumns+` FROM users.user WHERE `+condition+limit, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying scim users: %w", err)
	}
	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByName[Account])
	if err != nil {
		return nil, 0, fmt.Errorf("collecting scim users: %w", err)
	}
	return accounts, total, nil
}

func (r *Repository) findByEmail(ctx context.Context, address string) ([]candidate, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_user, is_agent, is_break_glass, scim_external_id FROM users.user WHERE lower(email) = lower($1) FOR UPDATE
	`, address)
	if err != nil {
		return nil, fmt.Errorf("querying users by e-mail: %w", err)
	}
	candidates, err := pgx.CollectRows(rows, pgx.RowToStructByName[candidate])
	if err != nil {
		return nil, fmt.Errorf("collecting users by e-mail: %w", err)
	}
	return candidates, nil
}

func (r *Repository) externalIdTakenByOther(ctx context.Context, externalId string, idUser int64) (bool, error) {
	var isTaken bool
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users.user WHERE scim_external_id = $1 AND id_user <> $2)
	`, externalId, idUser).Scan(&isTaken)
	if err != nil {
		return false, fmt.Errorf("checking scim external id: %w", err)
	}
	return isTaken, nil
}

func (r *Repository) setExternalId(ctx context.Context, idUser int64, externalId *string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.user SET scim_external_id = $1 WHERE id_user = $2`, externalId, idUser)
	if err != nil {
		return fmt.Errorf("saving scim external id: %w", err)
	}
	return nil
}

func (r *Repository) setDeleted(ctx context.Context, idUser int64, isDeleted bool) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.user SET scim_deleted_at = CASE WHEN $1 THEN now() END WHERE id_user = $2
	`, isDeleted, idUser)
	if err != nil {
		return fmt.Errorf("saving scim deletion: %w", err)
	}
	return nil
}

func (r *Repository) loadToken(ctx context.Context) (storedToken, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT t.token_hash, s.is_scim_enabled
		FROM users.sso_settings s
		LEFT JOIN users.scim_token t ON TRUE
	`)
	if err != nil {
		return storedToken{}, fmt.Errorf("querying scim token: %w", err)
	}
	token, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[storedToken])
	if err != nil {
		return storedToken{}, fmt.Errorf("collecting scim token: %w", err)
	}
	return token, nil
}

func (r *Repository) IsProvisioning(ctx context.Context) (bool, error) {
	stored, err := r.loadToken(ctx)
	if err != nil {
		return false, err
	}
	return stored.IsEnabled && stored.Hash != nil, nil
}

func (r *Repository) loadTokenInfo(ctx context.Context) (*TokenInfo, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `SELECT create_at, last_used_at FROM users.scim_token`)
	if err != nil {
		return nil, fmt.Errorf("querying scim token info: %w", err)
	}
	info, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[TokenInfo])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting scim token info: %w", err)
	}
	return info, nil
}

func (r *Repository) saveToken(ctx context.Context, hash string, idUser int64) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO users.scim_token (token_hash, create_by) VALUES ($1, $2)
		ON CONFLICT (id_scim_token) DO UPDATE SET
			token_hash = EXCLUDED.token_hash,
			create_at = now(),
			create_by = EXCLUDED.create_by,
			last_used_at = NULL
	`, hash, idUser)
	if err != nil {
		return fmt.Errorf("saving scim token: %w", err)
	}
	return nil
}

func (r *Repository) deleteToken(ctx context.Context) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `DELETE FROM users.scim_token`); err != nil {
		return fmt.Errorf("deleting scim token: %w", err)
	}
	return nil
}

func (r *Repository) touchToken(ctx context.Context) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.scim_token SET last_used_at = now()
		WHERE last_used_at IS NULL OR last_used_at < now() - interval '1 minute'
	`)
	if err != nil {
		return fmt.Errorf("touching scim token: %w", err)
	}
	return nil
}

func (r *Repository) setEnabled(ctx context.Context, isEnabled bool) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.sso_settings SET is_scim_enabled = $1`, isEnabled); err != nil {
		return fmt.Errorf("saving scim flag: %w", err)
	}
	return nil
}
