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

type WikiPageDraftRepository struct {
	pool *pgxpool.Pool
}

func NewWikiPageDraftRepository(pool *pgxpool.Pool) *WikiPageDraftRepository {
	return &WikiPageDraftRepository{pool: pool}
}

func (r *WikiPageDraftRepository) Upsert(ctx context.Context, draft *model.WikiPageDraft) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO wiki.page_draft (id_page, id_user, base_version, title, summary, body)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id_page, id_user) DO UPDATE
		SET base_version = EXCLUDED.base_version, title = EXCLUDED.title, summary = EXCLUDED.summary,
		    body = EXCLUDED.body, update_at = (now() at time zone 'utc')
	`, draft.IdPage, draft.IdUser, draft.BaseVersion, draft.Title, draft.Summary, draft.Body)
	if err != nil {
		return fmt.Errorf("upserting wiki draft: %w", err)
	}
	return nil
}

func (r *WikiPageDraftRepository) Load(ctx context.Context, idPage, idUser int64) (*model.WikiPageDraft, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_page, id_user, base_version, title, summary, body, update_at
		FROM wiki.page_draft WHERE id_page = $1 AND id_user = $2
	`, idPage, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying wiki draft: %w", err)
	}
	draft, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WikiPageDraft])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting wiki draft: %w", err)
	}
	return draft, nil
}

func (r *WikiPageDraftRepository) Delete(ctx context.Context, idPage, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM wiki.page_draft WHERE id_page = $1 AND id_user = $2`, idPage, idUser)
	if err != nil {
		return fmt.Errorf("deleting wiki draft: %w", err)
	}
	return nil
}
