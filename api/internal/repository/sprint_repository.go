package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

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
		WHERE id_project = $1 AND state = 'planned' AND id_sprint <> $2
		ORDER BY start_at LIMIT 1
	`, idProject, excludeSprint)
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
func (r *SprintRepository) MoveUnfinished(ctx context.Context, fromSprint int64, toSprint *int64, finalStateIds []int64) (int64, error) {
	if finalStateIds == nil {
		finalStateIds = []int64{} // nil would encode as SQL NULL and silently match nothing
	}
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE issues.issue
		SET id_sprint = $2, carryover_count = carryover_count + 1
		WHERE id_sprint = $1
		  AND (id_state IS NULL OR NOT (id_state = ANY($3)))
	`, fromSprint, toSprint, finalStateIds)
	if err != nil {
		return 0, fmt.Errorf("moving unfinished issues: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *SprintRepository) Stats(ctx context.Context, idSprint int64, finalStateIds []int64) (*model.SprintStats, error) {
	if finalStateIds == nil {
		finalStateIds = []int64{}
	}
	db := extctx.GetDb(ctx, r.pool)
	stats := &model.SprintStats{IdSprint: idSprint}
	err := db.QueryRow(ctx, `
		SELECT
			COALESCE(sum(points), 0),
			COALESCE(sum(points) FILTER (WHERE id_state = ANY($2)), 0),
			count(*),
			count(*) FILTER (WHERE id_state = ANY($2))
		FROM issues.issue WHERE id_sprint = $1
	`, idSprint, finalStateIds).Scan(&stats.TotalPoints, &stats.DonePoints, &stats.TotalIssues, &stats.DoneIssues)
	if err != nil {
		return nil, fmt.Errorf("querying sprint stats: %w", err)
	}
	return stats, nil
}
