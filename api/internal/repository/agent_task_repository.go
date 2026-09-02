package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentTaskRepository struct {
	pool *pgxpool.Pool
}

func NewAgentTaskRepository(pool *pgxpool.Pool) *AgentTaskRepository {
	return &AgentTaskRepository{pool: pool}
}

const agentTaskColumns = `
	id_task, id_run, id_user_bot, stage, attempt_no, status, id_output_message,
	error_reason, error_detail,
	tokens_used, duration_ms, tool_calls_count,
	started_at, finished_at, last_heartbeat_at, created_at`

func scanAgentTask(row pgx.Row) (*model.AgentTask, error) {
	task := &model.AgentTask{}
	err := row.Scan(
		&task.IdTask, &task.IdRun, &task.IdUserBot, &task.Stage, &task.AttemptNo, &task.Status, &task.IdOutputMessage,
		&task.ErrorReason, &task.ErrorDetail,
		&task.TokensUsed, &task.DurationMs, &task.ToolCallsCount,
		&task.StartedAt, &task.FinishedAt, &task.LastHeartbeatAt, &task.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning agent task: %w", err)
	}
	return task, nil
}

// Insert creates a pending task for a stage attempt. idUserBot is the
// executing bot, which can differ across stages after a manual hand-off.
func (r *AgentTaskRepository) Insert(ctx context.Context, idRun, idUserBot int64, stage string, attemptNo int) (*model.AgentTask, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO agent.task (id_run, id_user_bot, stage, attempt_no, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING %s`, agentTaskColumns),
		idRun, idUserBot, stage, attemptNo, constants.TaskStatusPending,
	)
	return scanAgentTask(row)
}

func (r *AgentTaskRepository) LoadById(ctx context.Context, idTask int64) (*model.AgentTask, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM agent.task WHERE id_task = $1`, agentTaskColumns), idTask)
	task, err := scanAgentTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	return task, err
}

// BotForTask returns the id of the bot that owns the task's run. Callers use it
// to authorize gateway callbacks that address a task directly, without paying
// for two full row loads on hot paths like the 30s heartbeat.
func (r *AgentTaskRepository) BotForTask(ctx context.Context, idTask int64) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	var idUserBot int64
	err := db.QueryRow(ctx, `
		SELECT r.id_user_bot
		FROM agent.task t
		JOIN agent.run r ON r.id_run = t.id_run
		WHERE t.id_task = $1`,
		idTask,
	).Scan(&idUserBot)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrTaskNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("loading bot for task %d: %w", idTask, err)
	}
	return idUserBot, nil
}

func (r *AgentTaskRepository) LoadByRun(ctx context.Context, idRun int64) ([]*model.AgentTask, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s FROM agent.task WHERE id_run = $1 ORDER BY created_at ASC`, agentTaskColumns), idRun)
	if err != nil {
		return nil, fmt.Errorf("querying tasks by run: %w", err)
	}
	defer rows.Close()

	var tasks []*model.AgentTask
	for rows.Next() {
		task, scanErr := scanAgentTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tasks by run: %w", err)
	}
	return tasks, nil
}

func (r *AgentTaskRepository) LoadLatestForStage(ctx context.Context, idRun int64, stage string) (*model.AgentTask, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.task
		WHERE id_run = $1 AND stage = $2
		ORDER BY attempt_no DESC
		LIMIT 1`, agentTaskColumns),
		idRun, stage,
	)
	task, err := scanAgentTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return task, err
}

