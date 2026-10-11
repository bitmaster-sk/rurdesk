package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	attachmentSniffBytes       = 512
	attachmentFileNameMaxRunes = 255
	attachmentFallbackFileName = "file"
)

var allowedAttachmentTypes = map[string]bool{
	"image/png":                    true,
	"image/jpeg":                   true,
	"image/gif":                    true,
	"image/webp":                   true,
	"image/bmp":                    true,
	"application/pdf":              true,
	"text/plain; charset=utf-8":    true,
	"text/plain; charset=utf-16be": true,
	"text/plain; charset=utf-16le": true,
	"application/zip":              true,
	"application/x-gzip":           true,
	"application/x-rar-compressed": true,
}

func detectAttachmentType(head []byte) (string, error) {
	if bytes.Contains(bytes.ToLower(head), []byte("<svg")) {
		return "", errs.ErrAttachmentTypeNotAllowed
	}
	mimeType := http.DetectContentType(head)
	if !allowedAttachmentTypes[mimeType] {
		return "", errs.ErrAttachmentTypeNotAllowed
	}
	return mimeType, nil
}

type AttachmentSizeLimiter interface {
	AttachmentMaxSizeMb() int
}

type AttachmentUpload struct {
	Scope    model.AttachmentScope
	FileName string
	Size     int64
	Content  io.Reader
}

type MessageContextChecker interface {
	CheckContextAccess(ctx context.Context, idUser int64, recipientType model.MessageRecipientType, idRecipient int64) error
}

type WikiSpaceAccessChecker interface {
	CheckSpaceRead(ctx context.Context, user model.User, idSpace int64) error
	CheckSpaceEdit(ctx context.Context, user model.User, idSpace int64) error
}

type AttachmentStorer interface {
	Insert(ctx context.Context, attachment *model.Attachment) error
	LoadById(ctx context.Context, idAttachment uuid.UUID) (*model.Attachment, error)
}

type AttachmentService struct {
	pool           *pgxpool.Pool
	contextChecker MessageContextChecker
	spaceChecker   WikiSpaceAccessChecker
	repo           AttachmentStorer
	storage        repository.AttachmentStorage
	sizeLimiter    AttachmentSizeLimiter
}

func NewAttachmentService(
	pool *pgxpool.Pool,
	contextChecker MessageContextChecker,
	spaceChecker WikiSpaceAccessChecker,
	repo AttachmentStorer,
	storage repository.AttachmentStorage,
	sizeLimiter AttachmentSizeLimiter,
) *AttachmentService {
	return &AttachmentService{
		pool:           pool,
		contextChecker: contextChecker,
		spaceChecker:   spaceChecker,
		repo:           repo,
		storage:        storage,
		sizeLimiter:    sizeLimiter,
	}
}

func (s *AttachmentService) MaxSizeBytes() int64 {
	return int64(s.sizeLimiter.AttachmentMaxSizeMb()) << 20
}

func (s *AttachmentService) Upload(ctx context.Context, user model.User, upload AttachmentUpload) (*model.Attachment, error) {
	if upload.Size > s.MaxSizeBytes() {
		return nil, errs.ErrAttachmentTooLarge
	}
	if err := s.checkUploadAccess(ctx, user, upload.Scope); err != nil {
		return nil, err
	}

	head := make([]byte, attachmentSniffBytes)
	headLength, err := io.ReadFull(upload.Content, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading attachment head: %w", err)
	}
	head = head[:headLength]
	mimeType, err := detectAttachmentType(head)
	if err != nil {
		return nil, err
	}

	attachment := &model.Attachment{
		FileName:        sanitizeAttachmentFileName(upload.FileName),
		MimeType:        mimeType,
		Size:            upload.Size,
		AttachmentScope: upload.Scope,
		CreateBy:        &user.IdUser,
	}
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		if err := s.repo.Insert(ctx, attachment); err != nil {
			return err
		}
		return s.storage.Put(ctx, attachment.IdAttachment, io.MultiReader(bytes.NewReader(head), upload.Content))
	})
	if err != nil {
		return nil, err
	}
	return attachment, nil
}

func sanitizeAttachmentFileName(raw string) string {
	name := strings.TrimSpace(path.Base(strings.ReplaceAll(raw, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return attachmentFallbackFileName
	}
	runes := []rune(name)
	if len(runes) > attachmentFileNameMaxRunes {
		return string(runes[:attachmentFileNameMaxRunes])
	}
	return name
}

func (s *AttachmentService) Open(ctx context.Context, user model.User, idAttachment uuid.UUID) (*model.Attachment, io.ReadCloser, error) {
	attachment, err := s.repo.LoadById(ctx, idAttachment)
	if errors.Is(err, repository.ErrAttachmentNotFound) {
		return nil, nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkReadAccess(ctx, user, attachment); err != nil {
		return nil, nil, err
	}
	content, err := s.storage.Get(ctx, idAttachment)
	if errors.Is(err, repository.ErrAttachmentNotFound) {
		return nil, nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return attachment, content, nil
}

func (s *AttachmentService) checkUploadAccess(ctx context.Context, user model.User, scope model.AttachmentScope) error {
	if scope.IdSpace != nil {
		return s.spaceChecker.CheckSpaceEdit(ctx, user, *scope.IdSpace)
	}
	return s.checkContextAccess(ctx, user.IdUser, scope)
}

func (s *AttachmentService) checkReadAccess(ctx context.Context, user model.User, attachment *model.Attachment) error {
	if attachment.CreateBy != nil && *attachment.CreateBy == user.IdUser {
		return nil
	}
	if attachment.IdUserTo != nil {
		if *attachment.IdUserTo == user.IdUser {
			return nil
		}
		return errs.ErrForbidden
	}
	if attachment.IdSpace != nil {
		return s.spaceChecker.CheckSpaceRead(ctx, user, *attachment.IdSpace)
	}
	return s.checkContextAccess(ctx, user.IdUser, attachment.AttachmentScope)
}

func (s *AttachmentService) checkContextAccess(ctx context.Context, idUser int64, scope model.AttachmentScope) error {
	recipientType, idRecipient, ok := scope.Recipient()
	if !ok {
		return errs.ErrBadRequest
	}
	return s.contextChecker.CheckContextAccess(ctx, idUser, recipientType, idRecipient)
}
