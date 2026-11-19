package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const agentConversationColumns = `cnv.id_conversation, cnv.id_user, cnv.id_user_agent, cnv.id_project,
	coalesce((SELECT array_agg(cpr.id_project ORDER BY cpr.id_project)
	          FROM agent.conversation_project cpr
	          WHERE cpr.id_conversation = cnv.id_conversation), '{}') AS ids_project_context,
	cnv.id_run, cnv.title, cnv.head_commit, cnv.is_locked, cnv.task_draft_request, cnv.task_draft_message_id, cnv.read_at, cnv.pinned_at, cnv.model, cnv.last_agent_message_at,
	cnv.created_at, cnv.updated_at`

// conversationMessageQuery reads the messages a query names `msg` in a WITH clause. A turn has
// thinking to replay when it kept live rows or a compacted blob, or at least a tail when
// persistence is off.
const conversationMessageQuery = `
	SELECT msg.id_message, msg.id_conversation, msg.author, msg.kind, msg.body, msg.id_user, msg.id_task,
	       coalesce(tsk.thinking_blob IS NOT NULL
	                OR coalesce(tsk.thinking_tail, '') <> ''
	                OR EXISTS (SELECT 1 FROM agent.task_thinking thk WHERE thk.id_task = tsk.id_task),
	                false) AS has_thinking,
	       msg.created_at
	FROM msg
	LEFT JOIN agent.task tsk ON tsk.id_task = msg.id_task`

type AgentConversationRepository struct {
	pool *pgxpool.Pool
}

func NewAgentConversationRepository(pool *pgxpool.Pool) *AgentConversationRepository {
	return &AgentConversationRepository{pool: pool}
}

func (r *AgentConversationRepository) InsertConversation(ctx context.Context, idUser, idUserAgent int64, idProject *int64, chatModel *string) (int64, error) {
	var idConversation int64
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO agent.conversation(id_user, id_user_agent, id_project, model)
		VALUES ($1, $2, $3, $4)
		RETURNING id_conversation`,
		idUser, idUserAgent, idProject, chatModel,
	).Scan(&idConversation)
	if err != nil {
		return 0, fmt.Errorf("inserting agent conversation: %w", err)
	}
	return idConversation, nil
}

// LoadConversation returns nil when the conversation does not exist.
func (r *AgentConversationRepository) LoadConversation(ctx context.Context, idConversation int64) (*model.AgentConversation, error) {
	return r.loadOne(ctx, idConversation, "")
}

// LoadConversationForUpdate locks the row so a project change cannot race the first message
// that locks the main project. Call it inside a transaction.
func (r *AgentConversationRepository) LoadConversationForUpdate(ctx context.Context, idConversation int64) (*model.AgentConversation, error) {
	return r.loadOne(ctx, idConversation, "FOR UPDATE OF cnv")
}

// LoadConversationByRun returns the conversation a chat run belongs to, or nil.
func (r *AgentConversationRepository) LoadConversationByRun(ctx context.Context, idRun int64) (*model.AgentConversation, error) {
	return r.loadWhere(ctx, "cnv.id_run = $1", idRun, "")
}

func (r *AgentConversationRepository) loadOne(ctx context.Context, idConversation int64, lockClause string) (*model.AgentConversation, error) {
	return r.loadWhere(ctx, "cnv.id_conversation = $1", idConversation, lockClause)
}

func (r *AgentConversationRepository) loadWhere(ctx context.Context, condition string, id int64, lockClause string) (*model.AgentConversation, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+agentConversationColumns+`
		FROM agent.conversation cnv
		WHERE `+condition+`
		`+lockClause,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("querying agent conversation: %w", err)
	}
	conversation, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[model.AgentConversation])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting agent conversation: %w", err)
	}
	return conversation, nil
}

// conversationUnread holds when the agent wrote after the owner last read the
// conversation (or the owner never read it).
const conversationUnread = `cnv.last_agent_message_at IS NOT NULL
	AND (cnv.read_at IS NULL OR cnv.last_agent_message_at > cnv.read_at)`

// LoadConversationsByUser returns the user's conversations, pinned first (latest pin on top), then newest activity. A non-nil
// idProject keeps only conversations whose main project it is.
func (r *AgentConversationRepository) LoadConversationsByUser(ctx context.Context, idUser int64, idProject *int64) ([]*model.AgentConversationListItem, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT `+agentConversationColumns+`,
		       coalesce(`+conversationUnread+`, false) AS is_unread,
		       coalesce((SELECT sum(tsk.tokens_used) FROM agent.task tsk WHERE tsk.id_run = cnv.id_run), 0) AS tokens_used,
		       (SELECT sum(tsk.cost_usd) FROM agent.task tsk WHERE tsk.id_run = cnv.id_run) AS cost_usd,
		       coalesce((SELECT bool_or(tsk.cost_estimated) FROM agent.task tsk WHERE tsk.id_run = cnv.id_run), false) AS cost_estimated
		FROM agent.conversation cnv
		WHERE cnv.id_user = $1
		  AND ($2::bigint IS NULL OR cnv.id_project = $2)
		ORDER BY cnv.pinned_at DESC NULLS LAST, cnv.updated_at DESC, cnv.id_conversation DESC`,
		idUser, idProject,
	)
	if err != nil {
		return nil, fmt.Errorf("querying agent conversations: %w", err)
	}
	conversations, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.AgentConversationListItem])
	if err != nil {
		return nil, fmt.Errorf("collecting agent conversations: %w", err)
	}
	return conversations, nil
}

