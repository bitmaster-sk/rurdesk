package links

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type linkRepository struct {
	pool *pgxpool.Pool
}

func (r *linkRepository) list(ctx context.Context, idUser int64) ([]IdentityRes, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_user_identity, issuer, subject, create_at
		FROM users.user_identity
		WHERE id_user = $1
		ORDER BY create_at
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying sso identities: %w", err)
	}
	identities, err := pgx.CollectRows(rows, pgx.RowToStructByName[IdentityRes])
	if err != nil {
		return nil, fmt.Errorf("collecting sso identities: %w", err)
	}
	return identities, nil
}

func (r *linkRepository) deleteAll(ctx context.Context, idUser int64) (int64, error) {
	tag, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `DELETE FROM users.user_identity WHERE id_user = $1`, idUser)
	if err != nil {
		return 0, fmt.Errorf("deleting sso identities: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *linkRepository) issuersForUpdate(ctx context.Context, idUser int64) ([]string, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT issuer FROM users.user_identity WHERE id_user = $1 FOR UPDATE
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("locking sso identities: %w", err)
	}
	issuers, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("collecting sso issuers: %w", err)
	}
	return issuers, nil
}

func (r *linkRepository) hasAnyIssuer(ctx context.Context, idUser int64, issuers []string) (bool, error) {
	var hasAny bool
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users.user_identity WHERE id_user = $1 AND issuer = ANY($2))
	`, idUser, issuers).Scan(&hasAny)
	if err != nil {
		return false, fmt.Errorf("checking sso issuers of the target: %w", err)
	}
	return hasAny, nil
}

func (r *linkRepository) moveAll(ctx context.Context, idUser, idTarget int64) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `UPDATE users.user_identity SET id_user = $2 WHERE id_user = $1`, idUser, idTarget)
	if err != nil {
		return fmt.Errorf("moving sso identities: %w", err)
	}
	return nil
}
