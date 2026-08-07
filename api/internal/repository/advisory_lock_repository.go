package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdvisoryLockRepository struct {
	pool *pgxpool.Pool
}

func NewAdvisoryLockRepository(pool *pgxpool.Pool) *AdvisoryLockRepository {
	return &AdvisoryLockRepository{pool: pool}
}

func (r *AdvisoryLockRepository) Lock(ctx context.Context, key int64) error {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, key); err != nil {
		return fmt.Errorf("acquiring advisory lock %d: %w", key, err)
	}
	return nil
}
