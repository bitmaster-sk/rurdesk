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

type IssueHierarchyRepository struct {
	pool *pgxpool.Pool
}

func NewIssueHierarchyRepository(pool *pgxpool.Pool) *IssueHierarchyRepository {
	return &IssueHierarchyRepository{pool: pool}
}

func (r *IssueHierarchyRepository) LoadParentIds(ctx context.Context, idsIssue []int64) ([]int64, error) {
	if len(idsIssue) == 0 {
		return nil, nil
	}
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT DISTINCT id_issue_from
		FROM issues.issue_relation
		WHERE relation_type = 'hierarchy' AND id_issue_to = ANY($1)
	`, idsIssue)
	if err != nil {
		return nil, fmt.Errorf("loading hierarchy parents: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting hierarchy parents: %w", err)
	}
	return ids, nil
}

func (r *IssueHierarchyRepository) LoadParentRelation(ctx context.Context, idIssue int64) (idRelation, idParent int64, err error) {
	db := extctx.GetDb(ctx, r.pool)
	err = db.QueryRow(ctx, `
		SELECT id_issue_relation, id_issue_from
		FROM issues.issue_relation
		WHERE relation_type = 'hierarchy' AND id_issue_to = $1
	`, idIssue).Scan(&idRelation, &idParent)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("loading parent relation: %w", err)
	}
	return idRelation, idParent, nil
}

// Derives each node from its direct children only, so children must be recomputed first.
// Keep in sync with migration 20261105120000_issue_summary_rollup.sql.
func (r *IssueHierarchyRepository) RecomputeNodes(ctx context.Context, idsIssue []int64) ([]model.RecomputedIssue, error) {
	if len(idsIssue) == 0 {
		return nil, nil
	}
	db := extctx.GetDb(ctx, r.pool)
	// The lock must stay a separate statement so the aggregate below reads children the lock holder committed.
	locked, err := db.Query(ctx, `
		SELECT id_issue FROM issues.issue
		WHERE id_issue = ANY($1)
		ORDER BY id_issue
		FOR UPDATE
	`, idsIssue)
	if err != nil {
		return nil, fmt.Errorf("locking hierarchy nodes: %w", err)
	}
	locked.Close()
	if err := locked.Err(); err != nil {
		return nil, fmt.Errorf("locking hierarchy nodes: %w", err)
	}
	rows, err := db.Query(ctx, `
		WITH agg AS (
			SELECT
				r.id_issue_from AS id_parent,
				bool_or(c.scheduled_at IS NOT NULL) AS any_scheduled,
				min(c.scheduled_at) AS min_start,
				max(c.scheduled_at + make_interval(secs => COALESCE(NULLIF(c.estimated, 0), 3600))) AS max_end,
				sum(CASE WHEN c.rollup_estimated IS NOT NULL THEN c.rollup_estimated ELSE COALESCE(NULLIF(c.estimated, 0), 3600) END) AS rollup_estimated,
				sum(CASE WHEN c.rollup_tracked IS NOT NULL THEN c.rollup_tracked ELSE c.tracked END) AS rollup_tracked,
				sum((CASE WHEN c.scheduled_at IS NULL THEN 1 ELSE 0 END) + c.backlog_children) AS backlog_children
			FROM issues.issue_relation r
			INNER JOIN issues.issue c ON c.id_issue = r.id_issue_to
			WHERE r.relation_type = 'hierarchy' AND r.id_issue_from = ANY($1)
			GROUP BY r.id_issue_from
		),
		computed AS (
			SELECT
				p.id_issue,
				COALESCE(a.any_scheduled, FALSE) AS is_summary,
				CASE WHEN a.any_scheduled THEN a.min_start ELSE p.scheduled_at END AS scheduled_at,
				CASE WHEN a.any_scheduled THEN EXTRACT(EPOCH FROM a.max_end - a.min_start)::BIGINT ELSE p.estimated END AS estimated,
				a.rollup_estimated,
				CASE WHEN a.id_parent IS NULL THEN NULL ELSE a.rollup_tracked + p.tracked END AS rollup_tracked,
				COALESCE(a.backlog_children, 0) AS backlog_children
			FROM issues.issue p
			LEFT JOIN agg a ON a.id_parent = p.id_issue
			WHERE p.id_issue = ANY($1)
		)
		UPDATE issues.issue p
		SET is_summary = c.is_summary,
			scheduled_at = c.scheduled_at,
			estimated = c.estimated,
			rollup_estimated = c.rollup_estimated,
			rollup_tracked = c.rollup_tracked,
			backlog_children = c.backlog_children
		FROM computed c, issues.issue prev
		WHERE p.id_issue = c.id_issue
		  AND prev.id_issue = p.id_issue
		  AND (p.is_summary, p.scheduled_at, p.estimated, p.rollup_estimated, p.rollup_tracked, p.backlog_children)
		      IS DISTINCT FROM (c.is_summary, c.scheduled_at, c.estimated, c.rollup_estimated, c.rollup_tracked, c.backlog_children)
		RETURNING p.id_issue,
		          prev.is_summary, prev.scheduled_at, prev.estimated, prev.rollup_estimated, prev.rollup_tracked,
		          p.is_summary, p.scheduled_at, p.estimated, p.rollup_estimated, p.rollup_tracked
	`, idsIssue)
	if err != nil {
		return nil, fmt.Errorf("recomputing hierarchy nodes: %w", err)
	}
	recomputed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.RecomputedIssue, error) {
		var issue model.RecomputedIssue
		err := row.Scan(
			&issue.IdIssue,
			&issue.Before.IsSummary, &issue.Before.ScheduledAt, &issue.Before.Estimated, &issue.Before.RollupEstimated, &issue.Before.RollupTracked,
			&issue.After.IsSummary, &issue.After.ScheduledAt, &issue.After.Estimated, &issue.After.RollupEstimated, &issue.After.RollupTracked,
		)
		return issue, err
	})
	if err != nil {
		return nil, fmt.Errorf("collecting recomputed nodes: %w", err)
	}
	return recomputed, nil
}

// Sees only committed-in-tx state: run it after the change, inside the same transaction.
// Schedule links between a task and its own ancestor are ignored, as the gantt cascade ignores them.
func (r *IssueHierarchyRepository) HasScheduleCycleThroughHierarchy(ctx context.Context, idProject int64, idsRoot []int64) (bool, error) {
	if len(idsRoot) == 0 {
		return false, nil
	}
	db := extctx.GetDb(ctx, r.pool)
	var flag int
	err := db.QueryRow(ctx, `
		WITH RECURSIVE
		h AS (
			SELECT id_issue_from AS id_parent, id_issue_to AS id_child
			FROM issues.issue_relation
			WHERE id_project = $1 AND relation_type = 'hierarchy'
		),
		sub(id_root, id_node) AS (
			SELECT id_issue, id_issue FROM issues.issue WHERE id_project = $1
			UNION
			SELECT s.id_root, h.id_child FROM sub s INNER JOIN h ON h.id_parent = s.id_node
		),
		leaf AS (
			SELECT s.id_root, s.id_node AS id_leaf
			FROM sub s
			WHERE NOT EXISTS (SELECT 1 FROM h WHERE h.id_parent = s.id_node)
		),
		e AS (
			SELECT DISTINCT lf.id_leaf AS id_a, lt.id_leaf AS id_b
			FROM issues.issue_relation r
			INNER JOIN leaf lf ON lf.id_root = r.id_issue_from
			INNER JOIN leaf lt ON lt.id_root = r.id_issue_to
			WHERE r.id_project = $1 AND r.relation_type = 'schedule'
			  AND NOT EXISTS (
				SELECT 1 FROM sub s
				WHERE (s.id_root = r.id_issue_from AND s.id_node = r.id_issue_to)
				   OR (s.id_root = r.id_issue_to AND s.id_node = r.id_issue_from)
			  )
		),
		walk(id_start, id_cur) AS (
			SELECT e.id_a, e.id_b
			FROM e
			WHERE e.id_a IN (SELECT id_leaf FROM leaf WHERE id_root = ANY($2))
			UNION
			SELECT w.id_start, e.id_b
			FROM walk w
			INNER JOIN e ON e.id_a = w.id_cur
			WHERE w.id_cur <> w.id_start
		)
		SELECT 1 FROM walk WHERE id_cur = id_start LIMIT 1
	`, idProject, idsRoot).Scan(&flag)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking schedule cycle through hierarchy: %w", err)
	}
	return true, nil
}

func (r *IssueHierarchyRepository) IsOnSameBranch(ctx context.Context, idA, idB int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var found bool
	err := db.QueryRow(ctx, `
		WITH RECURSIVE up(id_start, id_node) AS (
			SELECT id, id FROM unnest(ARRAY[$1::BIGINT, $2::BIGINT]) AS id
			UNION
			SELECT u.id_start, r.id_issue_from
			FROM up u
			INNER JOIN issues.issue_relation r ON r.relation_type = 'hierarchy' AND r.id_issue_to = u.id_node
		)
		SELECT EXISTS (
			SELECT 1 FROM up
			WHERE (id_start = $1 AND id_node = $2) OR (id_start = $2 AND id_node = $1)
		)
	`, idA, idB).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("checking hierarchy branch: %w", err)
	}
	return found, nil
}

// Concurrent bulk writes that touch overlapping issues deadlock unless they lock rows in one order.
func (r *IssueHierarchyRepository) LockIssuesInOrder(ctx context.Context, idProject int64, idsIssuePublic []int64) error {
	if len(idsIssuePublic) == 0 {
		return nil
	}
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_issue FROM issues.issue
		WHERE id_project = $1 AND id_issue_public = ANY($2)
		ORDER BY id_issue
		FOR UPDATE
	`, idProject, idsIssuePublic)
	if err != nil {
		return fmt.Errorf("locking issues: %w", err)
	}
	rows.Close()
	return rows.Err()
}
