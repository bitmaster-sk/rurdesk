package service

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

type NotificationService struct {
	repo     *repository.NotificationRepository
	userRepo *repository.UserRepository
	notifier notify.INotifier
}

func NewNotificationService(repo *repository.NotificationRepository, userRepo *repository.UserRepository, notifier notify.INotifier) *NotificationService {
	return &NotificationService{repo: repo, userRepo: userRepo, notifier: notifier}
}

func (s *NotificationService) Notify(ctx context.Context, dto *model.CreateNotificationReq) error {
	active, err := s.userRepo.IsActiveUser(ctx, dto.IdUser)
	if err != nil {
		return fmt.Errorf("checking notification recipient: %w", err)
	}
	if !active {
		return nil
	}
	notification, err := s.repo.Insert(ctx, dto)
	if err != nil {
		return fmt.Errorf("inserting notification: %w", err)
	}

	s.notifier.Send(&notify.Notice{
		IdUser:  dto.IdUser,
		Subject: notify.SubjectNotification,
		Action:  notify.ActionCreate,
		Payload: notification,
		Source:  dto.Source,
	})
	return nil
}
