package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAnchorWrongThread = errors.New("anchor parent message belongs to a different thread")

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

func (r *MessageRepository) LoadTeammateMessages(ctx context.Context, idRecipient int64, idReader int64, read *bool) ([]*model.Message, error) {
	db := extctx.GetDb(ctx, r.pool)

	var (
		query string
		args  []any
	)

	if read != nil && !*read {
		query = `
			SELECT
				msg.id_message,
				msg.message,
				msg.created_at,
				msg.updated_at,
				msg.version,
				usr.id_user,
				usr.name,
				usr.color_avatar_bg,
				CASE WHEN msg.created_at > COALESCE(ure.read_at, '1900-1-1') AND msg.id_user_from != $1 THEN FALSE ELSE TRUE END AS read,
				umg.id_user_to
			FROM
				messages.message msg
				INNER JOIN messages.user_message umg ON umg.id_message = msg.id_message
				INNER JOIN users.user usr ON msg.id_user_from = usr.id_user
				LEFT JOIN messages.user_read ure ON msg.id_user_from = ure.id_user_from AND ure.id_user = $1
			WHERE
				umg.id_user_to = $1
				AND msg.created_at > COALESCE(ure.read_at, '1900-1-1')
			ORDER BY msg.created_at DESC
		`
		args = []any{idReader}
	} else {
		query = `
			SELECT
				msg.id_message,
				msg.message,
				msg.created_at,
				msg.updated_at,
				msg.version,
				usr.id_user,
				usr.name,
				usr.color_avatar_bg,
				CASE WHEN msg.created_at > COALESCE(ure.read_at, '1900-1-1') AND msg.id_user_from != $2 THEN FALSE ELSE TRUE END AS read,
				umg.id_user_to
			FROM
				messages.message msg
				INNER JOIN messages.user_message umg ON umg.id_message = msg.id_message
				INNER JOIN users.user usr ON msg.id_user_from = usr.id_user
				LEFT JOIN messages.user_read ure ON msg.id_user_from = ure.id_user_from AND ure.id_user = $2
			WHERE
				(umg.id_user_to = $1 AND msg.id_user_from = $2) OR
				(umg.id_user_to = $2 AND msg.id_user_from = $1)
			ORDER BY msg.created_at DESC
		`
		args = []any{idRecipient, idReader}
	}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying teammate messages: %w", err)
	}
	return scanMessages(rows, model.TeammateRecipientType)
}

func (r *MessageRepository) LoadTeamMessages(ctx context.Context, idsTeam []int64, idReader int64, read *bool) ([]*model.Message, error) {
	db := extctx.GetDb(ctx, r.pool)

	readCond := ""
	if read != nil && !*read {
		readCond = "AND msg.created_at > COALESCE(tre.read_at, '1900-1-1') AND msg.id_user_from != $2"
	}

	query := fmt.Sprintf(`
		SELECT
			msg.id_message,
			msg.message,
			msg.created_at,
			msg.updated_at,
			msg.version,
			usr.id_user,
			usr.name,
			usr.color_avatar_bg,
			CASE WHEN msg.created_at > COALESCE(tre.read_at, '1900-1-1') AND msg.id_user_from != $2 THEN FALSE ELSE TRUE END AS read,
			tmm.id_team_to
		FROM
			messages.message msg
			INNER JOIN messages.team_message tmm ON tmm.id_message = msg.id_message
			INNER JOIN users.user usr ON msg.id_user_from = usr.id_user
			LEFT JOIN messages.team_read tre ON tre.id_team_to = tmm.id_team_to AND tre.id_user = $2
		WHERE
			tmm.id_team_to = ANY($1)
			%s
		ORDER BY msg.created_at DESC
	`, readCond)

	rows, err := db.Query(ctx, query, idsTeam, idReader)
	if err != nil {
		return nil, fmt.Errorf("querying team messages: %w", err)
	}
	return scanMessages(rows, model.TeamRecipientType)
}

