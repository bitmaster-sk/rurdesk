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

type PhaseStateMapRepository struct {
	pool *pgxpool.Pool
}

func NewPhaseStateMapRepository(pool *pgxpool.Pool) *PhaseStateMapRepository {
	return &PhaseStateMapRepository{pool: pool}
}

func (r *PhaseStateMapRepository) LoadMappings(ctx context.Context, idProject int64) ([]*model.PhaseStateMapping, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_project, phase, id_state
		FROM projects.agent_phase_state_map
		WHERE id_project = $1
		ORDER BY phase
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying phase state mappings: %w", err)
	}
	mappings, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.PhaseStateMapping])
	if err != nil {
		return nil, fmt.Errorf("collecting phase state mappings: %w", err)
	}
	return mappings, nil
}

func (r *PhaseStateMapRepository) LoadMapping(ctx context.Context, idProject int64, phase string) (*model.PhaseStateMapping, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_project, phase, id_state
		FROM projects.agent_phase_state_map
		WHERE id_project = $1 AND phase = $2
	`, idProject, phase)
	if err != nil {
		return nil, fmt.Errorf("querying phase state mapping: %w", err)
	}
	mapping, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.PhaseStateMapping])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting phase state mapping: %w", err)
	}
	return mapping, nil
}

func (r *PhaseStateMapRepository) ReplaceMappings(ctx context.Context, idProject int64, mappings []model.PhaseStateMappingEntry) ([]*model.PhaseStateMapping, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM projects.agent_phase_state_map WHERE id_project = $1`,
		idProject,
	); err != nil {
		return nil, fmt.Errorf("deleting phase state mappings: %w", err)
	}

	for _, entry := range mappings {
		if entry.IdState == nil {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO projects.agent_phase_state_map (id_project, phase, id_state) VALUES ($1, $2, $3)`,
			idProject, entry.Phase, entry.IdState,
		); err != nil {
			return nil, fmt.Errorf("inserting phase state mapping: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return r.LoadMappings(ctx, idProject)
}
