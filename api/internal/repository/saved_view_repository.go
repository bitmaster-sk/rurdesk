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

type SavedViewRepository struct {
	pool *pgxpool.Pool
}

func NewSavedViewRepository(pool *pgxpool.Pool) *SavedViewRepository {
	return &SavedViewRepository{pool: pool}
}

func (r *SavedViewRepository) Insert(ctx context.Context, v *model.SavedView, idUser int64) (*model.SavedView, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.saved_view(id_project, name, view_type, config, is_shared, create_by, update_by)
		VALUES($1, $2, $3, $4, $5, $6, $6)
		RETURNING id_saved_view, create_by, update_at
	`, v.IdProject, v.Name, v.ViewType, v.Config, v.IsShared, idUser).
		Scan(&v.IdSavedView, &v.CreateBy, &v.UpdateAt)
	if err != nil {
		return nil, fmt.Errorf("inserting saved view: %w", err)
	}
	return v, nil
}

// The order the client renders without re-sorting. Duplicate names are allowed, so
// id breaks the tie — otherwise two views named "Sprint" shuffle between requests.
func (r *SavedViewRepository) LoadByProject(ctx context.Context, idProject, idUser int64) ([]*model.SavedView, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_saved_view, id_project, name, view_type, config, is_shared, create_by, update_at
		FROM issues.saved_view
		WHERE id_project = $1 AND (is_shared OR create_by = $2)
		ORDER BY is_shared DESC, name, id_saved_view
	`, idProject, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying saved views: %w", err)
	}
	views, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.SavedView])
	if err != nil {
		return nil, fmt.Errorf("collecting saved views: %w", err)
	}
	return views, nil
}

// CountByCreator backs the per-author cap: the config column is never inspected by
// the server and nothing else bounds how many rows one member can insert.
func (r *SavedViewRepository) CountByCreator(ctx context.Context, idProject, idUser int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int
	err := db.QueryRow(ctx, `
		SELECT count(*) FROM issues.saved_view WHERE id_project = $1 AND create_by = $2
	`, idProject, idUser).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting saved views: %w", err)
	}
	return count, nil
}

func (r *SavedViewRepository) LoadOne(ctx context.Context, idSavedView int64) (*model.SavedView, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_saved_view, id_project, name, view_type, config, is_shared, create_by, update_at
		FROM issues.saved_view WHERE id_saved_view = $1
	`, idSavedView)
	if err != nil {
		return nil, fmt.Errorf("querying saved view: %w", err)
	}
	view, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.SavedView])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err // wrapping would hide it from the caller's errors.Is check
	}
	if err != nil {
		return nil, fmt.Errorf("collecting saved view: %w", err)
	}
	return view, nil
}

func (r *SavedViewRepository) Update(ctx context.Context, v *model.SavedView, idUser int64) (*model.SavedView, error) {
	db := extctx.GetDb(ctx, r.pool)
	// RETURNING update_at — without it the 200 response would carry the
	// pre-update timestamp loaded by LoadOne.
	err := db.QueryRow(ctx, `
		UPDATE issues.saved_view
		SET name = $2, view_type = $3, config = $4, is_shared = $5,
		    update_at = (now() at time zone 'utc'), update_by = $6
		WHERE id_saved_view = $1
		RETURNING update_at
	`, v.IdSavedView, v.Name, v.ViewType, v.Config, v.IsShared, idUser).Scan(&v.UpdateAt)
	if err != nil {
		return nil, fmt.Errorf("updating saved view: %w", err)
	}
	return v, nil
}

func (r *SavedViewRepository) Delete(ctx context.Context, idSavedView int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.saved_view WHERE id_saved_view = $1`, idSavedView)
	if err != nil {
		return fmt.Errorf("deleting saved view: %w", err)
	}
	return nil
}
