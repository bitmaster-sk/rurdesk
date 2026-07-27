package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewAppSettingsRepository(pool *pgxpool.Pool) *AppSettingsRepository {
	return &AppSettingsRepository{pool: pool}
}

// LoadAll returns every settings row as key -> raw JSON string.
func (r *AppSettingsRepository) LoadAll(ctx context.Context) (map[string]string, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT key, value::text FROM projects.app_settings`)
	if err != nil {
		return nil, fmt.Errorf("querying app settings: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scanning app setting: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating app settings: %w", err)
	}
	return out, nil
}

// Upsert writes each key/value (value is a raw JSON string) — one statement per key.
func (r *AppSettingsRepository) Upsert(ctx context.Context, values map[string]string) error {
	db := extctx.GetDb(ctx, r.pool)
	for k, v := range values {
		_, err := db.Exec(ctx, `
			INSERT INTO projects.app_settings (key, value, update_at)
			VALUES ($1, $2::jsonb, now())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, update_at = now()
		`, k, v)
		if err != nil {
			return fmt.Errorf("upserting app setting %q: %w", k, err)
		}
	}
	return nil
}
