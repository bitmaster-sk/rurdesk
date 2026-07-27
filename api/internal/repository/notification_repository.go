package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

func (r *NotificationRepository) Insert(ctx context.Context, dto *model.CreateNotificationReq) (*model.Notification, error) {
	db := extctx.GetDb(ctx, r.pool)

	var (
		bodyRaw   json.RawMessage
		bodyParam *string
	)
	if dto.Body != nil {
		raw, err := json.Marshal(dto.Body)
		if err != nil {
			return nil, fmt.Errorf("marshalling notification body: %w", err)
		}
		bodyRaw = json.RawMessage(raw)
		s := string(raw)
		bodyParam = &s
	}

	notification := &model.Notification{
		IdUser:        dto.IdUser,
		Type:          dto.Type,
		IdProject:     dto.IdProject,
		ProjectName:   dto.ProjectName,
		ProjectColor:  dto.ProjectColor,
		ActorName:     dto.ActorName,
		ActorAvatarBg: dto.ActorAvatarBg,
		RefType:       dto.RefType,
		RefId:         dto.RefId,
		RefTitle:      dto.RefTitle,
		RefPublicId:   dto.RefPublicId,
		Body:          bodyRaw,
	}
	err := db.QueryRow(ctx, `
		INSERT INTO notification.notification
			(id_user, type, id_project, actor_name, actor_avatar_bg,
			 ref_type, ref_id, ref_title, ref_public_id, body)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id_notification, is_read, created_at
	`,
		notification.IdUser,
		notification.Type,
		notification.IdProject,
		notification.ActorName,
		notification.ActorAvatarBg,
		notification.RefType,
		notification.RefId,
		notification.RefTitle,
		notification.RefPublicId,
		bodyParam,
	).Scan(&notification.IdNotification, &notification.IsRead, &notification.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting notification: %w", err)
	}
	return notification, nil
}

func (r *NotificationRepository) GetForUser(ctx context.Context, filter *model.NotificationListFilter) ([]*model.Notification, error) {
	db := extctx.GetDb(ctx, r.pool)

	args := []any{filter.IdUser}
	query := `
		SELECT
			n.id_notification, n.id_user, n.type, n.id_project,
			COALESCE(p.name, '')        AS project_name,
			COALESCE(p.color, '')       AS project_color,
			COALESCE(n.actor_name, '')  AS actor_name,
			COALESCE(n.actor_avatar_bg,'') AS actor_avatar_bg,
			COALESCE(n.ref_type, '')    AS ref_type,
			COALESCE(n.ref_id, '')      AS ref_id,
			COALESCE(n.ref_title, '')   AS ref_title,
			n.ref_public_id,
			n.body,
			n.is_read,
			n.created_at
		FROM notification.notification n
		LEFT JOIN projects.project p ON p.id_project = n.id_project
		WHERE n.id_user = $1
	`
	idx := 2

	if filter.IdProject != nil {
		args = append(args, *filter.IdProject)
		query += fmt.Sprintf(` AND n.id_project = $%d`, idx)
		idx++
	}
	if filter.OnlyUnread {
		query += ` AND n.is_read = FALSE`
	}

	query += ` ORDER BY n.created_at DESC`

	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(` LIMIT $%d`, idx)
		idx++
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(` OFFSET $%d`, idx)
	}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying notifications: %w", err)
	}
	notifications, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[model.Notification])
	if err != nil {
		return nil, fmt.Errorf("collecting notifications: %w", err)
	}
	return notifications, nil
}

func (r *NotificationRepository) GetUnreadCount(ctx context.Context, idUser int64) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int64
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notification.notification WHERE id_user = $1 AND is_read = FALSE`,
		idUser,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting unread notifications: %w", err)
	}
	return count, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, idNotification, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE notification.notification SET is_read = TRUE WHERE id_notification = $1 AND id_user = $2`,
		idNotification, idUser,
	)
	if err != nil {
		return fmt.Errorf("marking notification read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, idUser int64, idProject *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	if idProject != nil {
		_, err := db.Exec(ctx,
			`UPDATE notification.notification SET is_read = TRUE WHERE id_user = $1 AND id_project = $2`,
			idUser, *idProject,
		)
		if err != nil {
			return fmt.Errorf("marking project notifications read: %w", err)
		}
		return nil
	}
	_, err := db.Exec(ctx,
		`UPDATE notification.notification SET is_read = TRUE WHERE id_user = $1`,
		idUser,
	)
	if err != nil {
		return fmt.Errorf("marking all notifications read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) Delete(ctx context.Context, idNotification, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`DELETE FROM notification.notification WHERE id_notification = $1 AND id_user = $2`,
		idNotification, idUser,
	)
	if err != nil {
		return fmt.Errorf("deleting notification: %w", err)
	}
	return nil
}
