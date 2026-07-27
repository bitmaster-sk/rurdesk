package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TrackerRepository struct {
	pool *pgxpool.Pool
}

func NewTrackerRepository(pool *pgxpool.Pool) *TrackerRepository {
	return &TrackerRepository{pool: pool}
}

func (r *TrackerRepository) LoadTracker(ctx context.Context, idUser int64) (*model.Tracker, error) {
	db := extctx.GetDb(ctx, r.pool)
	tra := &model.Tracker{}
	err := db.QueryRow(ctx, `
		SELECT
			tra.id_tracker,
			tra.id_user,
			tra.id_issue,
			tra.start_at,
			iss.id_project,
			iss.id_issue_public
		FROM
			issues.tracker tra
			INNER JOIN issues.issue iss ON tra.id_issue = iss.id_issue
		WHERE
			tra.id_user = $1
	`, idUser).Scan(&tra.IdTracker, &tra.IdUser, &tra.IdIssue, &tra.StartAt, &tra.IdProject, &tra.IdIssuePublic)
	if err != nil {
		return nil, fmt.Errorf("querying tracker: %w", err)
	}
	return tra, nil
}

func (r *TrackerRepository) InsertTracker(ctx context.Context, tracker *model.Tracker) (*model.Tracker, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.tracker(id_user, id_issue)
		VALUES($1, $2) RETURNING id_tracker, start_at
	`, tracker.IdUser, tracker.IdIssue).Scan(&tracker.IdTracker, &tracker.StartAt)
	if err != nil {
		return nil, fmt.Errorf("inserting tracker: %w", err)
	}
	return tracker, nil
}

func (r *TrackerRepository) DeleteTracker(ctx context.Context, idTracker int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.tracker WHERE id_tracker = $1`, idTracker)
	if err != nil {
		return fmt.Errorf("deleting tracker: %w", err)
	}
	return nil
}

func (r *TrackerRepository) LoadTracks(ctx context.Context, filter model.TracksFilter) ([]*model.Track, error) {
	db := extctx.GetDb(ctx, r.pool)

	var (
		sb   strings.Builder
		args []any
		idx  = 1
	)

	sb.WriteString(`
		SELECT t.id_track, t.id_user, t.id_issue, iss.id_issue_public, iss.id_project,
		       iss.title AS issue_title, t.tracked, t.start_at, t.end_at
		FROM issues.track t
		INNER JOIN issues.issue iss ON iss.id_issue = t.id_issue
		WHERE 1=1
	`)

	if filter.IdIssue != nil {
		fmt.Fprintf(&sb, " AND iss.id_issue = $%d", idx)
		args = append(args, *filter.IdIssue)
		idx++
	}
	if len(filter.IdsProject) > 0 {
		fmt.Fprintf(&sb, " AND iss.id_project = ANY($%d)", idx)
		args = append(args, filter.IdsProject)
		idx++
	}
	if filter.IdUser != nil {
		fmt.Fprintf(&sb, " AND t.id_user = $%d", idx)
		args = append(args, *filter.IdUser)
		idx++
	}
	if filter.StartFrom != nil {
		fmt.Fprintf(&sb, " AND t.start_at >= $%d", idx)
		args = append(args, *filter.StartFrom)
		idx++
	}
	if filter.StartTo != nil {
		fmt.Fprintf(&sb, " AND t.start_at <= $%d", idx)
		args = append(args, *filter.StartTo)
		idx++
	}
	_ = idx

	rows, err := db.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("querying tracks: %w", err)
	}
	tracks, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Track])
	if err != nil {
		return nil, fmt.Errorf("collecting tracks: %w", err)
	}
	return tracks, nil
}

func (r *TrackerRepository) LoadTrack(ctx context.Context, idTrack int64) (*model.Track, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT t.id_track, t.id_user, t.id_issue, iss.id_issue_public, iss.id_project,
		       iss.title AS issue_title, t.tracked, t.start_at, t.end_at
		FROM issues.track t
		INNER JOIN issues.issue iss ON iss.id_issue = t.id_issue
		WHERE t.id_track = $1
	`, idTrack)
	if err != nil {
		return nil, fmt.Errorf("querying track: %w", err)
	}
	track, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Track])
	if err != nil {
		return nil, fmt.Errorf("collecting track: %w", err)
	}
	return track, nil
}

func (r *TrackerRepository) InsertTrack(ctx context.Context, track *model.Track) (*model.Track, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.track(id_user, id_issue, tracked, start_at, end_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id_track
	`, track.IdUser, track.IdIssue, track.Tracked, track.StartAt, track.EndAt).Scan(&track.IdTrack)
	if err != nil {
		return nil, fmt.Errorf("inserting track: %w", err)
	}
	return track, nil
}

func (r *TrackerRepository) UpdateTrack(ctx context.Context, track *model.Track) (*model.Track, error) {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.track SET id_user=$1, id_issue=$2, tracked=$3, start_at=$4, end_at=$5 WHERE id_track=$6
	`, track.IdUser, track.IdIssue, track.Tracked, track.StartAt, track.EndAt, track.IdTrack)
	if err != nil {
		return nil, fmt.Errorf("updating track: %w", err)
	}
	return track, nil
}

func (r *TrackerRepository) DeleteTrack(ctx context.Context, idTrack int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.track WHERE id_track = $1`, idTrack)
	if err != nil {
		return fmt.Errorf("deleting track: %w", err)
	}
	return nil
}

func (r *TrackerRepository) UpdateIssueTracked(ctx context.Context, idIssue int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue SET tracked = (SELECT COALESCE(SUM(tracked), 0) FROM issues.track WHERE id_issue = $1) WHERE id_issue = $1
	`, idIssue)
	if err != nil {
		return fmt.Errorf("updating issue tracked: %w", err)
	}
	return nil
}