func (r *MessageRepository) LoadProjectMessages(ctx context.Context, idsProject []int64, idReader int64, read *bool) ([]*model.Message, error) {
	db := extctx.GetDb(ctx, r.pool)

	readCond := ""
	if read != nil && !*read {
		readCond = "AND msg.created_at > COALESCE(pre.read_at, '1900-1-1') AND msg.id_user_from != $2"
	}

	query := fmt.Sprintf(`
		SELECT
			msg.id_message,
			msg.message,
			msg.created_at,
			msg.updated_at,
			msg.version,
			usr.id_user,
			usr.name,
			usr.color_avatar_bg,
			CASE WHEN msg.created_at > COALESCE(pre.read_at, '1900-1-1') AND msg.id_user_from != $2 THEN FALSE ELSE TRUE END AS read,
			prm.id_project_to
		FROM
			messages.message msg
			INNER JOIN messages.project_message prm ON prm.id_message = msg.id_message
			INNER JOIN users.user usr ON msg.id_user_from = usr.id_user
			LEFT JOIN messages.project_read pre ON pre.id_project_to = prm.id_project_to AND pre.id_user = $2
		WHERE
			prm.id_project_to = ANY($1)
			%s
		ORDER BY msg.created_at DESC
	`, readCond)

	rows, err := db.Query(ctx, query, idsProject, idReader)
	if err != nil {
		return nil, fmt.Errorf("querying project messages: %w", err)
	}
	return scanMessages(rows, model.ProjectRecipientType)
}

