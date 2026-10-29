package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	releaseNameUniqueIndex  = "release_project_name_uidx"
	releaseDraftUniqueIndex = "release_draft_tag_uidx"
)

type ReleaseRepository struct {
	pool *pgxpool.Pool
}

func NewReleaseRepository(pool *pgxpool.Pool) *ReleaseRepository {
	return &ReleaseRepository{pool: pool}
}

const releaseColumns = `
	rel.id_release, rel.id_project, rel.id_tag, rel.name, rel.status, rel.notes,
	rel.recreate_tag, rel.open_issues, rel.issue_count,
	rel.create_at, rel.create_by, rel.released_at, rel.released_by`

func (r *ReleaseRepository) LoadReleases(ctx context.Context, idProject int64) ([]*model.ReleaseView, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT `+releaseColumns+`,
			tag.name AS tag_name,
			run.id_run AS id_run_notes,
			run.phase AS notes_run_phase,
			run.error_message AS notes_run_error,
			COALESCE(run.phase <> ALL($2), false) AS is_notes_pending
		FROM issues.release rel
		LEFT JOIN issues.tag tag ON tag.id_tag = rel.id_tag
		LEFT JOIN LATERAL (
			SELECT id_run, phase, error_message FROM agent.run
			WHERE id_release = rel.id_release
			ORDER BY created_at DESC, id_run DESC
			LIMIT 1
		) run ON true
		WHERE rel.id_project = $1
		ORDER BY rel.status = 'draft' DESC, COALESCE(rel.released_at, rel.create_at) DESC, rel.id_release DESC
	`, idProject,
		[]string{constants.PhaseDone, constants.PhaseFailed, constants.PhaseCancelled},
	)
	if err != nil {
		return nil, fmt.Errorf("querying releases: %w", err)
	}
	releases, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.ReleaseView])
	if err != nil {
		return nil, fmt.Errorf("collecting releases: %w", err)
	}
	return releases, nil
}

func (r *ReleaseRepository) LoadRelease(ctx context.Context, idProject, idRelease int64) (*model.ReleaseView, error) {
	releases, err := r.LoadReleases(ctx, idProject)
	if err != nil {
		return nil, err
	}
	for _, release := range releases {
		if release.IdRelease == idRelease {
			return release, nil
		}
	}
	return nil, errs.ErrNotFound
}

func (r *ReleaseRepository) LockRelease(ctx context.Context, idProject, idRelease int64) (*model.Release, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT `+releaseColumns+` FROM issues.release rel
		WHERE rel.id_project = $1 AND rel.id_release = $2
		FOR UPDATE
	`, idProject, idRelease)
	if err != nil {
		return nil, fmt.Errorf("locking release: %w", err)
	}
	release, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Release])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting release: %w", err)
	}
	return release, nil
}

func (r *ReleaseRepository) LoadById(ctx context.Context, idRelease int64) (*model.Release, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT `+releaseColumns+` FROM issues.release rel WHERE rel.id_release = $1
	`, idRelease)
	if err != nil {
		return nil, fmt.Errorf("querying release: %w", err)
	}
	release, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Release])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting release: %w", err)
	}
	return release, nil
}

func (r *ReleaseRepository) IsNameTaken(ctx context.Context, idProject, idReleaseExcluded int64, name string) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var taken bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM issues.release
			WHERE id_project = $1 AND lower(name) = lower($2) AND id_release <> $3
		)
	`, idProject, name, idReleaseExcluded).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("checking release name: %w", err)
	}
	return taken, nil
}

