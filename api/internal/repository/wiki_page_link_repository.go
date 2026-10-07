package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WikiPageLinkRepository struct {
	pool *pgxpool.Pool
}

func NewWikiPageLinkRepository(pool *pgxpool.Pool) *WikiPageLinkRepository {
	return &WikiPageLinkRepository{pool: pool}
}

func (r *WikiPageLinkRepository) Replace(ctx context.Context, idPage int64, links []model.WikiLink) error {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `DELETE FROM wiki.page_link WHERE id_page_from = $1`, idPage); err != nil {
		return fmt.Errorf("clearing wiki links: %w", err)
	}
	if len(links) == 0 {
		return nil
	}
	idsSpace := make([]int64, len(links))
	slugs := make([]string, len(links))
	for i, link := range links {
		idsSpace[i] = link.IdSpaceTo
		slugs[i] = link.TargetSlug
	}
	_, err := db.Exec(ctx, `
		INSERT INTO wiki.page_link (id_page_from, id_space_to, target_slug)
		SELECT $1, t.id_space, t.slug FROM unnest($2::bigint[], $3::text[]) AS t(id_space, slug)
		ON CONFLICT DO NOTHING
	`, idPage, idsSpace, slugs)
	if err != nil {
		return fmt.Errorf("inserting wiki links: %w", err)
	}
	return nil
}

func (r *WikiPageLinkRepository) LoadBacklinks(ctx context.Context, idSpace int64, slug string, idsSourceSpace []int64, hideHidden bool) ([]*model.WikiPageRef, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT p.id_page, p.id_space, p.slug, p.title
		FROM wiki.page_link l
		INNER JOIN wiki.page p ON p.id_page = l.id_page_from
		WHERE l.id_space_to = $1 AND l.target_slug = $2 AND p.deleted_at IS NULL
		  AND (NOT $3 OR p.agent_access <> 'hidden') AND p.id_space = ANY($4)
		ORDER BY p.title
	`, idSpace, slug, hideHidden, idsSourceSpace)
	if err != nil {
		return nil, fmt.Errorf("querying wiki backlinks: %w", err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiPageRef])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki backlinks: %w", err)
	}
	return refs, nil
}
