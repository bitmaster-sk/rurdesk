package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SeverityRepository struct {
	pool *pgxpool.Pool
}

func NewSeverityRepository(pool *pgxpool.Pool) *SeverityRepository {
	return &SeverityRepository{pool: pool}
}

func (r *SeverityRepository) LoadSeverities(ctx context.Context, idsProject []int64) ([]*model.Severity, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			sev.id_severity,
			pis.id_project,
			sev.title,
			sev.color,
			sev.protected,
			pis.order_rank
		FROM
			issues.severity sev
			INNER JOIN projects.project_issue_severity pis ON sev.id_severity = pis.id_severity
		WHERE
			pis.id_project = ANY($1)
		ORDER BY pis.order_rank
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying severities: %w", err)
	}
	severities, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Severity])
	if err != nil {
		return nil, fmt.Errorf("collecting severities: %w", err)
	}
	return severities, nil
}

func (r *SeverityRepository) LoadSeverity(ctx context.Context, idProject int64, idSeverity int64) (*model.Severity, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			sev.id_severity,
			pis.id_project,
			sev.title,
			sev.color,
			sev.protected,
			pis.order_rank
		FROM
			issues.severity sev
			INNER JOIN projects.project_issue_severity pis ON sev.id_severity = pis.id_severity
		WHERE
			pis.id_project = $1 AND
			sev.id_severity = $2
	`, idProject, idSeverity)
	if err != nil {
		return nil, fmt.Errorf("querying severity: %w", err)
	}
	severity, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Severity])
	if err != nil {
		return nil, fmt.Errorf("collecting severity: %w", err)
	}
	return severity, nil
}

func (r *SeverityRepository) InsertDefaultSeverities(ctx context.Context, idProject int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO projects.project_issue_severity (id_project, id_severity, order_rank)
		SELECT $1, id_severity, ROW_NUMBER() OVER (ORDER BY id_severity) FROM issues.severity WHERE protected = true
	`, idProject)
	if err != nil {
		return fmt.Errorf("inserting default severities: %w", err)
	}
	return nil
}

func (r *SeverityRepository) InsertSeverity(ctx context.Context, sev *model.Severity) (*model.Severity, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.severity(title, color, protected)
		VALUES($1, $2, false) RETURNING id_severity
	`, sev.Title, sev.Color).Scan(&sev.IdSeverity)
	if err != nil {
		return nil, fmt.Errorf("inserting severity: %w", err)
	}
	sev.Protected = false
	return sev, nil
}

func (r *SeverityRepository) UpdateSeverity(ctx context.Context, sev *model.Severity) (*model.Severity, error) {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.severity SET title = $1, color = $2 WHERE id_severity = $3
	`, sev.Title, sev.Color, sev.IdSeverity)
	if err != nil {
		return nil, fmt.Errorf("updating severity: %w", err)
	}
	return sev, nil
}

func (r *SeverityRepository) DeleteSeverity(ctx context.Context, idSeverity int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.severity WHERE id_severity = $1`, idSeverity)
	if err != nil {
		return fmt.Errorf("deleting severity: %w", err)
	}
	return nil
}

// ReassignIssuesSeverity repoints every issue in a project from one severity
// to another (or to NULL when newIdSeverity is nil). Mirrors
// ReassignIssuesState. Scoped to id_project so other projects sharing the
// (protected) row are untouched.
func (r *SeverityRepository) ReassignIssuesSeverity(ctx context.Context, idProject, oldIdSeverity int64, newIdSeverity *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue SET id_severity = $3 WHERE id_project = $1 AND id_severity = $2
	`, idProject, oldIdSeverity, newIdSeverity)
	if err != nil {
		return fmt.Errorf("reassigning issues severity: %w", err)
	}
	return nil
}

func (r *SeverityRepository) InsertProjectSeverity(ctx context.Context, sev *model.Severity) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO projects.project_issue_severity (id_project, id_severity, order_rank)
		SELECT $1, $2, COALESCE(MAX(order_rank), 0) + 1 FROM projects.project_issue_severity WHERE id_project = $1
	`, sev.IdProject, sev.IdSeverity)
	if err != nil {
		return fmt.Errorf("inserting project severity: %w", err)
	}
	return nil
}

func (r *SeverityRepository) DeleteProjectSeverity(ctx context.Context, sev *model.Severity) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM projects.project_issue_severity WHERE id_project = $1 AND id_severity = $2
	`, sev.IdProject, sev.IdSeverity)
	if err != nil {
		return fmt.Errorf("deleting project severity: %w", err)
	}
	return r.reorder(ctx, sev, 0)
}

func (r *SeverityRepository) UpdateProjectSeverity(ctx context.Context, sev *model.Severity) error {
	db := extctx.GetDb(ctx, r.pool)
	var oldOrderRank int64
	err := db.QueryRow(ctx, `
		UPDATE projects.project_issue_severity new SET
			order_rank = $3
		FROM projects.project_issue_severity old
		WHERE
			old.id_project = new.id_project AND
			old.id_project = $2 AND
			old.id_severity = new.id_severity AND
			old.id_severity = $1
		RETURNING old.order_rank
	`, sev.IdSeverity, sev.IdProject, sev.OrderRank).Scan(&oldOrderRank)
	if err != nil {
		return fmt.Errorf("updating project severity: %w", err)
	}
	return r.reorder(ctx, sev, oldOrderRank)
}

// reorder renumbers order_rank sequentially after an insert/delete/move.
func (r *SeverityRepository) reorder(ctx context.Context, sev *model.Severity, oldOrderRank int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE projects.project_issue_severity des SET
			order_rank = src.order_rank
		FROM (
			SELECT
				src_pis.id_project,
				src_pis.id_severity,
				ROW_NUMBER() OVER (
					ORDER BY
						order_rank,
						CASE
							WHEN src_pis.id_severity = $1 AND src_pis.order_rank > $3 THEN 1
							WHEN src_pis.id_severity = $1 AND src_pis.order_rank < $3 THEN -1
							ELSE 0
						END
				) AS order_rank
			FROM projects.project_issue_severity src_pis
			WHERE src_pis.id_project = $2
		) src
		WHERE des.id_project = src.id_project AND des.id_severity = src.id_severity AND des.id_project = $2
	`, sev.IdSeverity, sev.IdProject, oldOrderRank)
	if err != nil {
		return fmt.Errorf("reordering project severities: %w", err)
	}
	return nil
}