// CountUnread counts the user's conversations with an agent message they have not read.
func (r *AgentConversationRepository) CountUnread(ctx context.Context, idUser int64) (int, error) {
	var count int
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT count(*) FROM agent.conversation cnv
		WHERE cnv.id_user = $1 AND `+conversationUnread,
		idUser,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting unread conversations of user %d: %w", idUser, err)
	}
	return count, nil
}

// SetRead marks every message so far as read. It leaves updated_at alone, so
// reading does not reorder the list.
func (r *AgentConversationRepository) SetRead(ctx context.Context, idConversation int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET read_at = now() WHERE id_conversation = $1`,
		idConversation,
	); err != nil {
		return fmt.Errorf("marking conversation %d read: %w", idConversation, err)
	}
	return nil
}

// SetPinned pins or unpins the conversation. Pinning an already pinned conversation keeps its
// pin time; like reading, it leaves updated_at alone.
func (r *AgentConversationRepository) SetPinned(ctx context.Context, idConversation int64, isPinned bool) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation
		SET pinned_at = CASE WHEN $2 THEN coalesce(pinned_at, now()) ELSE NULL END
		WHERE id_conversation = $1`,
		idConversation, isPinned,
	); err != nil {
		return fmt.Errorf("setting pin of conversation %d: %w", idConversation, err)
	}
	return nil
}

// SetSettings stores the model the agent answers with; like reading, it leaves updated_at alone.
func (r *AgentConversationRepository) SetSettings(ctx context.Context, idConversation int64, chatModel *string) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET model = $2 WHERE id_conversation = $1`,
		idConversation, chatModel,
	); err != nil {
		return fmt.Errorf("setting model of conversation %d: %w", idConversation, err)
	}
	return nil
}

// LoadChatQueuePosition returns where the queued chat run waits among the agent's queued chat
// runs (1 = next), or 0 when it is not queued.
func (r *AgentConversationRepository) LoadChatQueuePosition(ctx context.Context, idUserAgent, idRun int64) (int, error) {
	var position int
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT queued.position FROM (
			SELECT run.id_run,
			       row_number() OVER (ORDER BY run.queue_position ASC NULLS LAST, run.created_at ASC, run.id_run ASC) AS position
			FROM agent.run run
			WHERE run.id_user_agent = $1 AND run.kind = $2 AND run.phase = $3
		) queued
		WHERE queued.id_run = $4`,
		idUserAgent, constants.RunKindChat, constants.PhaseQueued, idRun,
	).Scan(&position)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("loading queue position of chat run %d: %w", idRun, err)
	}
	return position, nil
}

// SetMainProject sets or clears the main project. A project promoted to main leaves the context
// projects; clearing it keeps them. Call it inside a transaction.
func (r *AgentConversationRepository) SetMainProject(ctx context.Context, idConversation int64, idProject *int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET id_project = $2, updated_at = now()
		WHERE id_conversation = $1`,
		idConversation, idProject,
	); err != nil {
		return fmt.Errorf("setting conversation main project: %w", err)
	}
	if idProject != nil {
		return r.RemoveContextProject(ctx, idConversation, *idProject)
	}
	return nil
}

func (r *AgentConversationRepository) AddContextProject(ctx context.Context, idConversation, idProject int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO agent.conversation_project(id_conversation, id_project)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		idConversation, idProject,
	); err != nil {
		return fmt.Errorf("adding conversation context project: %w", err)
	}
	return nil
}

