package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueParticipantRepository struct {
	pool *pgxpool.Pool
}

func NewIssueParticipantRepository(pool *pgxpool.Pool) *IssueParticipantRepository {
	return &IssueParticipantRepository{pool: pool}
}

// Add inserts a participant row idempotently: ON CONFLICT DO NOTHING leaves an existing
// (id_issue, id_user) row unchanged, so a re-add never resets has_notifications_enabled.
func (r *IssueParticipantRepository) Add(ctx context.Context, idIssue, idUser int64, source string, addedBy *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.issue_participant (id_issue, id_user, source, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id_issue, id_user) DO NOTHING
	`, idIssue, idUser, source, addedBy)
	if err != nil {
		return fmt.Errorf("adding participant issue=%d user=%d: %w", idIssue, idUser, err)
	}
	return nil
}

// List returns all participants for the given issue, ordered by added_at ascending.
func (r *IssueParticipantRepository) List(ctx context.Context, idIssue int64) ([]*model.IssueParticipant, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT p.id_user, u.name, u.color_avatar_bg, u.is_bot,
		       p.source, p.has_notifications_enabled
		FROM issues.issue_participant p
		JOIN users.user u ON u.id_user = p.id_user
		WHERE p.id_issue = $1
		ORDER BY p.added_at ASC
	`, idIssue)
	if err != nil {
		return nil, fmt.Errorf("listing participants issue=%d: %w", idIssue, err)
	}
	defer rows.Close()
	list, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.IssueParticipant])
	if err != nil {
		return nil, fmt.Errorf("scanning participants issue=%d: %w", idIssue, err)
	}
	return list, nil
}

// NotifiableUserIds returns participant ids who have notifications enabled and aren't
// bots — the recipients for issue-comment notifications.
func (r *IssueParticipantRepository) NotifiableUserIds(ctx context.Context, idIssue int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT p.id_user
		FROM issues.issue_participant p
		JOIN users.user u ON u.id_user = p.id_user
		WHERE p.id_issue = $1
		  AND p.has_notifications_enabled = TRUE
		  AND NOT u.is_bot
	`, idIssue)
	if err != nil {
		return nil, fmt.Errorf("listing notifiable participants issue=%d: %w", idIssue, err)
	}
	defer rows.Close()
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("scanning notifiable ids issue=%d: %w", idIssue, err)
	}
	return ids, nil
}

// SetNotifications sets has_notifications_enabled for (idIssue, idUser); returns
// found=false if no such participant row exists.
func (r *IssueParticipantRepository) SetNotifications(ctx context.Context, idIssue, idUser int64, enabled bool) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE issues.issue_participant
		SET has_notifications_enabled = $3
		WHERE id_issue = $1 AND id_user = $2
	`, idIssue, idUser, enabled)
	if err != nil {
		return false, fmt.Errorf("setting participant notifications issue=%d user=%d: %w", idIssue, idUser, err)
	}
	return tag.RowsAffected() > 0, nil
}
