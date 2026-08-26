package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const skillColumns = `id_skill, name, description, content, builtin_key, builtin_checksum, created_at, updated_at`

type SkillRepository struct {
	pool *pgxpool.Pool
}

func NewSkillRepository(pool *pgxpool.Pool) *SkillRepository {
	return &SkillRepository{pool: pool}
}

func (r *SkillRepository) Load(ctx context.Context) ([]*model.Skill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+skillColumns+`
		FROM agent.skill
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying skills: %w", err)
	}
	all, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if err != nil {
		return nil, fmt.Errorf("collecting skills: %w", err)
	}
	return all, nil
}

func (r *SkillRepository) LoadById(ctx context.Context, idSkill int64) (*model.Skill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+skillColumns+`
		FROM agent.skill
		WHERE id_skill = $1
	`, idSkill)
	if err != nil {
		return nil, fmt.Errorf("querying skill: %w", err)
	}
	skill, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrSkillNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting skill: %w", err)
	}
	return skill, nil
}

func (r *SkillRepository) LoadByBuiltinKey(ctx context.Context, builtinKey string) (*model.Skill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+skillColumns+`
		FROM agent.skill
		WHERE builtin_key = $1
	`, builtinKey)
	if err != nil {
		return nil, fmt.Errorf("querying builtin skill %q: %w", builtinKey, err)
	}
	skill, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrSkillNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting builtin skill %q: %w", builtinKey, err)
	}
	return skill, nil
}

func (r *SkillRepository) LoadByIds(ctx context.Context, idsSkill []int64) ([]*model.Skill, error) {
	if len(idsSkill) == 0 {
		return nil, nil
	}
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+skillColumns+`
		FROM agent.skill
		WHERE id_skill = ANY($1)
		ORDER BY name
	`, idsSkill)
	if err != nil {
		return nil, fmt.Errorf("querying skills by ids: %w", err)
	}
	all, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if err != nil {
		return nil, fmt.Errorf("collecting skills by ids: %w", err)
	}
	return all, nil
}

func (r *SkillRepository) Insert(ctx context.Context, name, description, content string) (*model.Skill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		INSERT INTO agent.skill (name, description, content)
		VALUES ($1, $2, $3)
		RETURNING `+skillColumns, name, description, content)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.ErrSkillNameTaken
		}
		return nil, fmt.Errorf("inserting skill: %w", err)
	}
	skill, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.ErrSkillNameTaken
		}
		return nil, fmt.Errorf("collecting inserted skill: %w", err)
	}
	return skill, nil
}

// Reports whether the row was actually inserted; an existing builtin_key is left
// untouched and reported as not inserted.
func (r *SkillRepository) InsertBuiltin(ctx context.Context, builtinKey, name, description, content, checksum string) (int64, bool, error) {
	var idSkill int64
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO agent.skill (name, description, content, builtin_key, builtin_checksum)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (builtin_key) DO NOTHING
		RETURNING id_skill
	`, name, description, content, builtinKey, checksum).Scan(&idSkill)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		if isUniqueViolation(err) {
			return 0, false, errs.ErrSkillNameTaken
		}
		return 0, false, fmt.Errorf("inserting builtin skill %q: %w", builtinKey, err)
	}
	return idSkill, true, nil
}

func (r *SkillRepository) Update(ctx context.Context, idSkill int64, name, description, content string) (*model.Skill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		UPDATE agent.skill
		SET name = $2, description = $3, content = $4, updated_at = NOW()
		WHERE id_skill = $1
		RETURNING `+skillColumns, idSkill, name, description, content)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.ErrSkillNameTaken
		}
		return nil, fmt.Errorf("updating skill: %w", err)
	}
	skill, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrSkillNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.ErrSkillNameTaken
		}
		return nil, fmt.Errorf("collecting updated skill: %w", err)
	}
	return skill, nil
}

// Text and checksum move in one statement so a row can never record a version it
// does not hold.
func (r *SkillRepository) UpdateBuiltin(ctx context.Context, idSkill int64, name, description, content, checksum string) (*model.Skill, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		UPDATE agent.skill
		SET name = $2, description = $3, content = $4, builtin_checksum = $5, updated_at = NOW()
		WHERE id_skill = $1
		RETURNING `+skillColumns, idSkill, name, description, content, checksum)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.ErrSkillNameTaken
		}
		return nil, fmt.Errorf("updating builtin skill: %w", err)
	}
	skill, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Skill])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrSkillNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errs.ErrSkillNameTaken
		}
		return nil, fmt.Errorf("collecting updated builtin skill: %w", err)
	}
	return skill, nil
}

func (r *SkillRepository) SetChecksum(ctx context.Context, idSkill int64, checksum string) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx,
		`UPDATE agent.skill SET builtin_checksum = $2 WHERE id_skill = $1`, idSkill, checksum,
	); err != nil {
		return fmt.Errorf("setting checksum of skill %d: %w", idSkill, err)
	}
	return nil
}

func (r *SkillRepository) Delete(ctx context.Context, idSkill int64) error {
	tag, err := extctx.GetDb(ctx, r.pool).Exec(ctx,
		`DELETE FROM agent.skill WHERE id_skill = $1`, idSkill)
	if err != nil {
		return fmt.Errorf("deleting skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errs.ErrSkillNotFound
	}
	return nil
}
