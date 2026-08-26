package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectSkillRepository struct {
	pool *pgxpool.Pool
}

func NewProjectSkillRepository(pool *pgxpool.Pool) *ProjectSkillRepository {
	return &ProjectSkillRepository{pool: pool}
}

func (r *ProjectSkillRepository) Load(ctx context.Context, idProject int64) ([]*model.ProjectSkill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_project, id_skill, stage
		FROM agent.project_skill
		WHERE id_project = $1
		ORDER BY id_skill, stage
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying project skills: %w", err)
	}
	all, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.ProjectSkill])
	if err != nil {
		return nil, fmt.Errorf("collecting project skills: %w", err)
	}
	return all, nil
}

func (r *ProjectSkillRepository) Enable(ctx context.Context, idSkill int64, stages []string) error {
	for _, stage := range stages {
		if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
			INSERT INTO agent.project_skill (id_project, id_skill, stage)
			SELECT p.id_project, $1, $2 FROM projects.project p
			ON CONFLICT DO NOTHING
		`, idSkill, stage); err != nil {
			return fmt.Errorf("enabling skill %d for stage %q: %w", idSkill, stage, err)
		}
	}
	return nil
}

func (r *ProjectSkillRepository) EnableForProject(ctx context.Context, idProject int64, builtinKey, stage string) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO agent.project_skill (id_project, id_skill, stage)
		SELECT $1, s.id_skill, $3 FROM agent.skill s WHERE s.builtin_key = $2
		ON CONFLICT DO NOTHING
	`, idProject, builtinKey, stage); err != nil {
		return fmt.Errorf("enabling builtin skill %q for stage %q: %w", builtinKey, stage, err)
	}
	return nil
}

func (r *ProjectSkillRepository) Replace(ctx context.Context, idProject int64, entries []model.UpdateProjectSkillReq) ([]*model.ProjectSkill, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM agent.project_skill WHERE id_project = $1`, idProject,
	); err != nil {
		return nil, fmt.Errorf("deleting project skills: %w", err)
	}

	for _, entry := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent.project_skill (id_project, id_skill, stage)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING
		`, idProject, entry.IdSkill, entry.Stage); err != nil {
			return nil, fmt.Errorf("inserting project skill: %w", err)
		}
	}

	// Read inside the transaction: after the commit this would run on a different
	// pooled connection and could observe a concurrent replace.
	rows, err := tx.Query(ctx, `
		SELECT id_project, id_skill, stage
		FROM agent.project_skill
		WHERE id_project = $1
		ORDER BY id_skill, stage
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying replaced project skills: %w", err)
	}
	replaced, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.ProjectSkill])
	if err != nil {
		return nil, fmt.Errorf("collecting replaced project skills: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}
	return replaced, nil
}