// TransitionStatus is a CAS-style update: sets started_at on transition to
// active, finished_at on transition to a terminal status.
func (r *AgentTaskRepository) TransitionStatus(ctx context.Context, idTask int64, fromStatus, toStatus string) (*model.AgentTask, error) {
	db := extctx.GetDb(ctx, r.pool)

	timingClause := ""
	switch toStatus {
	case constants.TaskStatusActive:
		timingClause = ", started_at = COALESCE(started_at, now()), last_heartbeat_at = now()"
	case constants.TaskStatusCompleted, constants.TaskStatusFailed, constants.TaskStatusCancelled:
		timingClause = ", finished_at = COALESCE(finished_at, now())"
	}

	row := db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.task
		SET status = $3 %s
		WHERE id_task = $1 AND status = $2
		RETURNING %s`, timingClause, agentTaskColumns),
		idTask, fromStatus, toStatus,
	)
	task, err := scanAgentTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTaskStatusMismatch
	}
	return task, err
}

// CompleteReconcilable transitions a task to toStatus from `active` (normal
// completion) or from `failed` with a recoverable error_reason (a late
// complete_stage from a gateway orphaned by a restart). Otherwise returns
// ErrTaskStatusMismatch so the caller can no-op instead of corrupting state.
func (r *AgentTaskRepository) CompleteReconcilable(ctx context.Context, idTask int64, toStatus string, recoverableReasons []string) (*model.AgentTask, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.task
		SET status = $2, finished_at = COALESCE(finished_at, now())
		WHERE id_task = $1
		  AND (status = 'active'
		       OR (status = 'failed' AND error_reason = ANY($3)))
		RETURNING %s`, agentTaskColumns),
		idTask, toStatus, recoverableReasons,
	)
	task, err := scanAgentTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTaskStatusMismatch
	}
	return task, err
}

func (r *AgentTaskRepository) SetOutputAndStats(
	ctx context.Context,
	idTask int64,
	idOutputMessage *int64,
	tokensUsed, durationMs, toolCallsCount *int,
) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE agent.task
		SET id_output_message = $2,
		    tokens_used = $3,
		    duration_ms = $4,
		    tool_calls_count = $5
		WHERE id_task = $1`,
		idTask, idOutputMessage, tokensUsed, durationMs, toolCallsCount,
	)
	if err != nil {
		return fmt.Errorf("setting task output and stats: %w", err)
	}
	return nil
}

// UpdateStats backfills usage counters for the gateway success path, since
// complete_stage rarely carries them and the stats panel would otherwise
// stay at zero. A nil field keeps the existing value.
func (r *AgentTaskRepository) UpdateStats(
	ctx context.Context,
	idTask int64,
	tokensUsed, durationMs, toolCallsCount *int,
) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE agent.task
		SET tokens_used      = COALESCE($2, tokens_used),
		    duration_ms      = COALESCE($3, duration_ms),
		    tool_calls_count = COALESCE($4, tool_calls_count)
		WHERE id_task = $1`,
		idTask, tokensUsed, durationMs, toolCallsCount,
	)
	if err != nil {
		return fmt.Errorf("updating task stats: %w", err)
	}
	return nil
}

func (r *AgentTaskRepository) SetError(ctx context.Context, idTask int64, reason, detail string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE agent.task
		SET error_reason = $2, error_detail = $3
		WHERE id_task = $1`,
		idTask, reason, detail,
	)
	if err != nil {
		return fmt.Errorf("setting task error: %w", err)
	}
	return nil
}

func (r *AgentTaskRepository) RecordHeartbeat(ctx context.Context, idTask int64) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE agent.task
		SET last_heartbeat_at = now()
		WHERE id_task = $1 AND status = 'active'`,
		idTask,
	)
	if err != nil {
		return fmt.Errorf("recording task heartbeat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTaskStatusMismatch
	}
	return nil
}

// FailStaleHeartbeats marks active tasks with no heartbeat for longer than
// maxAge as failed, returning the distinct affected run ids so the caller can
// fail those runs too. maxAge=0 fails every active task (startup crash recovery).
func (r *AgentTaskRepository) FailStaleHeartbeats(ctx context.Context, maxAge time.Duration) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)

	var rows pgx.Rows
	var err error
	if maxAge == 0 {
		rows, err = db.Query(ctx, `
			UPDATE agent.task
			SET status = 'failed',
			    finished_at = now(),
			    error_reason = COALESCE(error_reason, 'crash_recovery')
			WHERE status = 'active'
			RETURNING id_run`)
	} else {
		rows, err = db.Query(ctx, `
			UPDATE agent.task
			SET status = 'failed',
			    finished_at = now(),
			    error_reason = COALESCE(error_reason, 'heartbeat_stale')
			WHERE status = 'active'
			  AND last_heartbeat_at < now() - $1::interval
			RETURNING id_run`,
			fmt.Sprintf("%d milliseconds", maxAge.Milliseconds()),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failing stale heartbeats: %w", err)
	}
	defer rows.Close()

	return distinctRunIds(rows)
}

