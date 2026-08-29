package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pinIssueSelectSQL is the shared SELECT … FROM … JOIN fragment used by both
// LoadPinnedIssues and LoadPinnedIssue.  Only the WHERE clause differs.
const pinIssueSelectSQL = `SELECT
			pin.id_pin,
			pin.id_pin_destination_type,
			pin.id_issue,
			pin.id_pin_destination,
			iss.id_issue,
			iss.id_issue_public,
			iss.id_project,
			iss.id_severity,
			iss.title,
			st.name             AS state_name,
			st.start            AS state_is_start,
			st.final            AS state_is_final,
			u.name              AS assigned_to_name,
			u.color_avatar_bg   AS assigned_to_color_avatar_bg
		FROM
			issues.pin
			INNER JOIN issues.issue iss ON pin.id_issue    = iss.id_issue
			LEFT  JOIN issues.state  st ON iss.id_state    = st.id_state
			LEFT  JOIN users.user     u ON iss.assigned_to = u.id_user
`

type PinRepository struct {
	pool *pgxpool.Pool
}

func NewPinRepository(pool *pgxpool.Pool) *PinRepository {
	return &PinRepository{pool: pool}
}

func (r *PinRepository) LoadPinnedIssues(ctx context.Context, idPinDestination int64, idPinDestinationType int) ([]*model.Pin, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, pinIssueSelectSQL+`
		WHERE
			pin.id_pin_destination_type = $1 AND
			pin.id_pin_destination = $2
		ORDER BY iss.title
	`, idPinDestinationType, idPinDestination)
	if err != nil {
		return nil, fmt.Errorf("querying pinned issues: %w", err)
	}
	defer rows.Close()

	var pins []*model.Pin
	for rows.Next() {
		p := &model.Pin{}
		if err := scanPin(rows, p); err != nil {
			return nil, err
		}
		pins = append(pins, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pinned issues: %w", err)
	}
	return pins, nil
}

func (r *PinRepository) LoadPinnedIssue(ctx context.Context, idPin int64) (*model.Pin, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, pinIssueSelectSQL+`
		WHERE
			pin.id_pin = $1
	`, idPin)

	p := &model.Pin{}
	if err := scanPin(row, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ProjectOfIssue returns the project an issue belongs to, so a caller can check
// the requester may read it before pinning it. A missing issue surfaces as a
// wrapped pgx.ErrNoRows, which callers match with errors.Is to answer 404.
func (r *PinRepository) ProjectOfIssue(ctx context.Context, idIssue int64) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	var idProject int64
	if err := db.QueryRow(ctx,
		`SELECT id_project FROM issues.issue WHERE id_issue = $1`, idIssue,
	).Scan(&idProject); err != nil {
		return 0, fmt.Errorf("loading project of issue %d: %w", idIssue, err)
	}
	return idProject, nil
}

func (r *PinRepository) InsertPinnedIssue(ctx context.Context, pin *model.Pin) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.pin (id_issue, id_pin_destination_type, id_pin_destination)
		VALUES ($1, $2, $3)
	`, pin.IdIssue, pin.IdPinDestinationType, pin.IdPinDestination)
	if err != nil {
		return fmt.Errorf("inserting pinned issue: %w", err)
	}
	return nil
}

func (r *PinRepository) DeletePinnedIssue(ctx context.Context, idPin int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.pin WHERE id_pin = $1`, idPin)
	if err != nil {
		return fmt.Errorf("deleting pinned issue: %w", err)
	}
	return nil
}

func (r *PinRepository) LoadPinDestinationType(ctx context.Context, idPinDestinationType int) (*model.PinDestinationType, error) {
	db := extctx.GetDb(ctx, r.pool)
	var pdt model.PinDestinationType
	err := db.QueryRow(ctx, `
		SELECT id_pin_destination_type, code
		FROM issues.pin_destination_type
		WHERE id_pin_destination_type = $1
	`, idPinDestinationType).Scan(&pdt.IdPinDestinationType, &pdt.Code)
	if err != nil {
		return nil, fmt.Errorf("querying pin destination type: %w", err)
	}
	return &pdt, nil
}

func (r *PinRepository) LoadPinDestinationTypes(ctx context.Context) ([]*model.PinDestinationType, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_pin_destination_type, code FROM issues.pin_destination_type
	`)
	if err != nil {
		return nil, fmt.Errorf("querying pin destination types: %w", err)
	}
	defer rows.Close()

	var pdts []*model.PinDestinationType
	for rows.Next() {
		var pdt model.PinDestinationType
		if err := rows.Scan(&pdt.IdPinDestinationType, &pdt.Code); err != nil {
			return nil, fmt.Errorf("scanning pin destination type: %w", err)
		}
		pdts = append(pdts, &pdt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pin destination types: %w", err)
	}
	return pdts, nil
}

// scanPin scans a single pin row (columns ordered as in pinIssueSelectSQL)
// into p and its embedded PinIssueView.  Both LoadPinnedIssues and
// LoadPinnedIssue share this logic so the column list lives in one place.
func scanPin(row pgx.Row, p *model.Pin) error {
	i := &model.PinIssueView{}
	if err := row.Scan(
		&p.IdPin, &p.IdPinDestinationType, &p.IdIssue, &p.IdPinDestination,
		&i.IdIssue, &i.IdIssuePublic, &i.IdProject, &i.IdSeverity,
		&i.Title,
		&i.StateName, &i.StateIsStart, &i.StateIsFinal,
		&i.AssignedToName, &i.AssignedToColorAvatarBg,
	); err != nil {
		return fmt.Errorf("scanning pinned issue: %w", err)
	}
	p.Issue = i
	return nil
}
