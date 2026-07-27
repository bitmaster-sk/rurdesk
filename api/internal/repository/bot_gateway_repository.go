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

var ErrBotGatewayNotFound = errors.New("bot gateway not found")

type BotGatewayRepository struct {
	pool *pgxpool.Pool
}

func NewBotGatewayRepository(pool *pgxpool.Pool) *BotGatewayRepository {
	return &BotGatewayRepository{pool: pool}
}

const botGatewayColumns = `id_bot_gateway, id_user_bot, gateway_url, max_concurrent, webhook_secret, config_json, created_at`

func scanBotGateway(row pgx.Row) (*model.BotGateway, error) {
	gw := &model.BotGateway{}
	err := row.Scan(
		&gw.IdBotGateway, &gw.IdUserBot, &gw.GatewayUrl,
		&gw.MaxConcurrent, &gw.WebhookSecret, &gw.ConfigJson, &gw.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning bot gateway: %w", err)
	}
	return gw, nil
}

func (r *BotGatewayRepository) Insert(ctx context.Context, idUserBot int64, req model.CreateBotGatewayReq, webhookSecret []byte) (*model.BotGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		INSERT INTO agent.bot_gateway (id_user_bot, gateway_url, webhook_secret)
		VALUES ($1, $2, $3)
		RETURNING `+botGatewayColumns,
		idUserBot, req.GatewayUrl, webhookSecret,
	)
	return scanBotGateway(row)
}

func (r *BotGatewayRepository) LoadByBotUser(ctx context.Context, idUserBot int64) (*model.BotGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		SELECT `+botGatewayColumns+` FROM agent.bot_gateway
		WHERE id_user_bot = $1`,
		idUserBot,
	)
	gw, err := scanBotGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return gw, err
}

func (r *BotGatewayRepository) DeleteByBotUser(ctx context.Context, idUserBot int64) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		DELETE FROM agent.bot_gateway
		WHERE id_user_bot = $1`,
		idUserBot,
	)
	if err != nil {
		return fmt.Errorf("deleting bot gateway: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBotGatewayNotFound
	}
	return nil
}

// UpdateUrl changes only the gateway URL (admin edit); the webhook secret is
// untouched, so no token is reminted.
func (r *BotGatewayRepository) UpdateUrl(ctx context.Context, idUserBot int64, gatewayUrl string) (*model.BotGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		UPDATE agent.bot_gateway
		SET gateway_url = $2
		WHERE id_user_bot = $1
		RETURNING `+botGatewayColumns,
		idUserBot, gatewayUrl,
	)
	gw, err := scanBotGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBotGatewayNotFound
	}
	return gw, err
}

func (r *BotGatewayRepository) UpdateSecret(ctx context.Context, idUserBot int64, webhookSecret []byte) (*model.BotGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		UPDATE agent.bot_gateway
		SET webhook_secret = $2
		WHERE id_user_bot = $1
		RETURNING `+botGatewayColumns,
		idUserBot, webhookSecret,
	)
	gw, err := scanBotGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBotGatewayNotFound
	}
	return gw, err
}