// FailActiveForBot marks every active task of the bot's runs as failed and
// returns the affected run ids. Used when a gateway reports a (re)start: its
// in-flight subprocesses are gone, so the caller fails those runs so the user
// sees Continue/Restart.
func (r *AgentTaskRepository) FailActiveForBot(ctx context.Context, idUserBot int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		UPDATE agent.task t
		SET status = 'failed',
		    finished_at = now(),
		    error_reason = COALESCE(error_reason, 'gateway_restart')
		FROM agent.run r
		WHERE t.id_run = r.id_run
		  AND r.id_user_bot = $1
		  AND t.status = 'active'
		RETURNING t.id_run`,
		idUserBot,
	)
	if err != nil {
		return nil, fmt.Errorf("failing active tasks for bot: %w", err)
	}
	defer rows.Close()
	return distinctRunIds(rows)
}

func distinctRunIds(rows pgx.Rows) ([]int64, error) {
	seen := map[int64]bool{}
	var ids []int64
	for rows.Next() {
		var idRun int64
		if scanErr := rows.Scan(&idRun); scanErr != nil {
			return nil, fmt.Errorf("scanning run id: %w", scanErr)
		}
		if !seen[idRun] {
			seen[idRun] = true
			ids = append(ids, idRun)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run ids: %w", err)
	}
	return ids, nil
}

// BotHasActiveTask reports whether the bot has any task currently active,
// so the scheduler can skip dispatching while it's busy with a stage.
func (r *AgentTaskRepository) BotHasActiveTask(ctx context.Context, idUserBot int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var n int
	err := db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM agent.task t
		JOIN agent.run r ON r.id_run = t.id_run
		WHERE r.id_user_bot = $1 AND t.status = 'active'`,
		idUserBot,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking bot active task: %w", err)
	}
	return n > 0, nil
}

// CancelNonTerminalForRun cancels any pending/active tasks under the given
// run. Used by Restart and Cancel paths.
func (r *AgentTaskRepository) CancelNonTerminalForRun(ctx context.Context, idRun int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE agent.task
		SET status = 'cancelled', finished_at = now()
		WHERE id_run = $1 AND status IN ('pending', 'active')`,
		idRun,
	)
	if err != nil {
		return fmt.Errorf("cancelling non-terminal tasks for run: %w", err)
	}
	return nil
}

// StatsForRun returns aggregate counters per stage / per status.
func (r *AgentTaskRepository) StatsForRun(ctx context.Context, idRun int64) (*model.RunStatsRes, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT stage, status, COUNT(*),
		       COALESCE(SUM(tokens_used), 0),
		       COALESCE(SUM(duration_ms), 0),
		       COALESCE(SUM(tool_calls_count), 0)
		FROM agent.task
		WHERE id_run = $1
		GROUP BY stage, status`,
		idRun,
	)
	if err != nil {
		return nil, fmt.Errorf("querying run stats: %w", err)
	}
	defer rows.Close()

	stats := &model.RunStatsRes{AttemptsPerStage: map[string]int{}}
	for rows.Next() {
		var stage, status string
		var count, tokens, duration, toolCalls int
		if scanErr := rows.Scan(&stage, &status, &count, &tokens, &duration, &toolCalls); scanErr != nil {
			return nil, fmt.Errorf("scanning run stats: %w", scanErr)
		}
		stats.TotalTokensUsed += tokens
		stats.TotalDurationMs += duration
		stats.TotalToolCallsCount += toolCalls
		stats.AttemptsPerStage[stage] += count
		if status == constants.TaskStatusFailed {
			stats.FailedAttempts += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run stats: %w", err)
	}
	return stats, nil
}