func (r *AgentConversationRepository) RemoveContextProject(ctx context.Context, idConversation, idProject int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		DELETE FROM agent.conversation_project
		WHERE id_conversation = $1 AND id_project = $2`,
		idConversation, idProject,
	); err != nil {
		return fmt.Errorf("removing conversation context project: %w", err)
	}
	return nil
}

// DeleteConversation removes the conversation with its chat run. agent.task does not cascade
// from agent.run, so the run's tasks go first; their thinking rows cascade.
func (r *AgentConversationRepository) DeleteConversation(ctx context.Context, idConversation int64) error {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `
		DELETE FROM agent.task
		WHERE id_run = (SELECT id_run FROM agent.conversation WHERE id_conversation = $1)`,
		idConversation,
	); err != nil {
		return fmt.Errorf("deleting conversation turns: %w", err)
	}
	if _, err := db.Exec(ctx, `
		DELETE FROM agent.run
		WHERE id_run = (SELECT id_run FROM agent.conversation WHERE id_conversation = $1)`,
		idConversation,
	); err != nil {
		return fmt.Errorf("deleting conversation run: %w", err)
	}
	if _, err := db.Exec(ctx, `
		DELETE FROM agent.conversation WHERE id_conversation = $1`,
		idConversation,
	); err != nil {
		return fmt.Errorf("deleting conversation: %w", err)
	}
	return nil
}

// DeleteChatRunsOfOwner deletes the chat runs of every conversation the user owns, with their
// turns and run events. Call it inside the transaction that deletes the user; returns the deleted
// runs that ran a turn, whose worktrees the gateway should drop.
func (r *AgentConversationRepository) DeleteChatRunsOfOwner(ctx context.Context, idUser int64) ([]*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	// Conversation rows first, then their runs: the order every turn transaction locks in.
	rows, err := db.Query(ctx, `
		SELECT id_run FROM agent.conversation
		WHERE id_user = $1 AND id_run IS NOT NULL
		ORDER BY id_conversation
		FOR UPDATE`,
		idUser,
	)
	if err != nil {
		return nil, fmt.Errorf("locking conversations of user %d: %w", idUser, err)
	}
	idsRun, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting chat runs of user %d: %w", idUser, err)
	}
	if len(idsRun) == 0 {
		return nil, nil
	}
	if _, err := db.Exec(ctx, `SELECT 1 FROM agent.run WHERE id_run = ANY($1) ORDER BY id_run FOR UPDATE`, idsRun); err != nil {
		return nil, fmt.Errorf("locking chat runs of user %d: %w", idUser, err)
	}
	taskRows, err := db.Query(ctx, `DELETE FROM agent.task WHERE id_run = ANY($1) RETURNING id_run`, idsRun)
	if err != nil {
		return nil, fmt.Errorf("deleting chat turns of user %d: %w", idUser, err)
	}
	idsRunWithTurn, err := pgx.CollectRows(taskRows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting deleted chat turns of user %d: %w", idUser, err)
	}
	runRows, err := db.Query(ctx, fmt.Sprintf(`DELETE FROM agent.run WHERE id_run = ANY($1) RETURNING %s`, agentRunColumns), idsRun)
	if err != nil {
		return nil, fmt.Errorf("deleting chat runs of user %d: %w", idUser, err)
	}
	defer runRows.Close()
	var deleted []*model.AgentRun
	for runRows.Next() {
		run, err := scanAgentRun(runRows)
		if err != nil {
			return nil, fmt.Errorf("scanning deleted chat run: %w", err)
		}
		if slices.Contains(idsRunWithTurn, run.IdRun) {
			deleted = append(deleted, run)
		}
	}
	return deleted, runRows.Err()
}

// LoadMessages returns the conversation in posting order.
func (r *AgentConversationRepository) LoadMessages(ctx context.Context, idConversation int64) ([]*model.AgentConversationMessage, error) {
	return r.loadMessages(ctx, idConversation, nil)
}

// LoadMessagesUntil returns the messages posted up to the given moment, oldest
// first: the history a turn that started then may see. A message posted later
// belongs to the next turn.
func (r *AgentConversationRepository) LoadMessagesUntil(ctx context.Context, idConversation int64, until time.Time) ([]*model.AgentConversationMessage, error) {
	return r.loadMessages(ctx, idConversation, &until)
}

// LoadMessage returns one message of the conversation, or nil when it is not there.
func (r *AgentConversationRepository) LoadMessage(ctx context.Context, idConversation, idMessage int64) (*model.AgentConversationMessage, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		WITH msg AS (
			SELECT * FROM agent.conversation_message
			WHERE id_conversation = $1 AND id_message = $2
		)`+conversationMessageQuery,
		idConversation, idMessage,
	)
	if err != nil {
		return nil, fmt.Errorf("querying conversation message %d: %w", idMessage, err)
	}
	message, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.AgentConversationMessage])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading conversation message %d: %w", idMessage, err)
	}
	return message, nil
}

