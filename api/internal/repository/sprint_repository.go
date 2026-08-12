package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SprintRepository struct {
	pool *pgxpool.Pool
}

func NewSprintRepository(pool *pgxpool.Pool) *SprintRepository {
	return &SprintRepository{pool: pool}
}

func (r *SprintRepository) Insert(ctx context.Context, s *model.Sprint, idUser int64) (*model.Sprint, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.sprint(id_project, name, start_at, end_at, state, create_by, update_by)
		VALUES($1, $2, $3, $4, $5, $6, $6)
		RETURNING id_sprint
	`, s.IdProject, s.Name, s.StartAt, s.EndAt, s.State, idUser).Scan(&s.IdSprint)
	if err != nil {
		return nil, fmt.Errorf("inserting sprint: %w", err)
	}
	return s, nil
}

func (r *SprintRepository) LoadByProject(ctx context.Context, idProject int64) ([]*model.Sprint, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_sprint, id_project, name, start_at, end_at, state
		FROM issues.sprint WHERE id_project = $1 ORDER BY start_at DESC
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying sprints: %w", err)
	}
	sprints, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Sprint])
	if err != nil {
		return nil, fmt.Errorf("collecting sprints: %w", err)
	}
	return sprints, nil
}

func (r *SprintRepository) LoadOne(ctx context.Context, idSprint int64) (*model.Sprint, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_sprint, id_project, name, start_at, end_at, state
		FROM issues.sprint WHERE id_sprint = $1
	`, idSprint)
	if err != nil {
		return nil, fmt.Errorf("querying sprint: %w", err)
	}
	sprint, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Sprint])
	if err != nil {
		return nil, fmt.Errorf("collecting sprint: %w", err)
	}
	return sprint, nil
}

func (r *SprintRepository) Update(ctx context.Context, s *model.Sprint, idUser int64) (*model.Sprint, error) {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.sprint
		SET name = $2, start_at = $3, end_at = $4, state = $5,
		    update_at = (now() at time zone 'utc'), update_by = $6
		WHERE id_sprint = $1
	`, s.IdSprint, s.Name, s.StartAt, s.EndAt, s.State, idUser)
	if err != nil {
		return nil, fmt.Errorf("updating sprint: %w", err)
	}
	return s, nil
}

func (r *SprintRepository) Delete(ctx context.Context, idSprint int64) error {
	db := extctx.GetDb(ctx, r.pool)
	// issue.id_sprint is ON DELETE SET NULL, so member issues fall back to Backlog.
	_, err := db.Exec(ctx, `DELETE FROM issues.sprint WHERE id_sprint = $1`, idSprint)
	if err != nil {
		return fmt.Errorf("deleting sprint: %w", err)
	}
	return nil
}

var trailingSeq = regexp.MustCompile(`(\d+)\s*$`)

// MaxNameSeq returns the highest trailing integer among the project's sprint
// names ("Sprint 12" → 12), 0 if none — the basis for the next auto-name.
func (r *SprintRepository) MaxNameSeq(ctx context.Context, idProject int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT name FROM issues.sprint WHERE id_project = $1`, idProject)
	if err != nil {
		return 0, fmt.Errorf("querying sprint names: %w", err)
	}
	defer rows.Close()
	maxSeq := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, fmt.Errorf("scanning sprint name: %w", err)
		}
		if m := trailingSeq.FindStringSubmatch(name); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > maxSeq {
				maxSeq = n
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating sprint names: %w", err)
	}
	return maxSeq, nil
}

func (r *SprintRepository) LatestEnd(ctx context.Context, idProject int64) (*time.Time, error) {
	db := extctx.GetDb(ctx, r.pool)
	var t *time.Time
	if err := db.QueryRow(ctx, `SELECT max(end_at) FROM issues.sprint WHERE id_project = $1`, idProject).Scan(&t); err != nil {
		return nil, fmt.Errorf("querying latest sprint end: %w", err)
	}
	return t, nil
}