func (r *ReleaseRepository) InsertDraft(ctx context.Context, release *model.Release) (*model.Release, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.release (id_project, id_tag, name, status, create_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id_release, status, recreate_tag, open_issues, create_at
	`, release.IdProject, release.IdTag, release.Name, constants.ReleaseStatusDraft, release.CreateBy,
	).Scan(&release.IdRelease, &release.Status, &release.RecreateTag, &release.OpenIssues, &release.CreateAt)
	if err != nil {
		return nil, mapReleaseWriteErr("inserting release", err)
	}
	return release, nil
}

func (r *ReleaseRepository) UpdateDraft(ctx context.Context, release *model.Release) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.release SET name = $2, recreate_tag = $3, open_issues = $4, notes = $5
		WHERE id_release = $1
	`, release.IdRelease, release.Name, release.RecreateTag, release.OpenIssues, release.Notes)
	if err != nil {
		return mapReleaseWriteErr("updating release", err)
	}
	return nil
}

func (r *ReleaseRepository) MarkReleased(ctx context.Context, release *model.Release) error {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		UPDATE issues.release SET
			status = $2, issue_count = $3,
			released_at = (now() AT TIME ZONE 'utc'), released_by = $4
		WHERE id_release = $1
		RETURNING released_at
	`, release.IdRelease, constants.ReleaseStatusReleased, release.IssueCount, release.ReleasedBy,
	).Scan(&release.ReleasedAt)
	if err != nil {
		return fmt.Errorf("marking release as released: %w", err)
	}
	release.Status = constants.ReleaseStatusReleased
	return nil
}

func (r *ReleaseRepository) SetDraftNotes(ctx context.Context, idRelease int64, notes string) (*model.Release, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		UPDATE issues.release rel SET notes = $2
		WHERE rel.id_release = $1 AND rel.status = $3
		RETURNING `+releaseColumns, idRelease, notes, constants.ReleaseStatusDraft)
	if err != nil {
		return nil, fmt.Errorf("writing release notes: %w", err)
	}
	release, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Release])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting release: %w", err)
	}
	return release, nil
}

func mapReleaseWriteErr(action string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case releaseNameUniqueIndex:
			return errs.ErrReleaseNameTaken
		case releaseDraftUniqueIndex:
			return errs.ErrReleaseDraftExists
		}
	}
	return fmt.Errorf("%s: %w", action, err)
}

func (r *ReleaseRepository) UpdateNotes(ctx context.Context, idRelease int64, notes string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `UPDATE issues.release SET notes = $2 WHERE id_release = $1`, idRelease, notes)
	if err != nil {
		return fmt.Errorf("updating release notes: %w", err)
	}
	return nil
}

func (r *ReleaseRepository) DeleteRelease(ctx context.Context, idRelease int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.release WHERE id_release = $1`, idRelease)
	if err != nil {
		return fmt.Errorf("deleting release: %w", err)
	}
	return nil
}

func (r *ReleaseRepository) LoadTagIssues(ctx context.Context, idProject, idTag int64) ([]*model.ReleaseIssue, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			iss.id_issue,
			iss.id_issue_public,
			iss.title,
			iss.id_issue_type,
			typ.name AS issue_type_name,
			typ.order_rank AS issue_type_order,
			COALESCE(sta.final, false) AS is_closed
		FROM issues.issue iss
		JOIN issues.issue_tag itg ON itg.id_issue = iss.id_issue AND itg.id_tag = $2
		LEFT JOIN issues.state sta ON sta.id_state = iss.id_state
		LEFT JOIN issues.issue_type typ ON typ.id_issue_type = iss.id_issue_type
		WHERE iss.id_project = $1
		ORDER BY typ.order_rank NULLS LAST, iss.id_issue_public
	`, idProject, idTag)
	if err != nil {
		return nil, fmt.Errorf("querying release issues: %w", err)
	}
	issues, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.ReleaseIssue])
	if err != nil {
		return nil, fmt.Errorf("collecting release issues: %w", err)
	}
	return issues, nil
}

func (r *ReleaseRepository) LoadTagIssueIds(ctx context.Context, idTag int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT id_issue FROM issues.issue_tag WHERE id_tag = $1`, idTag)
	if err != nil {
		return nil, fmt.Errorf("querying tag issues: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting tag issues: %w", err)
	}
	return ids, nil
}
