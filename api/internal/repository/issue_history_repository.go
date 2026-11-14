package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type IssueHistoryRepository struct {
	pool *pgxpool.Pool
}

func NewIssueHistoryRepository(pool *pgxpool.Pool) *IssueHistoryRepository {
	return &IssueHistoryRepository{pool: pool}
}

func (r *IssueHistoryRepository) InsertUserChanges(ctx context.Context, idIssue int64, idUser int64, idChange uuid.UUID, changes []model.IssueHistoryChange) error {
	if len(changes) == 0 {
		return nil
	}
	db := extctx.GetDb(ctx, r.pool)
	for _, change := range changes {
		oldValue, newValue, err := marshalHistoryValues(change)
		if err != nil {
			return err
		}
		_, err = db.Exec(ctx, `
			INSERT INTO issues.issue_history (id_issue, id_change, field, old_value, new_value, id_user, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, idIssue, idChange, change.Field, oldValue, newValue, idUser, model.IssueHistorySourceUser)
		if err != nil {
			return fmt.Errorf("inserting %s history of issue %d: %w", change.Field, idIssue, err)
		}
	}
	return nil
}

func (r *IssueHistoryRepository) UpsertSystemChange(ctx context.Context, idIssue int64, idChange uuid.UUID, change model.IssueHistoryChange) error {
	oldValue, newValue, err := marshalHistoryValues(change)
	if err != nil {
		return err
	}
	db := extctx.GetDb(ctx, r.pool)
	var idIssueHistory int64
	var isMergeable bool
	err = db.QueryRow(ctx, `
		SELECT id_issue_history,
		       source = $3 AND create_at > (now() at time zone 'utc') - interval '15 minutes'
		FROM issues.issue_history
		WHERE id_issue = $1 AND field = $2
		ORDER BY id_issue_history DESC
		LIMIT 1
		FOR UPDATE
	`, idIssue, change.Field, model.IssueHistorySourceSystem).Scan(&idIssueHistory, &isMergeable)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("loading last %s history of issue %d: %w", change.Field, idIssue, err)
	}
	if err == nil && isMergeable {
		return r.mergeSystemChange(ctx, idIssueHistory, newValue)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO issues.issue_history (id_issue, id_change, field, old_value, new_value, id_user, source)
		VALUES ($1, $2, $3, $4, $5, NULL, $6)
	`, idIssue, idChange, change.Field, oldValue, newValue, model.IssueHistorySourceSystem)
	if err != nil {
		return fmt.Errorf("inserting %s history of issue %d: %w", change.Field, idIssue, err)
	}
	return nil
}

func (r *IssueHistoryRepository) mergeSystemChange(ctx context.Context, idIssueHistory int64, newValue []byte) error {
	db := extctx.GetDb(ctx, r.pool)
	var isNoop bool
	err := db.QueryRow(ctx, `
		UPDATE issues.issue_history
		SET new_value = $2, create_at = now() at time zone 'utc'
		WHERE id_issue_history = $1
		RETURNING old_value IS NOT DISTINCT FROM new_value
	`, idIssueHistory, newValue).Scan(&isNoop)
	if err != nil {
		return fmt.Errorf("merging issue history %d: %w", idIssueHistory, err)
	}
	if !isNoop {
		return nil
	}
	if _, err := db.Exec(ctx, `DELETE FROM issues.issue_history WHERE id_issue_history = $1`, idIssueHistory); err != nil {
		return fmt.Errorf("deleting no-op issue history %d: %w", idIssueHistory, err)
	}
	return nil
}

func (r *IssueHistoryRepository) LoadByIssue(ctx context.Context, idIssue int64) ([]model.IssueHistory, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_issue_history, id_issue, id_change, field, old_value, new_value, id_user, source, create_at
		FROM issues.issue_history
		WHERE id_issue = $1
		ORDER BY create_at, id_issue_history
	`, idIssue)
	if err != nil {
		return nil, fmt.Errorf("loading history of issue %d: %w", idIssue, err)
	}
	history, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.IssueHistory])
	if err != nil {
		return nil, fmt.Errorf("scanning history of issue %d: %w", idIssue, err)
	}
	return history, nil
}

func marshalHistoryValues(change model.IssueHistoryChange) (oldValue, newValue []byte, err error) {
	oldValue, err = json.Marshal(change.OldValue)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding old %s value: %w", change.Field, err)
	}
	newValue, err = json.Marshal(change.NewValue)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding new %s value: %w", change.Field, err)
	}
	return oldValue, newValue, nil
}
