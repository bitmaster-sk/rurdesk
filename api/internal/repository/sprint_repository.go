package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
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

const statsAggregate = `
	SELECT
		COALESCE(sum(points), 0) AS total_points,
		COALESCE(sum(points) FILTER (WHERE bucket = 'done'), 0) AS done_points,
		COALESCE(sum(points) FILTER (WHERE bucket = 'start'), 0) AS start_points,
		COALESCE(sum(points) FILTER (WHERE bucket = 'progress'), 0) AS progress_points,
		count(*) AS total_issues,
		count(*) FILTER (WHERE bucket = 'done') AS done_issues,
		count(*) FILTER (WHERE bucket = 'start') AS start_issues,
		count(*) FILTER (WHERE bucket = 'progress') AS progress_issues,
		count(*) FILTER (WHERE points IS NOT NULL) AS pointed_issues
	FROM (
		SELECT
			points,
			CASE
				WHEN id_state = ANY($1) THEN 'done'
				WHEN id_state IS NULL OR id_state = ANY($2) THEN 'start'
				ELSE 'progress'
			END AS bucket
		FROM issues.issue WHERE %s
	) scoped`

func sanitizeStateIds(idsFinal, idsStart []int64) ([]int64, []int64) {
	if idsFinal == nil {
		idsFinal = []int64{}
	}
	if idsStart == nil {
		idsStart = []int64{}
	}
	return idsFinal, idsStart
}

func bucketTargets(stats *model.SprintStats) []any {
	return []any{
		&stats.TotalPoints, &stats.DonePoints, &stats.StartPoints, &stats.ProgressPoints,
		&stats.TotalIssues, &stats.DoneIssues, &stats.StartIssues, &stats.ProgressIssues,
		&stats.PointedIssues,
	}
}

func (r *SprintRepository) SprintStats(ctx context.Context, idSprint int64, idsFinal, idsStart []int64) (*model.SprintStats, error) {
	idsFinal, idsStart = sanitizeStateIds(idsFinal, idsStart)
	stats := &model.SprintStats{}
	targets := append(bucketTargets(stats),
		&stats.FrozenTotalPoints, &stats.FrozenDonePoints, &stats.FrozenTotalIssues,
		&stats.FrozenDoneIssues, &stats.FrozenPointedIssues)

	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		SELECT agg.total_points, agg.done_points, agg.start_points, agg.progress_points,
		       agg.total_issues, agg.done_issues, agg.start_issues, agg.progress_issues,
		       agg.pointed_issues,
		       frozen.total_points, frozen.done_points, frozen.total_issues,
		       frozen.done_issues, frozen.pointed_issues
		FROM (`+fmt.Sprintf(statsAggregate, "id_sprint = $3")+`) agg
		LEFT JOIN LATERAL (
			SELECT sn.total_points, sn.done_points, sn.total_issues, sn.done_issues, sn.pointed_issues
			FROM issues.sprint s
			JOIN issues.sprint_snapshot sn ON sn.id_sprint = s.id_sprint
			WHERE s.id_sprint = $3 AND s.state = $4
			ORDER BY sn.day DESC
			LIMIT 1
		) frozen ON TRUE
	`, idsFinal, idsStart, idSprint, constants.SprintStateClosed).Scan(targets...)
	if err != nil {
		return nil, fmt.Errorf("querying sprint stats: %w", err)
	}
	if stats.FrozenTotalIssues != nil && stats.FrozenDoneIssues != nil {
		rolled := *stats.FrozenTotalIssues - *stats.FrozenDoneIssues
		stats.RolledOverIssues = &rolled
	}
	return stats, nil
}

func (r *SprintRepository) BacklogStats(ctx context.Context, idProject int64, idsFinal, idsStart []int64) (*model.SprintStats, error) {
	idsFinal, idsStart = sanitizeStateIds(idsFinal, idsStart)
	stats := &model.SprintStats{}

	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx,
		fmt.Sprintf(statsAggregate, "id_sprint IS NULL AND id_project = $3"),
		idsFinal, idsStart, idProject).Scan(bucketTargets(stats)...)
	if err != nil {
		return nil, fmt.Errorf("querying backlog stats: %w", err)
	}
	return stats, nil
}

func (r *SprintRepository) UpsertSnapshotToday(ctx context.Context, idSprint int64, idsFinal []int64) error {
	if idsFinal == nil {
		idsFinal = []int64{}
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.sprint_snapshot(
			id_sprint, day, total_points, done_points, total_issues, done_issues, pointed_issues)
		SELECT
			s.id_sprint,
			(now() at time zone 'utc')::date,
			COALESCE(sum(i.points), 0)::int,
			COALESCE(sum(i.points) FILTER (WHERE i.id_state = ANY($2)), 0)::int,
			count(i.id_issue)::int,
			count(i.id_issue) FILTER (WHERE i.id_state = ANY($2))::int,
			count(i.points)::int
		FROM issues.sprint s
		LEFT JOIN issues.issue i ON i.id_sprint = s.id_sprint
		WHERE s.id_sprint = $1
		  AND s.state <> $3
		  AND (now() at time zone 'utc')::date >= s.start_at::date
		GROUP BY s.id_sprint
		ON CONFLICT (id_sprint, day) DO UPDATE SET
			total_points   = EXCLUDED.total_points,
			done_points    = EXCLUDED.done_points,
			total_issues   = EXCLUDED.total_issues,
			done_issues    = EXCLUDED.done_issues,
			pointed_issues = EXCLUDED.pointed_issues
		WHERE (
			issues.sprint_snapshot.total_points,
			issues.sprint_snapshot.done_points,
			issues.sprint_snapshot.total_issues,
			issues.sprint_snapshot.done_issues,
			issues.sprint_snapshot.pointed_issues
		) IS DISTINCT FROM (
			EXCLUDED.total_points,
			EXCLUDED.done_points,
			EXCLUDED.total_issues,
			EXCLUDED.done_issues,
			EXCLUDED.pointed_issues
		)
	`, idSprint, idsFinal, constants.SprintStateClosed)
	if err != nil {
		return fmt.Errorf("upserting sprint snapshot: %w", err)
	}
	return nil
}

