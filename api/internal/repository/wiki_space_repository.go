package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WikiSpaceRepository struct {
	pool *pgxpool.Pool
}

const wikiBudgetLockKey = 176_001

type WikiProjectBudget struct {
	IdProject int64  `db:"id_project"`
	IdSpace   *int64 `db:"id_space"`
	Limit     int    `db:"wiki_always_token_limit"`
}

func NewWikiSpaceRepository(pool *pgxpool.Pool) *WikiSpaceRepository {
	return &WikiSpaceRepository{pool: pool}
}

func (r *WikiSpaceRepository) LockBudget(ctx context.Context) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, wikiBudgetLockKey); err != nil {
		return fmt.Errorf("locking wiki budget: %w", err)
	}
	return nil
}

func (r *WikiSpaceRepository) LockSpace(ctx context.Context, idSpace int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `SELECT 1 FROM wiki.space WHERE id_space = $1 FOR NO KEY UPDATE`, idSpace); err != nil {
		return fmt.Errorf("locking wiki space: %w", err)
	}
	return nil
}

func (r *WikiSpaceRepository) LockAllSpaces(ctx context.Context) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `SELECT 1 FROM wiki.space ORDER BY id_space FOR NO KEY UPDATE`); err != nil {
		return fmt.Errorf("locking wiki spaces: %w", err)
	}
	return nil
}

func (r *WikiSpaceRepository) LoadInstanceSpace(ctx context.Context) (*model.WikiSpace, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT id_space, kind, id_project FROM wiki.space WHERE kind = 'instance'`)
	if err != nil {
		return nil, fmt.Errorf("querying instance wiki space: %w", err)
	}
	space, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WikiSpace])
	if err != nil {
		return nil, fmt.Errorf("collecting instance wiki space: %w", err)
	}
	return space, nil
}

func (r *WikiSpaceRepository) EnsureProjectSpace(ctx context.Context, idProject int64) (*model.WikiSpace, error) {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO wiki.space (kind, id_project) VALUES ('project', $1)
		ON CONFLICT (id_project) DO NOTHING
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("inserting project wiki space: %w", err)
	}
	rows, err := db.Query(ctx, `SELECT id_space, kind, id_project FROM wiki.space WHERE id_project = $1`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying project wiki space: %w", err)
	}
	space, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WikiSpace])
	if err != nil {
		return nil, fmt.Errorf("collecting project wiki space: %w", err)
	}
	return space, nil
}

func (r *WikiSpaceRepository) LoadSpace(ctx context.Context, idSpace int64) (*model.WikiSpace, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT id_space, kind, id_project FROM wiki.space WHERE id_space = $1`, idSpace)
	if err != nil {
		return nil, fmt.Errorf("querying wiki space: %w", err)
	}
	space, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WikiSpace])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting wiki space: %w", err)
	}
	return space, nil
}

func (r *WikiSpaceRepository) LoadProjectBudgets(ctx context.Context, idsProject []int64) ([]*WikiProjectBudget, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT p.id_project, s.id_space, p.wiki_always_token_limit
		FROM projects.project p
		LEFT JOIN wiki.space s ON s.id_project = p.id_project
		WHERE $1::bigint[] IS NULL OR p.id_project = ANY($1)
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying wiki budgets: %w", err)
	}
	budgets, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[WikiProjectBudget])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki budgets: %w", err)
	}
	return budgets, nil
}

func (r *WikiSpaceRepository) LoadHomePage(ctx context.Context, idSpace int64) (*int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT s.id_home_page FROM wiki.space s
		INNER JOIN wiki.page p ON p.id_page = s.id_home_page AND p.deleted_at IS NULL
		WHERE s.id_space = $1
	`, idSpace)
	if err != nil {
		return nil, fmt.Errorf("querying wiki home page: %w", err)
	}
	idPage, err := pgx.CollectOneRow(rows, pgx.RowTo[int64])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting wiki home page: %w", err)
	}
	return &idPage, nil
}

func (r *WikiSpaceRepository) UpdateHomePage(ctx context.Context, idSpace int64, idPage *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `UPDATE wiki.space SET id_home_page = $2 WHERE id_space = $1`, idSpace, idPage); err != nil {
		return fmt.Errorf("updating wiki home page: %w", err)
	}
	return nil
}

func (r *WikiSpaceRepository) UpdateTokenLimit(ctx context.Context, idProject int64, limit int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `UPDATE projects.project SET wiki_always_token_limit = $2 WHERE id_project = $1`, idProject, limit)
	if err != nil {
		return fmt.Errorf("updating wiki token limit: %w", err)
	}
	return nil
}
