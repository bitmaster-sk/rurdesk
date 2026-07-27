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
	notifier *notify.Notifier
}

func NewNotificationService(repo *repository.NotificationRepository, notifier *notify.Notifier) *NotificationService {
	return &NotificationService{repo: repo, notifier: notifier}
}

func (s *NotificationService) Notify(ctx context.Context, dto *model.CreateNotificationReq) error {
	notification, err := s.repo.Insert(ctx, dto)
	if err != nil {
		return fmt.Errorf("inserting notification: %w", err)
	}

	s.notifier.Send <- &notify.Notice{
		IdUser:  dto.IdUser,
		Subject: notify.SubjectNotification,
		Action:  notify.ActionCreate,
		Payload: notification,
		Source:  dto.Source,
	}
	return nil
}
