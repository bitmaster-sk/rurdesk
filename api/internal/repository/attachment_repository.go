package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentRepository struct {
	pool *pgxpool.Pool
}

func NewAttachmentRepository(pool *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{pool: pool}
}

func (r *AttachmentRepository) Insert(ctx context.Context, attachment *model.Attachment) error {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO files.attachment (file_name, mime_type, size, id_issue, id_project, id_team, id_user_to, create_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id_attachment, create_at
	`, attachment.FileName, attachment.MimeType, attachment.Size,
		attachment.IdIssue, attachment.IdProject, attachment.IdTeam, attachment.IdUserTo, attachment.CreateBy,
	).Scan(&attachment.IdAttachment, &attachment.CreateAt)
	if err != nil {
		return fmt.Errorf("inserting attachment: %w", err)
	}
	return nil
}

func (r *AttachmentRepository) LoadById(ctx context.Context, idAttachment uuid.UUID) (*model.Attachment, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_attachment, file_name, mime_type, size, id_issue, id_project, id_team, id_user_to, create_by, create_at
		FROM files.attachment
		WHERE id_attachment = $1
	`, idAttachment)
	if err != nil {
		return nil, fmt.Errorf("querying attachment %s: %w", idAttachment, err)
	}
	attachment, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[model.Attachment])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAttachmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting attachment %s: %w", idAttachment, err)
	}
	return attachment, nil
}

func (r *AttachmentRepository) LoadByIds(ctx context.Context, idsAttachment []uuid.UUID) ([]*model.Attachment, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_attachment, file_name, mime_type, size, id_issue, id_project, id_team, id_user_to, create_by, create_at
		FROM files.attachment
		WHERE id_attachment = ANY($1)
	`, idsAttachment)
	if err != nil {
		return nil, fmt.Errorf("querying attachments: %w", err)
	}
	attachments, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Attachment])
	if err != nil {
		return nil, fmt.Errorf("collecting attachments: %w", err)
	}
	return attachments, nil
}

func (r *AttachmentRepository) ReplaceMessageLinks(ctx context.Context, idMessage int64, idsAttachment []uuid.UUID) error {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `DELETE FROM messages.message_attachment WHERE id_message = $1`, idMessage); err != nil {
		return fmt.Errorf("deleting attachment links of message %d: %w", idMessage, err)
	}
	if len(idsAttachment) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		INSERT INTO messages.message_attachment (id_message, id_attachment)
		SELECT $1, unnest($2::uuid[])
	`, idMessage, idsAttachment)
	if err != nil {
		return fmt.Errorf("inserting attachment links of message %d: %w", idMessage, err)
	}
	return nil
}

func (r *AttachmentRepository) DeleteOrphansOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		DELETE FROM files.attachment attachment
		WHERE attachment.create_at < $1
		  AND NOT EXISTS (
			SELECT 1 FROM messages.message_attachment link
			WHERE link.id_attachment = attachment.id_attachment
		  )
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting orphan attachments: %w", err)
	}
	return tag.RowsAffected(), nil
}
