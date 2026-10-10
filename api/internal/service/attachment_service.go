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

	"github.com/bitmaster-sk/rurdesk/api/internal/attachmenttext"
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
	RecipientType model.MessageRecipientType
	IdRecipient   int64
	FileName      string
	Size          int64
	Content       io.Reader
}

type MessageContextChecker interface {
	CheckContextAccess(ctx context.Context, idUser int64, recipientType model.MessageRecipientType, idRecipient int64) error
}

type AttachmentStorer interface {
	Insert(ctx context.Context, attachment *model.Attachment) error
	LoadById(ctx context.Context, idAttachment uuid.UUID) (*model.Attachment, error)
	LoadByIds(ctx context.Context, idsAttachment []uuid.UUID) ([]*model.Attachment, error)
	ReplaceMessageLinks(ctx context.Context, idMessage int64, idsAttachment []uuid.UUID) error
}

type AttachmentService struct {
	pool           *pgxpool.Pool
	contextChecker MessageContextChecker
	repo           AttachmentStorer
	storage        repository.AttachmentStorage
	sizeLimiter    AttachmentSizeLimiter
}

func NewAttachmentService(
	pool *pgxpool.Pool,
	contextChecker MessageContextChecker,
	repo AttachmentStorer,
	storage repository.AttachmentStorage,
	sizeLimiter AttachmentSizeLimiter,
) *AttachmentService {
	return &AttachmentService{pool: pool, contextChecker: contextChecker, repo: repo, storage: storage, sizeLimiter: sizeLimiter}
}

func (s *AttachmentService) MaxSizeBytes() int64 {
	return int64(s.sizeLimiter.AttachmentMaxSizeMb()) << 20
}

func (s *AttachmentService) Upload(ctx context.Context, idUser int64, upload AttachmentUpload) (*model.Attachment, error) {
	scope, ok := model.AttachmentScopeOf(upload.RecipientType, upload.IdRecipient)
	if !ok {
		return nil, errs.ErrBadRequest.WithMessage("unknown idMessageRecipientType")
	}
	if upload.Size > s.MaxSizeBytes() {
		return nil, errs.ErrAttachmentTooLarge
	}
	if err := s.checkScopeAccess(ctx, idUser, scope); err != nil {
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
		AttachmentScope: scope,
		CreateBy:        &idUser,
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

func (s *AttachmentService) Open(ctx context.Context, idUser int64, idAttachment uuid.UUID) (*model.Attachment, io.ReadCloser, error) {
	attachment, err := s.repo.LoadById(ctx, idAttachment)
	if errors.Is(err, repository.ErrAttachmentNotFound) {
		return nil, nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkReadAccess(ctx, idUser, attachment); err != nil {
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

func (s *AttachmentService) LinkMessageAttachments(ctx context.Context, msg *model.Message) error {
	scope, ok := model.AttachmentScopeOf(msg.IdMessageRecipientType, msg.IdRecipient)
	if !ok {
		return errs.ErrBadRequest
	}
	idsAttachment := attachmenttext.ParseIds(msg.Message)
	if len(idsAttachment) > 0 {
		attachments, err := s.repo.LoadByIds(ctx, idsAttachment)
		if err != nil {
			return fmt.Errorf("loading linked attachments of message %d: %w", msg.IdMessage, err)
		}
		if len(attachments) != len(idsAttachment) {
			return errs.ErrAttachmentLinkInvalid
		}
		for _, attachment := range attachments {
			if !canLinkAttachment(attachment, msg.Creator.IdUser, scope) {
				return errs.ErrAttachmentLinkInvalid
			}
		}
	}
	if err := s.repo.ReplaceMessageLinks(ctx, msg.IdMessage, idsAttachment); err != nil {
		return fmt.Errorf("replacing attachment links of message %d: %w", msg.IdMessage, err)
	}
	return nil
}

func canLinkAttachment(attachment *model.Attachment, idAuthor int64, scope model.AttachmentScope) bool {
	if !attachment.HasScope(scope) {
		return false
	}
	if scope.IdUserTo != nil {
		return attachment.CreateBy != nil && *attachment.CreateBy == idAuthor
	}
	return true
}

func (s *AttachmentService) checkScopeAccess(ctx context.Context, idUser int64, scope model.AttachmentScope) error {
	recipientType, idRecipient, ok := scope.Recipient()
	if !ok {
		return errs.ErrBadRequest
	}
	return s.contextChecker.CheckContextAccess(ctx, idUser, recipientType, idRecipient)
}

func (s *AttachmentService) checkReadAccess(ctx context.Context, idUser int64, attachment *model.Attachment) error {
	if attachment.CreateBy != nil && *attachment.CreateBy == idUser {
		return nil
	}
	if attachment.IdUserTo != nil {
		if *attachment.IdUserTo == idUser {
			return nil
		}
		return errs.ErrForbidden
	}
	return s.checkScopeAccess(ctx, idUser, attachment.AttachmentScope)
}
