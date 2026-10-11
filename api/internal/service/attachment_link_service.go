package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/attachmenttext"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/google/uuid"
)

type AttachmentLinkStorer interface {
	LoadByIds(ctx context.Context, idsAttachment []uuid.UUID) ([]*model.Attachment, error)
	ReplaceMessageLinks(ctx context.Context, idMessage int64, idsAttachment []uuid.UUID) error
	AddWikiLinks(ctx context.Context, idPage int64, idsAttachment []uuid.UUID) error
	ReplaceIssueLinks(ctx context.Context, idIssue int64, idsAttachment []uuid.UUID) error
}

type AttachmentLinkService struct {
	repo AttachmentLinkStorer
}

func NewAttachmentLinkService(repo AttachmentLinkStorer) *AttachmentLinkService {
	return &AttachmentLinkService{repo: repo}
}

func (s *AttachmentLinkService) LinkMessageAttachments(ctx context.Context, msg *model.Message) error {
	scope, ok := model.AttachmentScopeOf(msg.IdMessageRecipientType, msg.IdRecipient)
	if !ok {
		return errs.ErrBadRequest
	}
	idsAttachment, err := s.loadLinkable(ctx, msg.Message, func(attachment *model.Attachment) bool {
		return canLinkAttachment(attachment, msg.Creator.IdUser, scope)
	})
	if err != nil {
		return err
	}
	if err := s.repo.ReplaceMessageLinks(ctx, msg.IdMessage, idsAttachment); err != nil {
		return fmt.Errorf("replacing attachment links of message %d: %w", msg.IdMessage, err)
	}
	return nil
}

func (s *AttachmentLinkService) LinkWikiAttachments(ctx context.Context, idSpace, idPage int64, body string) error {
	scope := model.AttachmentScope{IdSpace: &idSpace}
	idsAttachment, err := s.loadLinkable(ctx, body, func(attachment *model.Attachment) bool {
		return attachment.HasScope(scope)
	})
	if err != nil {
		return err
	}
	return s.addWikiLinks(ctx, idPage, idsAttachment)
}

func (s *AttachmentLinkService) LinkWikiDraftAttachments(ctx context.Context, idSpace, idPage int64, body string) error {
	scope := model.AttachmentScope{IdSpace: &idSpace}
	idsAttachment := attachmenttext.ParseIds(body)
	attachments, err := s.loadByIds(ctx, idsAttachment)
	if err != nil {
		return err
	}
	idsLinked := make([]uuid.UUID, 0, len(idsAttachment))
	for _, idAttachment := range idsAttachment {
		if attachment, ok := attachments[idAttachment]; ok && attachment.HasScope(scope) {
			idsLinked = append(idsLinked, idAttachment)
		}
	}
	return s.addWikiLinks(ctx, idPage, idsLinked)
}

func (s *AttachmentLinkService) addWikiLinks(ctx context.Context, idPage int64, idsAttachment []uuid.UUID) error {
	if err := s.repo.AddWikiLinks(ctx, idPage, idsAttachment); err != nil {
		return fmt.Errorf("adding attachment links of wiki page %d: %w", idPage, err)
	}
	return nil
}

func (s *AttachmentLinkService) LinkIssueAttachments(ctx context.Context, issue *model.Issue, previousDescription string) error {
	issueScope := model.AttachmentScope{IdIssue: &issue.IdIssue}
	projectScope := model.AttachmentScope{IdProject: &issue.IdProject}
	idsAttachment := attachmenttext.ParseIds(issue.Description)
	attachments, err := s.loadByIds(ctx, idsAttachment)
	if err != nil {
		return err
	}
	idsPrevious := map[uuid.UUID]bool{}
	for _, idAttachment := range attachmenttext.ParseIds(previousDescription) {
		idsPrevious[idAttachment] = true
	}
	idsLinked := make([]uuid.UUID, 0, len(idsAttachment))
	for _, idAttachment := range idsAttachment {
		attachment, ok := attachments[idAttachment]
		if ok && (attachment.HasScope(issueScope) || attachment.HasScope(projectScope)) {
			idsLinked = append(idsLinked, idAttachment)
			continue
		}
		if !idsPrevious[idAttachment] {
			return errs.ErrAttachmentLinkInvalid
		}
	}
	if err := s.repo.ReplaceIssueLinks(ctx, issue.IdIssue, idsLinked); err != nil {
		return fmt.Errorf("replacing attachment links of issue %d: %w", issue.IdIssue, err)
	}
	return nil
}

func (s *AttachmentLinkService) loadByIds(ctx context.Context, idsAttachment []uuid.UUID) (map[uuid.UUID]*model.Attachment, error) {
	byId := make(map[uuid.UUID]*model.Attachment, len(idsAttachment))
	if len(idsAttachment) == 0 {
		return byId, nil
	}
	attachments, err := s.repo.LoadByIds(ctx, idsAttachment)
	if err != nil {
		return nil, fmt.Errorf("loading linked attachments: %w", err)
	}
	for _, attachment := range attachments {
		byId[attachment.IdAttachment] = attachment
	}
	return byId, nil
}

func (s *AttachmentLinkService) loadLinkable(ctx context.Context, text string, canLink func(*model.Attachment) bool) ([]uuid.UUID, error) {
	idsAttachment := attachmenttext.ParseIds(text)
	if len(idsAttachment) == 0 {
		return nil, nil
	}
	attachments, err := s.repo.LoadByIds(ctx, idsAttachment)
	if err != nil {
		return nil, fmt.Errorf("loading linked attachments: %w", err)
	}
	if len(attachments) != len(idsAttachment) {
		return nil, errs.ErrAttachmentLinkInvalid
	}
	for _, attachment := range attachments {
		if !canLink(attachment) {
			return nil, errs.ErrAttachmentLinkInvalid
		}
	}
	return idsAttachment, nil
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
