package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const tagNameUniqueIndex = "tag_project_name_uidx"

type TagRepository struct {
	pool *pgxpool.Pool
}

func NewTagRepository(pool *pgxpool.Pool) *TagRepository {
	return &TagRepository{pool: pool}
}

func (r *TagRepository) LoadTags(ctx context.Context, idsProject []int64) ([]*model.Tag, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			tag.id_tag,
			tag.id_project,
			tag.name,
			tag.color,
			tag.order_rank,
			(SELECT count(*) FROM issues.issue_tag itg WHERE itg.id_tag = tag.id_tag) AS issue_count
		FROM
			issues.tag tag
		WHERE
			tag.id_project = ANY($1)
		ORDER BY tag.id_project, tag.order_rank
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying tags: %w", err)
	}
	tags, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Tag])
	if err != nil {
		return nil, fmt.Errorf("collecting tags: %w", err)
	}
	return tags, nil
}

func (r *TagRepository) LoadTag(ctx context.Context, idProject, idTag int64) (*model.Tag, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			tag.id_tag,
			tag.id_project,
			tag.name,
			tag.color,
			tag.order_rank,
			(SELECT count(*) FROM issues.issue_tag itg WHERE itg.id_tag = tag.id_tag) AS issue_count
		FROM
			issues.tag tag
		WHERE
			tag.id_project = $1 AND
			tag.id_tag = $2
	`, idProject, idTag)
	if err != nil {
		return nil, fmt.Errorf("querying tag: %w", err)
	}
	tag, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Tag])
	if err != nil {
		return nil, fmt.Errorf("collecting tag: %w", err)
	}
	return tag, nil
}

func (r *TagRepository) CountTagsInProject(ctx context.Context, idProject int64, idsTag []int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int
	err := db.QueryRow(ctx, `
		SELECT count(*) FROM issues.tag WHERE id_project = $1 AND id_tag = ANY($2)
	`, idProject, idsTag).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting tags in project: %w", err)
	}
	return count, nil
}

func (r *TagRepository) InsertTag(ctx context.Context, tag *model.Tag) (*model.Tag, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.tag (id_project, name, color, order_rank)
		SELECT $1, $2, $3, COALESCE(MAX(order_rank), 0) + 1 FROM issues.tag WHERE id_project = $1
		RETURNING id_tag, order_rank
	`, tag.IdProject, tag.Name, tag.Color).Scan(&tag.IdTag, &tag.OrderRank)
	if err != nil {
		return nil, mapTagWriteErr("inserting tag", err)
	}
	return tag, nil
}

func (r *TagRepository) UpdateTag(ctx context.Context, tag *model.Tag) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.tag SET name = $1, color = $2 WHERE id_tag = $3 AND id_project = $4
	`, tag.Name, tag.Color, tag.IdTag, tag.IdProject)
	if err != nil {
		return mapTagWriteErr("updating tag", err)
	}
	return nil
}

func (r *TagRepository) MoveTag(ctx context.Context, tag *model.Tag) error {
	db := extctx.GetDb(ctx, r.pool)
	var oldOrderRank int
	err := db.QueryRow(ctx, `
		UPDATE issues.tag new SET
			order_rank = $3
		FROM issues.tag old
		WHERE
			old.id_tag = new.id_tag AND
			old.id_tag = $1 AND
			old.id_project = $2
		RETURNING old.order_rank
	`, tag.IdTag, tag.IdProject, tag.OrderRank).Scan(&oldOrderRank)
	if err != nil {
		return fmt.Errorf("moving tag: %w", err)
	}
	return r.reorder(ctx, tag.IdProject, tag.IdTag, oldOrderRank)
}

func (r *TagRepository) DeleteTag(ctx context.Context, idProject, idTag int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM issues.tag WHERE id_tag = $1 AND id_project = $2
	`, idTag, idProject)
	if err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}
	return r.reorder(ctx, idProject, idTag, 0)
}

func (r *TagRepository) reorder(ctx context.Context, idProject, idTag int64, oldOrderRank int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.tag des SET
			order_rank = src.order_rank
		FROM (
			SELECT
				src_tag.id_tag,
				ROW_NUMBER() OVER (
					ORDER BY
						order_rank,
						CASE
							WHEN src_tag.id_tag = $1 AND src_tag.order_rank > $3 THEN 1
							WHEN src_tag.id_tag = $1 AND src_tag.order_rank < $3 THEN -1
							ELSE 0
						END
				) AS order_rank
			FROM issues.tag src_tag
			WHERE src_tag.id_project = $2
		) src
		WHERE des.id_tag = src.id_tag AND des.id_project = $2
	`, idTag, idProject, oldOrderRank)
	if err != nil {
		return fmt.Errorf("reordering tags: %w", err)
	}
	return nil
}

func mapTagWriteErr(action string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == tagNameUniqueIndex {
		return errs.ErrTagNameExists
	}
	return fmt.Errorf("%s: %w", action, err)
}
