package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type IssueHistoryService struct {
	repo     *repository.IssueHistoryRepository
	userRepo *repository.UserRepository
}

func NewIssueHistoryService(repo *repository.IssueHistoryRepository, userRepo *repository.UserRepository) *IssueHistoryService {
	return &IssueHistoryService{repo: repo, userRepo: userRepo}
}

func (s *IssueHistoryService) RecordUserEdit(ctx context.Context, idUser int64, before, after *model.Issue) error {
	return s.repo.InsertUserChanges(ctx, after.IdIssue, idUser, uuid.New(), model.DiffIssue(before, after))
}

func (s *IssueHistoryService) RecordTagEdits(ctx context.Context, idUser int64, before, after map[int64][]int64) error {
	for idIssue, idsTag := range after {
		if err := s.repo.InsertUserChanges(ctx, idIssue, idUser, uuid.New(), model.DiffTags(before[idIssue], idsTag)); err != nil {
			return err
		}
	}
	return nil
}

func (s *IssueHistoryService) RecordSystem(ctx context.Context, idIssue int64, changes []model.IssueHistoryChange) error {
	if len(changes) == 0 {
		return nil
	}
	idChange := uuid.New()
	for _, change := range changes {
		if err := s.repo.UpsertSystemChange(ctx, idIssue, idChange, change); err != nil {
			return err
		}
	}
	return nil
}

func (s *IssueHistoryService) List(ctx context.Context, idIssue int64) ([]model.IssueHistory, []*model.User, error) {
	history, err := s.repo.LoadByIssue(ctx, idIssue)
	if err != nil {
		return nil, nil, err
	}
	users, err := s.userRepo.LoadUsersByIds(ctx, model.IssueHistoryUserIds(history))
	if err != nil {
		return nil, nil, err
	}
	return history, users, nil
}