// LoadLatestMessage returns the newest message of the conversation, or nil.
func (r *AgentConversationRepository) LoadLatestMessage(ctx context.Context, idConversation int64) (*model.AgentConversationMessage, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		WITH msg AS (
			SELECT * FROM agent.conversation_message
			WHERE id_conversation = $1
			ORDER BY id_message DESC
			LIMIT 1
		)`+conversationMessageQuery,
		idConversation,
	)
	if err != nil {
		return nil, fmt.Errorf("querying latest conversation message: %w", err)
	}
	message, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.AgentConversationMessage])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting latest conversation message: %w", err)
	}
	return message, nil
}

func (r *AgentConversationRepository) loadMessages(ctx context.Context, idConversation int64, until *time.Time) ([]*model.AgentConversationMessage, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		WITH msg AS (
			SELECT * FROM agent.conversation_message
			WHERE id_conversation = $1
			  AND ($2::timestamptz IS NULL OR created_at <= $2)
		)`+conversationMessageQuery+`
		ORDER BY msg.id_message`,
		idConversation, until,
	)
	if err != nil {
		return nil, fmt.Errorf("querying conversation messages: %w", err)
	}
	messages, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.AgentConversationMessage])
	if err != nil {
		return nil, fmt.Errorf("collecting conversation messages: %w", err)
	}
	return messages, nil
}

// InsertMessage stores a message and bumps the conversation's activity; an agent message also
// stamps last_agent_message_at, which drives the unread state. created_at is the insert time, so
// a message inserted after a turn started sorts after that turn's started_at.
func (r *AgentConversationRepository) InsertMessage(
	ctx context.Context,
	idConversation int64,
	author, kind, body string,
	idUser, idTask *int64,
) (*model.AgentConversationMessage, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		WITH msg AS (
			INSERT INTO agent.conversation_message(id_conversation, author, kind, body, id_user, id_task, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp())
			RETURNING *
		)`+conversationMessageQuery,
		idConversation, author, kind, body, idUser, idTask,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting conversation message: %w", err)
	}
	message, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.AgentConversationMessage])
	if err != nil {
		return nil, fmt.Errorf("reading inserted conversation message: %w", err)
	}
	if _, err := db.Exec(ctx, `
		UPDATE agent.conversation
		SET updated_at = now(),
		    last_agent_message_at = CASE WHEN $2 THEN now() ELSE last_agent_message_at END
		WHERE id_conversation = $1`,
		idConversation, author == constants.ConversationAuthorAgent,
	); err != nil {
		return nil, fmt.Errorf("touching conversation %d: %w", idConversation, err)
	}
	return message, nil
}

// HasUserMessageSince reports whether the owner wrote after the given moment, i.e.
// after the turn that is ending started.
func (r *AgentConversationRepository) HasUserMessageSince(ctx context.Context, idConversation int64, since time.Time) (bool, error) {
	var hasMessage bool
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM agent.conversation_message
			WHERE id_conversation = $1 AND author = $2 AND created_at > $3
		)`,
		idConversation, constants.ConversationAuthorUser, since,
	).Scan(&hasMessage)
	if err != nil {
		return false, fmt.Errorf("checking new messages of conversation %d: %w", idConversation, err)
	}
	return hasMessage, nil
}

// LockConversation fixes the main project and titles the conversation; done on the first message.
func (r *AgentConversationRepository) LockConversation(ctx context.Context, idConversation int64, title string) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET is_locked = true, title = $2
		WHERE id_conversation = $1`,
		idConversation, title,
	); err != nil {
		return fmt.Errorf("locking conversation %d: %w", idConversation, err)
	}
	return nil
}

func (r *AgentConversationRepository) SetRun(ctx context.Context, idConversation, idRun int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET id_run = $2 WHERE id_conversation = $1`,
		idConversation, idRun,
	); err != nil {
		return fmt.Errorf("setting run of conversation %d: %w", idConversation, err)
	}
	return nil
}

// SetTaskDraftRequest records (constants.TaskDraftNextTurn / TaskDraftAfterTurn)
// or clears (nil) the owner's pending task draft, with the agent answer it quotes.
func (r *AgentConversationRepository) SetTaskDraftRequest(ctx context.Context, idConversation int64, request *string, idMessage *int64) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET task_draft_request = $2, task_draft_message_id = $3 WHERE id_conversation = $1`,
		idConversation, request, idMessage,
	); err != nil {
		return fmt.Errorf("setting task draft request of conversation %d: %w", idConversation, err)
	}
	return nil
}

func (r *AgentConversationRepository) SetHeadCommit(ctx context.Context, idConversation int64, headCommit string) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		UPDATE agent.conversation SET head_commit = $2 WHERE id_conversation = $1`,
		idConversation, headCommit,
	); err != nil {
		return fmt.Errorf("setting head commit of conversation %d: %w", idConversation, err)
	}
	return nil
}

// LoadConversationByRunForUpdate is LoadByRun with the row locked; call it inside a transaction.
func (r *AgentConversationRepository) LoadConversationByRunForUpdate(ctx context.Context, idRun int64) (*model.AgentConversation, error) {
	return r.loadWhere(ctx, "cnv.id_run = $1", idRun, "FOR UPDATE OF cnv")
}