func (r *SprintRepository) UpsertSnapshotsForOpenSprints(ctx context.Context) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		INSERT INTO issues.sprint_snapshot(
			id_sprint, day, total_points, done_points, total_issues, done_issues, pointed_issues)
		SELECT
			s.id_sprint,
			(now() at time zone 'utc')::date,
			COALESCE(sum(i.points), 0)::int,
			COALESCE(sum(i.points) FILTER (WHERE st.final), 0)::int,
			count(i.id_issue)::int,
			count(i.id_issue) FILTER (WHERE st.final)::int,
			count(i.points)::int
		FROM issues.sprint s
		LEFT JOIN issues.issue i ON i.id_sprint = s.id_sprint
		LEFT JOIN issues.state st ON st.id_state = i.id_state
		WHERE s.state <> $1
		  AND (now() at time zone 'utc')::date >= s.start_at::date
		GROUP BY s.id_sprint
		ON CONFLICT (id_sprint, day) DO UPDATE SET
			total_points   = EXCLUDED.total_points,
			done_points    = EXCLUDED.done_points,
			total_issues   = EXCLUDED.total_issues,
			done_issues    = EXCLUDED.done_issues,
			pointed_issues = EXCLUDED.pointed_issues
		WHERE (
			issues.sprint_snapshot.total_points,
			issues.sprint_snapshot.done_points,
			issues.sprint_snapshot.total_issues,
			issues.sprint_snapshot.done_issues,
			issues.sprint_snapshot.pointed_issues
		) IS DISTINCT FROM (
			EXCLUDED.total_points,
			EXCLUDED.done_points,
			EXCLUDED.total_issues,
			EXCLUDED.done_issues,
			EXCLUDED.pointed_issues
		)
	`, constants.SprintStateClosed)
	if err != nil {
		return 0, fmt.Errorf("upserting open sprint snapshots: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *SprintRepository) LoadSnapshots(ctx context.Context, idSprint int64) ([]*model.SprintSnapshot, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_sprint, day, total_points, done_points, total_issues, done_issues, pointed_issues
		FROM issues.sprint_snapshot WHERE id_sprint = $1 ORDER BY day
	`, idSprint)
	if err != nil {
		return nil, fmt.Errorf("querying sprint snapshots: %w", err)
	}
	snapshots, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.SprintSnapshot])
	if err != nil {
		return nil, fmt.Errorf("collecting sprint snapshots: %w", err)
	}
	return snapshots, nil
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
			COALESCE(last.done_points, live.done_points) AS done_points,
			COALESCE(last.done_issues, live.done_issues) AS done_issues,
			CASE WHEN base.day <> last.day THEN base.total_points END AS planned_points,
			CASE WHEN base.day <> last.day THEN base.total_issues END AS planned_issues,
			(last.day IS NOT NULL) AS frozen
		FROM (
			SELECT s.id_sprint, s.name, s.start_at, s.end_at
			FROM issues.sprint s
			WHERE s.id_project = $1 AND s.state = $4
			ORDER BY s.end_at DESC
			LIMIT $3
		) recent
		LEFT JOIN LATERAL (
			SELECT sn.day, sn.done_points, sn.done_issues
			FROM issues.sprint_snapshot sn
			WHERE sn.id_sprint = recent.id_sprint
			ORDER BY sn.day DESC
			LIMIT 1
		) last ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(sum(i.points), 0)::int AS done_points,
				count(*)::int AS done_issues
			FROM issues.issue i
			WHERE last.day IS NULL
			  AND i.id_sprint = recent.id_sprint
			  AND i.id_state = ANY($2)
		) live ON TRUE
		LEFT JOIN LATERAL (
			SELECT sn.day, sn.total_points, sn.total_issues
			FROM issues.sprint_snapshot sn
			WHERE sn.id_sprint = recent.id_sprint
			ORDER BY (sn.day >= recent.start_at::date) DESC,
			         CASE WHEN sn.day >= recent.start_at::date THEN sn.day END ASC NULLS LAST,
			         sn.day DESC
			LIMIT 1
		) base ON TRUE
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