func (r *MessageRepository) LoadIssueMessages(ctx context.Context, idIssue int64, idReader int64, read *bool) ([]*model.Message, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			msg.id_message,
			msg.message,
			msg.message_kind,
			msg.created_at,
			msg.updated_at,
			msg.version,
			usr.id_user,
			usr.name,
			usr.color_avatar_bg,
			true AS read,
			ism.id_issue_to,
			a.id_parent_message,
			a.parent_version,
			a.anchor_line_start,
			a.anchor_line_end,
			parent.version AS parent_current_version
		FROM messages.message msg
		INNER JOIN messages.issue_message ism ON ism.id_message = msg.id_message
		INNER JOIN users.user usr ON msg.id_user_from = usr.id_user
		LEFT JOIN messages.message_anchor a ON a.id_message = msg.id_message
		LEFT JOIN messages.message parent ON parent.id_message = a.id_parent_message
		WHERE ism.id_issue_to = $1
		ORDER BY msg.created_at DESC
	`, idIssue)
	if err != nil {
		return nil, fmt.Errorf("querying issue messages: %w", err)
	}
	return scanIssueMessages(rows)
}

func (r *MessageRepository) InsertTeammateMessage(ctx context.Context, message string, creator *model.User, idRecipient int64) (*model.Message, error) {
	msg, err := r.insertMessage(ctx, message, creator, constants.MessageKindComment)
	if err != nil {
		return nil, err
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err = db.Exec(ctx, `INSERT INTO messages.user_message(id_message, id_user_to) VALUES ($1, $2)`, msg.IdMessage, idRecipient)
	if err != nil {
		return nil, fmt.Errorf("inserting user message: %w", err)
	}
	msg.IdRecipient = idRecipient
	msg.IdMessageRecipientType = model.TeammateRecipientType
	return msg, nil
}

func (r *MessageRepository) InsertTeamMessage(ctx context.Context, message string, creator *model.User, idRecipient int64) (*model.Message, error) {
	msg, err := r.insertMessage(ctx, message, creator, constants.MessageKindComment)
	if err != nil {
		return nil, err
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err = db.Exec(ctx, `INSERT INTO messages.team_message(id_message, id_team_to) VALUES ($1, $2)`, msg.IdMessage, idRecipient)
	if err != nil {
		return nil, fmt.Errorf("inserting team message: %w", err)
	}
	msg.IdRecipient = idRecipient
	msg.IdMessageRecipientType = model.TeamRecipientType
	return msg, nil
}

func (r *MessageRepository) InsertProjectMessage(ctx context.Context, message string, creator *model.User, idRecipient int64) (*model.Message, error) {
	msg, err := r.insertMessage(ctx, message, creator, constants.MessageKindComment)
	if err != nil {
		return nil, err
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err = db.Exec(ctx, `INSERT INTO messages.project_message(id_message, id_project_to) VALUES ($1, $2)`, msg.IdMessage, idRecipient)
	if err != nil {
		return nil, fmt.Errorf("inserting project message: %w", err)
	}
	msg.IdRecipient = idRecipient
	msg.IdMessageRecipientType = model.ProjectRecipientType
	return msg, nil
}

// InsertIssueAgentMessage inserts a non-anchored issue message authored by an agent,
// tagged with the given MessageKind (Plan or Clarification).
func (r *MessageRepository) InsertIssueAgentMessage(
	ctx context.Context,
	message string,
	creator *model.User,
	idIssue int64,
	kind constants.MessageKind,
) (*model.Message, error) {
	msg, err := r.insertMessage(ctx, message, creator, kind)
	if err != nil {
		return nil, err
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err = db.Exec(ctx, `INSERT INTO messages.issue_message(id_message, id_issue_to) VALUES ($1, $2)`, msg.IdMessage, idIssue)
	if err != nil {
		return nil, fmt.Errorf("inserting issue message: %w", err)
	}
	msg.IdRecipient = idIssue
	msg.IdMessageRecipientType = model.IssueRecipientType
	msg.IsRead = true
	return msg, nil
}

func (r *MessageRepository) InsertIssueMessage(ctx context.Context, message string, creator *model.User, idRecipient int64, anchor *model.MessageAnchor) (*model.Message, error) {
	msg, err := r.insertMessage(ctx, message, creator, constants.MessageKindComment)
	if err != nil {
		return nil, err
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err = db.Exec(ctx, `INSERT INTO messages.issue_message(id_message, id_issue_to) VALUES ($1, $2)`, msg.IdMessage, idRecipient)
	if err != nil {
		return nil, fmt.Errorf("inserting issue message: %w", err)
	}

	if anchor != nil {
		var parentIssueID int64
		guardErr := db.QueryRow(ctx,
			`SELECT ism.id_issue_to FROM messages.issue_message ism WHERE ism.id_message = $1`,
			anchor.IdParentMessage,
		).Scan(&parentIssueID)
		if errors.Is(guardErr, pgx.ErrNoRows) || parentIssueID != idRecipient {
			return nil, ErrAnchorWrongThread
		}
		if guardErr != nil {
			return nil, fmt.Errorf("checking anchor parent thread: %w", guardErr)
		}

		var capturedParentVersion int
		anchorErr := db.QueryRow(ctx, `
			INSERT INTO messages.message_anchor (id_message, id_parent_message, parent_version, anchor_line_start, anchor_line_end)
			VALUES ($1, $2, (SELECT version FROM messages.message WHERE id_message = $2), $3, $4)
			RETURNING parent_version
		`, msg.IdMessage, anchor.IdParentMessage, anchor.AnchorLineStart, anchor.AnchorLineEnd).Scan(&capturedParentVersion)
		if anchorErr != nil {
			return nil, fmt.Errorf("inserting message anchor: %w", anchorErr)
		}

		msg.Anchor = &model.MessageAnchor{
			IdParentMessage: anchor.IdParentMessage,
			ParentVersion:   capturedParentVersion,
			AnchorLineStart: anchor.AnchorLineStart,
			AnchorLineEnd:   anchor.AnchorLineEnd,
			IsOutdated:      false,
		}
	}

	msg.IdRecipient = idRecipient
	msg.IdMessageRecipientType = model.IssueRecipientType
	msg.IsRead = true
	return msg, nil
}

func (r *MessageRepository) insertMessage(ctx context.Context, message string, creator *model.User, kind constants.MessageKind) (*model.Message, error) {
	db := extctx.GetDb(ctx, r.pool)
	msg := &model.Message{Message: message, Creator: creator, MessageKind: kind}
	err := db.QueryRow(ctx,
		`INSERT INTO messages.message(message, id_user_from, message_kind) VALUES ($1, $2, $3) RETURNING id_message, created_at, version`,
		message, creator.IdUser, string(kind),
	).Scan(&msg.IdMessage, &msg.CreatedAt, &msg.Version)
	if err != nil {
		return nil, fmt.Errorf("inserting message: %w", err)
	}
	return msg, nil
}

func (r *MessageRepository) UpdateMessage(ctx context.Context, idMessage int64, idUserFrom int64, newMessage string) (*model.Message, error) {
	db := extctx.GetDb(ctx, r.pool)
	msg := &model.Message{}
	creator := &model.User{}
	err := db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE messages.message
			SET message = $3, updated_at = now() at time zone 'utc', version = version + 1
			WHERE id_message = $1 AND id_user_from = $2
			RETURNING id_message, message, created_at, updated_at, id_user_from, version
		)
		SELECT u.id_message, u.message, u.created_at, u.updated_at, u.version,
		       usr.id_user, usr.name, usr.color_avatar_bg
		FROM updated u
		INNER JOIN users.user usr ON usr.id_user = u.id_user_from
	`, idMessage, idUserFrom, newMessage).Scan(
		&msg.IdMessage, &msg.Message, &msg.CreatedAt, &msg.UpdatedAt, &msg.Version,
		&creator.IdUser, &creator.Name, &creator.ColorAvatarBg,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("updating message %d: %w", idMessage, err)
	}
	msg.Creator = creator
	return msg, nil
}

