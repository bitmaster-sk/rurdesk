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

type AgentThinkingRepository struct {
	pool *pgxpool.Pool
}

func NewAgentThinkingRepository(pool *pgxpool.Pool) *AgentThinkingRepository {
	return &AgentThinkingRepository{pool: pool}
}

func (r *AgentThinkingRepository) StoredSize(ctx context.Context, idTask int64) (storedBytes int, isTruncated bool, err error) {
	db := extctx.GetDb(ctx, r.pool)
	// octet_length, not length: the cap is a byte budget, and length() counts
	// characters, which lets non-ASCII thinking store several times the cap.
	if err := db.QueryRow(ctx, `
		SELECT coalesce(sum(octet_length(text) + octet_length(tool)), 0),
		       count(*) FILTER (WHERE kind = $2) > 0
		FROM agent.task_thinking WHERE id_task = $1`,
		idTask, model.ThinkingKindTruncated,
	).Scan(&storedBytes, &isTruncated); err != nil {
		return 0, false, fmt.Errorf("measuring stored thinking of task %d: %w", idTask, err)
	}
	return storedBytes, isTruncated, nil
}

func (r *AgentThinkingRepository) Append(ctx context.Context, idTask int64, seq int, events model.AgentThinkingEvents) error {
	db := extctx.GetDb(ctx, r.pool)
	kinds := make([]string, len(events))
	tools := make([]string, len(events))
	texts := make([]string, len(events))
	timestamps := make([]int64, len(events))
	for index, event := range events {
		kinds[index] = event.Kind
		tools[index] = event.Tool
		texts[index] = event.Text
		timestamps[index] = event.At
	}
	_, err := db.Exec(ctx, `
		INSERT INTO agent.task_thinking (id_task, seq, event_index, kind, tool, text, event_at)
		SELECT $1, $2, ordinality - 1, kind, tool, text, event_at
		FROM unnest($3::text[], $4::text[], $5::text[], $6::bigint[])
			WITH ORDINALITY AS event(kind, tool, text, event_at, ordinality)
		ON CONFLICT (id_task, seq, event_index) DO NOTHING`,
		idTask, seq, kinds, tools, texts, timestamps)
	if err != nil {
		return fmt.Errorf("appending thinking batch of task %d: %w", idTask, err)
	}
	return nil
}

func (r *AgentThinkingRepository) MarkTruncated(ctx context.Context, idTask int64, seq int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO agent.task_thinking (id_task, seq, event_index, kind, text)
		SELECT $1, $2, -1, $3, ''
		WHERE NOT EXISTS (
			SELECT 1 FROM agent.task_thinking WHERE id_task = $1 AND kind = $3
		)
		ON CONFLICT (id_task, seq, event_index) DO NOTHING`,
		idTask, seq, model.ThinkingKindTruncated)
	if err != nil {
		return fmt.Errorf("marking thinking of task %d truncated: %w", idTask, err)
	}
	return nil
}

func (r *AgentThinkingRepository) LoadEvents(ctx context.Context, idTask int64) (model.AgentThinkingEvents, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx,
		"SELECT kind, tool, text, event_at FROM agent.task_thinking WHERE id_task = $1 ORDER BY seq, event_index", idTask)
	if err != nil {
		return nil, fmt.Errorf("loading thinking of task %d: %w", idTask, err)
	}
	defer rows.Close()

	var events model.AgentThinkingEvents
	for rows.Next() {
		var event model.AgentThinkingEvent
		if err := rows.Scan(&event.Kind, &event.Tool, &event.Text, &event.At); err != nil {
			return nil, fmt.Errorf("scanning thinking event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// Compact stores the blob and the tail on the task and deletes its event rows.
// A nil blob keeps only the tail.
func (r *AgentThinkingRepository) Compact(ctx context.Context, idTask int64, blob []byte, tail string) error {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `
		UPDATE agent.task
		SET thinking_blob = coalesce($2, thinking_blob),
		    thinking_tail = coalesce(nullif($3, ''), thinking_tail)
		WHERE id_task = $1`, idTask, blob, tail); err != nil {
		return fmt.Errorf("compacting thinking of task %d: %w", idTask, err)
	}
	if _, err := db.Exec(ctx,
		"DELETE FROM agent.task_thinking WHERE id_task = $1", idTask); err != nil {
		return fmt.Errorf("clearing thinking chunks of task %d: %w", idTask, err)
	}
	return nil
}

// LoadCompacted returns the compacted thinking of the stage's latest attempt.
// A stage with no attempt on record reads the same as one that recorded
// nothing: no blob, no tail, no error.
func (r *AgentThinkingRepository) LoadCompacted(ctx context.Context, idRun int64, stage string) (blob []byte, tail string, err error) {
	db := extctx.GetDb(ctx, r.pool)
	var storedTail *string
	err = db.QueryRow(ctx, `
		SELECT thinking_blob, thinking_tail
		FROM agent.task
		WHERE id_run = $1 AND stage = $2
		ORDER BY attempt_no DESC, created_at DESC
		LIMIT 1`, idRun, stage).Scan(&blob, &storedTail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("loading compacted thinking of run %d stage %s: %w", idRun, stage, err)
	}
	if storedTail != nil {
		tail = *storedTail
	}
	return blob, tail, nil
}

func (r *AgentThinkingRepository) OrphanedTaskIds(ctx context.Context) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT DISTINCT chunk.id_task
		FROM agent.task_thinking chunk
		JOIN agent.task task ON task.id_task = chunk.id_task
		WHERE task.status NOT IN ($1, $2)`,
		constants.TaskStatusPending, constants.TaskStatusActive)
	if err != nil {
		return nil, fmt.Errorf("listing orphaned thinking tasks: %w", err)
	}
	defer rows.Close()

	var idsTask []int64
	for rows.Next() {
		var idTask int64
		if err := rows.Scan(&idTask); err != nil {
			return nil, fmt.Errorf("scanning orphaned thinking task: %w", err)
		}
		idsTask = append(idsTask, idTask)
	}
	return idsTask, rows.Err()
}
