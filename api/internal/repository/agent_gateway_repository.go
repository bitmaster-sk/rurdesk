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

const agentGatewayColumns = `id_gateway, id_user_agent, gateway_url, webhook_secret,
	models, model_default, models_reported_at, max_turns_limit, max_turns_plan, max_turns_implement, created_at`

func scanAgentGateway(row pgx.Row) (*model.AgentGateway, error) {
	gw := &model.AgentGateway{}
	err := row.Scan(
		&gw.IdGateway, &gw.IdUserAgent, &gw.GatewayUrl, &gw.WebhookSecret,
		&gw.Models, &gw.ModelDefault, &gw.ModelsReportedAt, &gw.MaxTurnsLimit, &gw.MaxTurnsPlan, &gw.MaxTurnsImplement, &gw.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning agent gateway: %w", err)
	}
	return gw, nil
}

func (r *AgentGatewayRepository) Insert(ctx context.Context, idUserAgent int64, req model.CreateAgentGatewayReq, webhookSecret []byte) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		INSERT INTO agent.gateway (id_user_agent, gateway_url, webhook_secret, max_turns_plan, max_turns_implement)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+agentGatewayColumns,
		idUserAgent, req.GatewayUrl, webhookSecret, req.MaxTurnsPlan, req.MaxTurnsImplement,
	)
	return scanAgentGateway(row)
}

func (r *AgentGatewayRepository) LoadByAgentUser(ctx context.Context, idUserAgent int64) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		SELECT `+agentGatewayColumns+` FROM agent.gateway
		WHERE id_user_agent = $1`,
		idUserAgent,
	)
	gw, err := scanAgentGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return gw, err
}

func (r *AgentGatewayRepository) DeleteByAgentUser(ctx context.Context, idUserAgent int64) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		DELETE FROM agent.gateway
		WHERE id_user_agent = $1`,
		idUserAgent,
	)
	if err != nil {
		return fmt.Errorf("deleting agent gateway: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAgentGatewayNotFound
	}
	return nil
}

// Update applies the admin edit; the webhook secret is untouched, so no token
// is reminted.
func (r *AgentGatewayRepository) Update(ctx context.Context, idUserAgent int64, req model.CreateAgentGatewayReq) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		UPDATE agent.gateway
		SET gateway_url = $2, max_turns_plan = $3, max_turns_implement = $4
		WHERE id_user_agent = $1
		RETURNING `+agentGatewayColumns,
		idUserAgent, req.GatewayUrl, req.MaxTurnsPlan, req.MaxTurnsImplement,
	)
	gw, err := scanAgentGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentGatewayNotFound
	}
	return gw, err
}

func (r *AgentGatewayRepository) UpdateSecret(ctx context.Context, idUserAgent int64, webhookSecret []byte) (*model.AgentGateway, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, `
		UPDATE agent.gateway
		SET webhook_secret = $2
		WHERE id_user_agent = $1
		RETURNING `+agentGatewayColumns,
		idUserAgent, webhookSecret,
	)
	gw, err := scanAgentGateway(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAgentGatewayNotFound
	}
	return gw, err
}

func (r *AgentGatewayRepository) SetModels(ctx context.Context, idUserAgent int64, report model.GatewayRecoveredReq) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE agent.gateway
		SET models = $2, model_default = NULLIF($3, ''), max_turns_limit = NULLIF($4, 0), models_reported_at = now()
		WHERE id_user_agent = $1`,
		idUserAgent, report.Models, report.ModelDefault, report.MaxTurnsLimit,
	)
	if err != nil {
		return fmt.Errorf("setting agent gateway models: %w", err)
	}
	return nil
}

func (r *AgentGatewayRepository) LoadByAgentUsers(ctx context.Context, idsUserAgent []int64) (map[int64]*model.AgentGateway, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+agentGatewayColumns+` FROM agent.gateway
		WHERE id_user_agent = ANY($1)`,
		idsUserAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("querying agent gateways: %w", err)
	}
	defer rows.Close()

	byAgent := make(map[int64]*model.AgentGateway, len(idsUserAgent))
	for rows.Next() {
		gw, err := scanAgentGateway(rows)
		if err != nil {
			return nil, err
		}
		byAgent[gw.IdUserAgent] = gw
	}
	return byAgent, rows.Err()
}
