package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookDedupRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookDedupRepository(pool *pgxpool.Pool) *WebhookDedupRepository {
	return &WebhookDedupRepository{pool: pool}
}

func (r *WebhookDedupRepository) Exists(ctx context.Context, uuidEvent uuid.UUID) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent.webhook_event_dedup WHERE uuid_event = $1)`,
		uuidEvent,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking webhook event dedup: %w", err)
	}
	return exists, nil
}

func (r *WebhookDedupRepository) Insert(ctx context.Context, uuidEvent uuid.UUID, idRun int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`INSERT INTO agent.webhook_event_dedup (uuid_event, id_run) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		uuidEvent, idRun,
	)
	if err != nil {
		return fmt.Errorf("inserting webhook event dedup: %w", err)
	}
	return nil
}

func (r *WebhookDedupRepository) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	cutoff := time.Now().Add(-olderThan)
	tag, err := db.Exec(ctx,
		`DELETE FROM agent.webhook_event_dedup WHERE created_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("cleaning up webhook event dedup: %w", err)
	}
	return tag.RowsAffected(), nil
}