func (r *SprintRepository) NextPlanned(ctx context.Context, idProject, excludeSprint int64) (*model.Sprint, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_sprint, id_project, name, start_at, end_at, state
		FROM issues.sprint
		WHERE id_project = $1 AND state = $3 AND id_sprint <> $2
		ORDER BY start_at LIMIT 1
	`, idProject, excludeSprint, constants.SprintStatePlanned)
	if err != nil {
		return nil, fmt.Errorf("querying next planned sprint: %w", err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Sprint])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting next planned sprint: %w", err)
	}
	return s, nil
}

// AssignIssue sets/clears id_sprint; the same-project rule is enforced in SQL.
// 0 rows (false) covers both "issue not found" and "sprint belongs to
// another project" — the endpoint maps both to one 400, no pre-check query.
// Bumps update_at/update_by but deliberately sends no notification; those
// stay reserved for state/severity/assignee changes.
func (r *SprintRepository) AssignIssue(ctx context.Context, idProject, idIssuePublic int64, idSprint *int64, idUser int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE issues.issue iss
		SET id_sprint = $3,
		    update_at = (now() at time zone 'utc'),
		    update_by = $4
		WHERE iss.id_project = $1 AND iss.id_issue_public = $2
		  AND ($3::bigint IS NULL OR EXISTS (
		      SELECT 1 FROM issues.sprint s
		      WHERE s.id_sprint = $3 AND s.id_project = iss.id_project))
	`, idProject, idIssuePublic, idSprint, idUser)
	if err != nil {
		return false, fmt.Errorf("assigning issue to sprint: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// MoveUnfinished reassigns member issues NOT in a final state from fromSprint to
// toSprint (nil clears to Backlog). Returns the number of issues moved.
func (r *SprintRepository) MoveUnfinished(ctx context.Context, fromSprint int64, toSprint *int64, idsFinal []int64) (int64, error) {
	if idsFinal == nil {
		idsFinal = []int64{} // nil would encode as SQL NULL and silently match nothing
	}
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE issues.issue
		SET id_sprint = $2, carryover_count = carryover_count + 1
		WHERE id_sprint = $1
		  AND (id_state IS NULL OR NOT (id_state = ANY($3)))
	`, fromSprint, toSprint, idsFinal)
	if err != nil {
		return 0, fmt.Errorf("moving unfinished issues: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *SprintRepository) SprintStats(ctx context.Context, filter model.SprintStatsFilter) (*model.SprintStats, error) {
	if filter.IdSprint == nil && filter.IdProject == nil {
		return nil, errs.ErrUnscopedSprintStats
	}
	idsFinal, idsStart := filter.IdsFinal, filter.IdsStart
	if idsFinal == nil {
		idsFinal = []int64{}
	}
	if idsStart == nil {
		idsStart = []int64{}
	}
	args := []any{idsFinal, idsStart}
	where := make([]string, 0, 2)
	if filter.IdSprint != nil {
		args = append(args, *filter.IdSprint)
		where = append(where, fmt.Sprintf("id_sprint = $%d", len(args)))
	} else {
		where = append(where, "id_sprint IS NULL")
	}
	if filter.IdProject != nil {
		args = append(args, *filter.IdProject)
		where = append(where, fmt.Sprintf("id_project = $%d", len(args)))
	}

	stats := &model.SprintStats{}
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		SELECT
			COALESCE(sum(points), 0),
			COALESCE(sum(points) FILTER (WHERE bucket = 'done'), 0),
			COALESCE(sum(points) FILTER (WHERE bucket = 'start'), 0),
			COALESCE(sum(points) FILTER (WHERE bucket = 'progress'), 0),
			count(*),
			count(*) FILTER (WHERE bucket = 'done'),
			count(*) FILTER (WHERE bucket = 'start'),
			count(*) FILTER (WHERE bucket = 'progress'),
			count(*) FILTER (WHERE points IS NOT NULL)
		FROM (
			SELECT
				points,
				CASE
					WHEN id_state = ANY($1) THEN 'done'
					WHEN id_state IS NULL OR id_state = ANY($2) THEN 'start'
					ELSE 'progress'
				END AS bucket
			FROM issues.issue WHERE `+strings.Join(where, " AND ")+`
		) scoped`, args...).Scan(
		&stats.TotalPoints, &stats.DonePoints, &stats.StartPoints, &stats.ProgressPoints,
		&stats.TotalIssues, &stats.DoneIssues, &stats.StartIssues, &stats.ProgressIssues,
		&stats.PointedIssues,
	)
	if err != nil {
		return nil, fmt.Errorf("querying sprint stats: %w", err)
	}
	return stats, nil
}

func (r *SprintRepository) VelocityByProject(ctx context.Context, idProject int64, idsFinal []int64, limit int) ([]*model.SprintVelocity, error) {
	if idsFinal == nil {
		idsFinal = []int64{}
	}
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			recent.id_sprint,
			recent.name,
			recent.end_at,
			done.done_points,
			done.done_issues,
			FALSE AS frozen
		FROM (
			SELECT s.id_sprint, s.name, s.end_at
			FROM issues.sprint s
			WHERE s.id_project = $1 AND s.state = $4
			ORDER BY s.end_at DESC
			LIMIT $3
		) recent
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(sum(i.points), 0)::int AS done_points,
				count(*)::int AS done_issues
			FROM issues.issue i
			WHERE i.id_sprint = recent.id_sprint AND i.id_state = ANY($2)
		) done ON TRUE
		ORDER BY recent.end_at
	`, idProject, idsFinal, limit, constants.SprintStateClosed)
	if err != nil {
		return nil, fmt.Errorf("querying sprint velocity: %w", err)
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.SprintVelocity])
	if err != nil {
		return nil, fmt.Errorf("collecting sprint velocity: %w", err)
	}
	return entries, nil
}
