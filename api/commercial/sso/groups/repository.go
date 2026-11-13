package groups

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const adminWithPasswordLockKey int64 = 4 << 48

type mappingRepository struct {
	pool *pgxpool.Pool
}

func (r *mappingRepository) listMappings(ctx context.Context) ([]MappingRes, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT m.id_sso_group_mapping, m.group_value, m.id_team, t.name AS team_name
		FROM users.sso_group_mapping m
		JOIN users.team t ON t.id_team = m.id_team
		ORDER BY m.group_value, t.name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying sso group mappings: %w", err)
	}
	mappings, err := pgx.CollectRows(rows, pgx.RowToStructByName[MappingRes])
	if err != nil {
		return nil, fmt.Errorf("collecting sso group mappings: %w", err)
	}
	return mappings, nil
}

func (r *mappingRepository) markGroupsSent(ctx context.Context, issuer, subject string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE users.user_identity SET has_sent_groups = TRUE
		WHERE issuer = $1 AND subject = $2 AND NOT has_sent_groups
	`, issuer, subject)
	if err != nil {
		return fmt.Errorf("marking sso groups as sent: %w", err)
	}
	return nil
}

func (r *mappingRepository) hasSentGroups(ctx context.Context, issuer, subject string) (bool, error) {
	var hasSent bool
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT has_sent_groups FROM users.user_identity WHERE issuer = $1 AND subject = $2
	`, issuer, subject).Scan(&hasSent)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking sso groups history: %w", err)
	}
	return hasSent, nil
}

func (r *mappingRepository) loadMappings(ctx context.Context) ([]Mapping, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `SELECT group_value, id_team FROM users.sso_group_mapping`)
	if err != nil {
		return nil, fmt.Errorf("querying sso group mappings: %w", err)
	}
	mappings, err := pgx.CollectRows(rows, pgx.RowToStructByName[Mapping])
	if err != nil {
		return nil, fmt.Errorf("collecting sso group mappings: %w", err)
	}
	return mappings, nil
}

func (r *mappingRepository) insertMapping(ctx context.Context, groupValue string, idTeam int64) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO users.sso_group_mapping (group_value, id_team) VALUES ($1, $2)
	`, groupValue, idTeam)
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return ErrMappingExists
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		return ErrTeamNotFound
	case err != nil:
		return fmt.Errorf("inserting sso group mapping: %w", err)
	}
	return nil
}

func (r *mappingRepository) deleteMapping(ctx context.Context, idMapping int64) error {
	tag, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `DELETE FROM users.sso_group_mapping WHERE id_sso_group_mapping = $1`, idMapping)
	if err != nil {
		return fmt.Errorf("deleting sso group mapping: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMappingNotFound
	}
	return nil
}

func (r *mappingRepository) loadMappedMemberships(ctx context.Context, idUser int64) ([]int64, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT ut.id_team
		FROM users.user_team ut
		WHERE ut.id_user = $1
		  AND EXISTS (SELECT 1 FROM users.sso_group_mapping m WHERE m.id_team = ut.id_team)
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying mapped memberships: %w", err)
	}
	idsTeam, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting mapped memberships: %w", err)
	}
	return idsTeam, nil
}

func (r *mappingRepository) countAdminsWithPasswordLocked(ctx context.Context) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, adminWithPasswordLockKey); err != nil {
		return 0, fmt.Errorf("locking admin count: %w", err)
	}
	var count int64
	err := db.QueryRow(ctx, `
		SELECT count(*) FROM users.user WHERE is_admin AND is_active AND password IS NOT NULL
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting admins with a password: %w", err)
	}
	return count, nil
}
