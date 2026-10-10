package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentStorage interface {
	Put(ctx context.Context, idAttachment uuid.UUID, content io.Reader) error
	Get(ctx context.Context, idAttachment uuid.UUID) (io.ReadCloser, error)
}

type PostgresAttachmentStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresAttachmentStorage(pool *pgxpool.Pool) *PostgresAttachmentStorage {
	return &PostgresAttachmentStorage{pool: pool}
}

func (s *PostgresAttachmentStorage) Put(ctx context.Context, idAttachment uuid.UUID, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("reading attachment %s content: %w", idAttachment, err)
	}
	db := extctx.GetDb(ctx, s.pool)
	_, err = db.Exec(ctx, `
		INSERT INTO files.attachment_content (id_attachment, data) VALUES ($1, $2)
	`, idAttachment, data)
	if err != nil {
		return fmt.Errorf("storing attachment %s content: %w", idAttachment, err)
	}
	return nil
}

func (s *PostgresAttachmentStorage) Get(ctx context.Context, idAttachment uuid.UUID) (io.ReadCloser, error) {
	db := extctx.GetDb(ctx, s.pool)
	var data []byte
	err := db.QueryRow(ctx, `
		SELECT data FROM files.attachment_content WHERE id_attachment = $1
	`, idAttachment).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAttachmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading attachment %s content: %w", idAttachment, err)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
