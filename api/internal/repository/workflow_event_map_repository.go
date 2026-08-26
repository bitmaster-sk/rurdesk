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

type WorkflowEventMapRepository struct {
	pool *pgxpool.Pool
}

func NewWorkflowEventMapRepository(pool *pgxpool.Pool) *WorkflowEventMapRepository {
	return &WorkflowEventMapRepository{pool: pool}
}

func (r *WorkflowEventMapRepository) LoadMappings(ctx context.Context, idProject int64) ([]*model.WorkflowEventMapping, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_project, event, id_state
		FROM projects.workflow_event_state_map
		WHERE id_project = $1
		ORDER BY event
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying workflow event mappings: %w", err)
	}
	mappings, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WorkflowEventMapping])
	if err != nil {
		return nil, fmt.Errorf("collecting workflow event mappings: %w", err)
	}
	return mappings, nil
}

func (r *WorkflowEventMapRepository) LoadMapping(ctx context.Context, idProject int64, event string) (*model.WorkflowEventMapping, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_project, event, id_state
		FROM projects.workflow_event_state_map
		WHERE id_project = $1 AND event = $2
	`, idProject, event)
	if err != nil {
		return nil, fmt.Errorf("querying workflow event mapping: %w", err)
	}
	mapping, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WorkflowEventMapping])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting workflow event mapping: %w", err)
	}
	return mapping, nil
}

func (r *WorkflowEventMapRepository) ReplaceMappings(ctx context.Context, idProject int64, mappings []model.WorkflowEventMappingEntry) ([]*model.WorkflowEventMapping, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM projects.workflow_event_state_map WHERE id_project = $1`,
		idProject,
	); err != nil {
		return nil, fmt.Errorf("deleting workflow event mappings: %w", err)
	}

	for _, entry := range mappings {
		if entry.IdState == nil {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO projects.workflow_event_state_map (id_project, event, id_state) VALUES ($1, $2, $3)`,
			idProject, entry.Event, entry.IdState,
		); err != nil {
			return nil, fmt.Errorf("inserting workflow event mapping: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return r.LoadMappings(ctx, idProject)
}
