package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AiModelRepository struct {
	pool *pgxpool.Pool
}

func NewAiModelRepository(pool *pgxpool.Pool) *AiModelRepository {
	return &AiModelRepository{pool: pool}
}

func (r *AiModelRepository) LoadNamesWithoutThinking(ctx context.Context, names []string) ([]string, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT name FROM agent.ai_model
		WHERE name = ANY($1) AND NOT is_thinking`,
		names,
	)
	if err != nil {
		return nil, fmt.Errorf("querying ai models without thinking: %w", err)
	}
	defer rows.Close()

	found := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning model without thinking: %w", err)
		}
		found = append(found, name)
	}
	return found, rows.Err()
}
