package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type ContextReadChecker interface {
	CanReadTeam(ctx context.Context, idUser, idTeam int64) bool
	CanReadProject(ctx context.Context, idUser, idProject int64) bool
}

type IssueProjectLoader interface {
	LoadProjectByIssue(ctx context.Context, idIssue int64) (*model.Project, error)
}

type UserLoader interface {
	LoadUser(ctx context.Context, idUser int64) (*model.User, error)
}

type MessageAccessService struct {
	acl           ContextReadChecker
	projectLoader IssueProjectLoader
	userLoader    UserLoader
}

func NewMessageAccessService(acl ContextReadChecker, projectLoader IssueProjectLoader, userLoader UserLoader) *MessageAccessService {
	return &MessageAccessService{acl: acl, projectLoader: projectLoader, userLoader: userLoader}
}

func (s *MessageAccessService) CheckContextAccess(
	ctx context.Context,
	idUser int64,
	recipientType model.MessageRecipientType,
	idRecipient int64,
) error {
	switch recipientType {
	case model.TeammateRecipientType:
		// DMs are open to all users — only require that the recipient exists.
		if _, err := s.userLoader.LoadUser(ctx, idRecipient); err != nil {
			return errs.ErrForbidden
		}
		return nil

	case model.TeamRecipientType:
		if !s.acl.CanReadTeam(ctx, idUser, idRecipient) {
			return errs.ErrForbidden
		}
		return nil

	case model.ProjectRecipientType:
		if !s.acl.CanReadProject(ctx, idUser, idRecipient) {
			return errs.ErrForbidden
		}
		return nil

	case model.IssueRecipientType:
		project, err := s.projectLoader.LoadProjectByIssue(ctx, idRecipient)
		if err != nil {
			return fmt.Errorf("loading project of issue %d: %w", idRecipient, err)
		}
		if !s.acl.CanReadProject(ctx, idUser, project.IdProject) {
			return errs.ErrForbidden
		}
		return nil

	default:
		return errs.ErrBadRequest
	}
}