func (r *MessageRepository) LoadMessageRecipientInfo(ctx context.Context, idMessage int64) (int64, model.MessageRecipientType, error) {
	db := extctx.GetDb(ctx, r.pool)
	var idRecipient int64
	var recipientType int64
	err := db.QueryRow(ctx, `
		SELECT id_recipient, id_recipient_type FROM (
			SELECT id_user_to    AS id_recipient, 1 AS id_recipient_type FROM messages.user_message    WHERE id_message = $1
			UNION ALL
			SELECT id_team_to    AS id_recipient, 2 AS id_recipient_type FROM messages.team_message    WHERE id_message = $1
			UNION ALL
			SELECT id_project_to AS id_recipient, 3 AS id_recipient_type FROM messages.project_message WHERE id_message = $1
			UNION ALL
			SELECT id_issue_to   AS id_recipient, 4 AS id_recipient_type FROM messages.issue_message   WHERE id_message = $1
		) t LIMIT 1
	`, idMessage).Scan(&idRecipient, &recipientType)
	if err != nil {
		return 0, 0, fmt.Errorf("loading recipient info for message %d: %w", idMessage, err)
	}
	return idRecipient, model.MessageRecipientType(recipientType), nil
}

func (r *MessageRepository) InsertReadTeammatesMessages(ctx context.Context, idCreator int64, idReader int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO messages.user_read(id_user, id_user_from)
		VALUES ($1, $2)
		ON CONFLICT (id_user, id_user_from) DO UPDATE SET read_at = now() at time zone 'utc'
	`, idReader, idCreator)
	if err != nil {
		return fmt.Errorf("marking teammate messages read: %w", err)
	}
	return nil
}

func (r *MessageRepository) InsertReadTeamMessages(ctx context.Context, idRecipient int64, idReader int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO messages.team_read(id_team_to, id_user)
		VALUES ($1, $2)
		ON CONFLICT (id_team_to, id_user) DO UPDATE SET read_at = now() at time zone 'utc'
	`, idRecipient, idReader)
	if err != nil {
		return fmt.Errorf("marking team messages read: %w", err)
	}
	return nil
}

func (r *MessageRepository) InsertReadProjectMessages(ctx context.Context, idRecipient int64, idReader int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO messages.project_read(id_project_to, id_user)
		VALUES ($1, $2)
		ON CONFLICT (id_project_to, id_user) DO UPDATE SET read_at = now() at time zone 'utc'
	`, idRecipient, idReader)
	if err != nil {
		return fmt.Errorf("marking project messages read: %w", err)
	}
	return nil
}

func scanMessages(rows pgx.Rows, recipientType model.MessageRecipientType) ([]*model.Message, error) {
	defer rows.Close()
	var msgs []*model.Message
	for rows.Next() {
		m := &model.Message{}
		u := &model.User{}
		if err := rows.Scan(&m.IdMessage, &m.Message, &m.CreatedAt, &m.UpdatedAt, &m.Version, &u.IdUser, &u.Name, &u.ColorAvatarBg, &m.IsRead, &m.IdRecipient); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		m.Creator = u
		m.IdMessageRecipientType = recipientType
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}
	return msgs, nil
}

func scanIssueMessages(rows pgx.Rows) ([]*model.Message, error) {
	defer rows.Close()
	var msgs []*model.Message
	for rows.Next() {
		m := &model.Message{}
		u := &model.User{}
		var (
			idParentMessage  *int64
			parentVersion    *int
			anchorLineStart  *int
			anchorLineEnd    *int
			parentCurrentVer *int
			kind             string
		)
		if err := rows.Scan(
			&m.IdMessage, &m.Message, &kind, &m.CreatedAt, &m.UpdatedAt, &m.Version,
			&u.IdUser, &u.Name, &u.ColorAvatarBg,
			&m.IsRead, &m.IdRecipient,
			&idParentMessage, &parentVersion, &anchorLineStart, &anchorLineEnd, &parentCurrentVer,
		); err != nil {
			return nil, fmt.Errorf("scanning issue message: %w", err)
		}
		m.MessageKind = constants.MessageKind(kind)
		m.Creator = u
		m.IdMessageRecipientType = model.IssueRecipientType
		if idParentMessage != nil {
			isOutdated := parentCurrentVer != nil && *parentVersion < *parentCurrentVer
			m.Anchor = &model.MessageAnchor{
				IdParentMessage: *idParentMessage,
				ParentVersion:   *parentVersion,
				AnchorLineStart: *anchorLineStart,
				AnchorLineEnd:   *anchorLineEnd,
				IsOutdated:      isOutdated,
			}
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating issue messages: %w", err)
	}
	return msgs, nil
}
