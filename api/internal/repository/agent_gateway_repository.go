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

type AgentGatewayRepository struct {
	pool *pgxpool.Pool
}

func NewAgentGatewayRepository(pool *pgxpool.Pool) *AgentGatewayRepository {
	return &AgentGatewayRepository{pool: pool}
}

const agentGatewayColumns = `id_bot_gateway, id_user_bot, gateway_url, max_concurrent, webhook_secret, config_json, created_at`

func scanAgentGateway(row pgx.Row) (*model.AgentGateway, error) {
	gw := &model.AgentGateway{}
	err := row.Scan(
		&gw.IdBotGateway, &gw.IdUserBot, &gw.GatewayUrl,
		&gw.MaxConcurrent, &gw.WebhookSecret, &gw.ConfigJson, &gw.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning agent gateway: %w", err)
	}
	return gw, nil
}

func (r *AgentGatewayRepository) Insert(ctx context.Context, idUserBot int64, req model.CreateAgentGatewayReq, webhookSecret []byte) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		INSERT INTO agent.bot_gateway (id_user_bot, gateway_url, webhook_secret)
		VALUES ($1, $2, $3)
		RETURNING `+agentGatewayColumns,
		idUserBot, req.GatewayUrl, webhookSecret,
	)
	return scanAgentGateway(row)
}

func (r *AgentGatewayRepository) LoadByAgentUser(ctx context.Context, idUserBot int64) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		SELECT `+agentGatewayColumns+` FROM agent.bot_gateway
		WHERE id_user_bot = $1`,
		idUserBot,
	)
	gw, err := scanAgentGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return gw, err
}

func (r *AgentGatewayRepository) DeleteByAgentUser(ctx context.Context, idUserBot int64) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		DELETE FROM agent.bot_gateway
		WHERE id_user_bot = $1`,
		idUserBot,
	)
	if err != nil {
		return fmt.Errorf("deleting agent gateway: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAgentGatewayNotFound
	}
	return nil
}

// UpdateUrl changes only the gateway URL (admin edit); the webhook secret is
// untouched, so no token is reminted.
func (r *AgentGatewayRepository) UpdateUrl(ctx context.Context, idUserBot int64, gatewayUrl string) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		UPDATE agent.bot_gateway
		SET gateway_url = $2
		WHERE id_user_bot = $1
		RETURNING `+agentGatewayColumns,
		idUserBot, gatewayUrl,
	)
	gw, err := scanAgentGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentGatewayNotFound
	}
	return gw, err
}

func (r *AgentGatewayRepository) UpdateSecret(ctx context.Context, idUserBot int64, webhookSecret []byte) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		UPDATE agent.bot_gateway
		SET webhook_secret = $2
		WHERE id_user_bot = $1
		RETURNING `+agentGatewayColumns,
		idUserBot, webhookSecret,
	)
	gw, err := scanAgentGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentGatewayNotFound
	}
	return gw, err
}
