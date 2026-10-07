package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WikiIssuePageRepository struct {
	pool *pgxpool.Pool
}

type WikiIssuePageRef struct {
	model.WikiPageRef
	Source constants.WikiIssueLinkSource `json:"source" db:"source"`
}

type WikiPageIssueRef struct {
	IdIssue       int64  `json:"idIssue"       db:"id_issue"`
	IdIssuePublic int64  `json:"idIssuePublic" db:"id_issue_public"`
	Title         string `json:"title"         db:"title"`
}

func NewWikiIssuePageRepository(pool *pgxpool.Pool) *WikiIssuePageRepository {
	return &WikiIssuePageRepository{pool: pool}
}

func (r *WikiIssuePageRepository) ReplaceFromDescription(ctx context.Context, idIssue int64, idsPage []int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM wiki.issue_page WHERE id_issue = $1 AND source = 'description' AND NOT (id_page = ANY($2))
	`, idIssue, idsPage)
	if err != nil {
		return fmt.Errorf("clearing issue wiki description links: %w", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO wiki.issue_page (id_issue, id_page, source)
		SELECT $1, unnest($2::bigint[]), 'description'
		ON CONFLICT (id_issue, id_page) DO NOTHING
	`, idIssue, idsPage)
	if err != nil {
		return fmt.Errorf("inserting issue wiki description links: %w", err)
	}
	return nil
}

func (r *WikiIssuePageRepository) Insert(ctx context.Context, idIssue, idPage int64, source constants.WikiIssueLinkSource) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO wiki.issue_page (id_issue, id_page, source) VALUES ($1, $2, $3)
		ON CONFLICT (id_issue, id_page) DO UPDATE SET source = EXCLUDED.source
	`, idIssue, idPage, source)
	if err != nil {
		return fmt.Errorf("inserting issue wiki link: %w", err)
	}
	return nil
}

func (r *WikiIssuePageRepository) Delete(ctx context.Context, idIssue, idPage int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM wiki.issue_page WHERE id_issue = $1 AND id_page = $2`, idIssue, idPage)
	if err != nil {
		return fmt.Errorf("deleting issue wiki link: %w", err)
	}
	return nil
}

func (r *WikiIssuePageRepository) LoadByIssue(ctx context.Context, idIssue int64, hideHidden bool) ([]*WikiIssuePageRef, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT p.id_page, p.id_space, p.slug, p.title, ip.source
		FROM wiki.issue_page ip
		INNER JOIN wiki.page p ON p.id_page = ip.id_page
		WHERE ip.id_issue = $1 AND p.deleted_at IS NULL AND (NOT $2 OR p.agent_access <> 'hidden')
		ORDER BY p.title
	`, idIssue, hideHidden)
	if err != nil {
		return nil, fmt.Errorf("querying issue wiki pages: %w", err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[WikiIssuePageRef])
	if err != nil {
		return nil, fmt.Errorf("collecting issue wiki pages: %w", err)
	}
	return refs, nil
}

func (r *WikiIssuePageRepository) LoadByPage(ctx context.Context, idPage int64, idsProject []int64) ([]*WikiPageIssueRef, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT i.id_issue, i.id_issue_public, i.title
		FROM wiki.issue_page ip
		INNER JOIN issues.issue i ON i.id_issue = ip.id_issue
		WHERE ip.id_page = $1 AND i.id_project = ANY($2)
		ORDER BY i.id_issue_public
	`, idPage, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying wiki page issues: %w", err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[WikiPageIssueRef])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki page issues: %w", err)
	}
	return refs, nil
}
