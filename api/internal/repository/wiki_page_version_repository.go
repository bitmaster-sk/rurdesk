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

type WikiPageVersionRepository struct {
	pool *pgxpool.Pool
}

func NewWikiPageVersionRepository(pool *pgxpool.Pool) *WikiPageVersionRepository {
	return &WikiPageVersionRepository{pool: pool}
}

func (r *WikiPageVersionRepository) Insert(ctx context.Context, version *model.WikiPageVersion) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO wiki.page_version (id_page, version_no, id_parent, title, summary, body, agent_access, note, merged_from, create_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, version.IdPage, version.VersionNo, version.IdParent, version.Title, version.Summary, version.Body,
		version.AgentAccess, version.Note, version.MergedFrom, version.CreateBy)
	if err != nil {
		return fmt.Errorf("inserting wiki page version: %w", err)
	}
	return nil
}

func (r *WikiPageVersionRepository) Load(ctx context.Context, idPage int64) ([]*model.WikiPageVersionSummary, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT version_no, title, agent_access, note, merged_from, create_at, create_by
		FROM wiki.page_version WHERE id_page = $1
		ORDER BY version_no DESC
	`, idPage)
	if err != nil {
		return nil, fmt.Errorf("querying wiki versions: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiPageVersionSummary])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki versions: %w", err)
	}
	return versions, nil
}

func (r *WikiPageVersionRepository) LoadOne(ctx context.Context, idPage int64, versionNo int) (*model.WikiPageVersion, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_version, id_page, version_no, id_parent, title, summary, body, agent_access, note, merged_from, create_at, create_by
		FROM wiki.page_version WHERE id_page = $1 AND version_no = $2
	`, idPage, versionNo)
	if err != nil {
		return nil, fmt.Errorf("querying wiki version: %w", err)
	}
	version, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WikiPageVersion])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting wiki version: %w", err)
	}
	return version, nil
}
