package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var defaultIssueTypeNames = []string{"Bug", "Feature", "Task"}

type IssueTypeRepository struct {
	pool *pgxpool.Pool
}

func NewIssueTypeRepository(pool *pgxpool.Pool) *IssueTypeRepository {
	return &IssueTypeRepository{pool: pool}
}

func (r *IssueTypeRepository) LoadIssueTypes(ctx context.Context, idsProject []int64) ([]*model.IssueType, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			id_issue_type,
			id_project,
			name,
			protected,
			order_rank
		FROM
			issues.issue_type
		WHERE
			id_project = ANY($1)
		ORDER BY id_project, order_rank
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying issue types: %w", err)
	}
	issueTypes, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.IssueType])
	if err != nil {
		return nil, fmt.Errorf("collecting issue types: %w", err)
	}
	return issueTypes, nil
}

func (r *IssueTypeRepository) LoadIssueType(ctx context.Context, idProject, idIssueType int64) (*model.IssueType, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			id_issue_type,
			id_project,
			name,
			protected,
			order_rank
		FROM
			issues.issue_type
		WHERE
			id_project = $1 AND
			id_issue_type = $2
	`, idProject, idIssueType)
	if err != nil {
		return nil, fmt.Errorf("querying issue type: %w", err)
	}
	issueType, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.IssueType])
	if err != nil {
		return nil, fmt.Errorf("collecting issue type: %w", err)
	}
	return issueType, nil
}

func (r *IssueTypeRepository) LoadIssueTypeUsage(ctx context.Context, idProject, idIssueType int64) (*model.IssueTypeUsage, error) {
	db := extctx.GetDb(ctx, r.pool)
	u := &model.IssueTypeUsage{}
	err := db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM issues.issue WHERE id_project = $1 AND id_issue_type = $2),
			EXISTS (SELECT 1 FROM projects.project WHERE id_project = $1 AND id_issue_type_default = $2)
	`, idProject, idIssueType).Scan(&u.Issues, &u.IsProjectDefault)
	if err != nil {
		return nil, fmt.Errorf("querying issue type usage: %w", err)
	}
	return u, nil
}

func (r *IssueTypeRepository) InsertDefaultIssueTypes(ctx context.Context, idProject int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.issue_type (id_project, name, order_rank)
		SELECT $1, name, ordinality FROM unnest($2::text[]) WITH ORDINALITY AS seed(name, ordinality)
	`, idProject, defaultIssueTypeNames)
	if err != nil {
		return fmt.Errorf("inserting default issue types: %w", err)
	}
	return nil
}

func (r *IssueTypeRepository) InsertIssueType(ctx context.Context, it *model.IssueType) (*model.IssueType, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.issue_type (id_project, name, protected, order_rank)
		SELECT $1, $2, false, COALESCE(MAX(order_rank), 0) + 1 FROM issues.issue_type WHERE id_project = $1
		RETURNING id_issue_type, order_rank
	`, it.IdProject, it.Name).Scan(&it.IdIssueType, &it.OrderRank)
	if err != nil {
		return nil, fmt.Errorf("inserting issue type: %w", err)
	}
	it.Protected = false
	return it, nil
}

func (r *IssueTypeRepository) UpdateIssueType(ctx context.Context, it *model.IssueType) (*model.IssueType, error) {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue_type SET name = $1 WHERE id_issue_type = $2 AND id_project = $3
	`, it.Name, it.IdIssueType, it.IdProject)
	if err != nil {
		return nil, fmt.Errorf("updating issue type: %w", err)
	}
	return it, nil
}

func (r *IssueTypeRepository) MoveIssueType(ctx context.Context, it *model.IssueType) error {
	db := extctx.GetDb(ctx, r.pool)
	var oldOrderRank int
	err := db.QueryRow(ctx, `
		UPDATE issues.issue_type new SET
			order_rank = $3
		FROM issues.issue_type old
		WHERE
			old.id_issue_type = new.id_issue_type AND
			old.id_issue_type = $1 AND
			old.id_project = $2
		RETURNING old.order_rank
	`, it.IdIssueType, it.IdProject, it.OrderRank).Scan(&oldOrderRank)
	if err != nil {
		return fmt.Errorf("moving issue type: %w", err)
	}
	return r.reorder(ctx, it.IdProject, it.IdIssueType, oldOrderRank)
}

func (r *IssueTypeRepository) DeleteIssueType(ctx context.Context, idProject, idIssueType int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM issues.issue_type WHERE id_issue_type = $1 AND id_project = $2
	`, idIssueType, idProject)
	if err != nil {
		return fmt.Errorf("deleting issue type: %w", err)
	}
	return r.reorder(ctx, idProject, idIssueType, 0)
}

func (r *IssueTypeRepository) ReassignIssuesIssueType(ctx context.Context, idProject, oldIdIssueType int64, newIdIssueType *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue SET id_issue_type = $3 WHERE id_project = $1 AND id_issue_type = $2
	`, idProject, oldIdIssueType, newIdIssueType)
	if err != nil {
		return fmt.Errorf("reassigning issues issue type: %w", err)
	}
	return nil
}

func (r *IssueTypeRepository) RepointProjectDefaultIssueType(ctx context.Context, idProject, oldIdIssueType int64, newIdIssueType *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE projects.project SET id_issue_type_default = $3
		WHERE id_project = $1 AND id_issue_type_default = $2
	`, idProject, oldIdIssueType, newIdIssueType)
	if err != nil {
		return fmt.Errorf("repointing project default issue type: %w", err)
	}
	return nil
}

func (r *IssueTypeRepository) reorder(ctx context.Context, idProject, idIssueType int64, oldOrderRank int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue_type des SET
			order_rank = src.order_rank
		FROM (
			SELECT
				src_it.id_issue_type,
				ROW_NUMBER() OVER (
					ORDER BY
						order_rank,
						CASE
							WHEN src_it.id_issue_type = $1 AND src_it.order_rank > $3 THEN 1
							WHEN src_it.id_issue_type = $1 AND src_it.order_rank < $3 THEN -1
							ELSE 0
						END
				) AS order_rank
			FROM issues.issue_type src_it
			WHERE src_it.id_project = $2
		) src
		WHERE des.id_issue_type = src.id_issue_type AND des.id_project = $2
	`, idIssueType, idProject, oldOrderRank)
	if err != nil {
		return fmt.Errorf("reordering issue types: %w", err)
	}
	return nil
}
