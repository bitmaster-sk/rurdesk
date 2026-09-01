package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// isUniqueViolation reports whether err is a Postgres unique-constraint violation
// (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var ErrRunNotFound = errors.New("agent run not found")
var ErrPhaseMismatch = errors.New("run phase does not match expected phase")

// EventMirror applies issue-state side effects when a run transitions phase.
type EventMirror interface {
	ApplyMirror(ctx context.Context, idProject, idIssue int64, toPhase string)
}

type AgentRunRepository struct {
	pool        *pgxpool.Pool
	eventMirror EventMirror
}

func NewAgentRunRepository(pool *pgxpool.Pool) *AgentRunRepository {
	return &AgentRunRepository{pool: pool}
}

func (r *AgentRunRepository) WithEventMirror(m EventMirror) *AgentRunRepository {
	r.eventMirror = m
	return r
}

const agentRunColumns = `
	id_run, id_issue, id_user_bot, id_project, id_git_integration,
	phase, stage_plan, queue_position,
	pr_url, pr_host_type, pr_id, branch_name, error_message,
	approved_mockup_ref,
	started_at, finished_at, created_at`

func scanAgentRun(row pgx.Row) (*model.AgentRun, error) {
	run := &model.AgentRun{}
	err := row.Scan(
		&run.IdRun, &run.IdIssue, &run.IdUserBot, &run.IdProject, &run.IdGitIntegration,
		&run.Phase, &run.StagePlan, &run.QueuePosition,
		&run.PrUrl, &run.PrHostType, &run.PrId, &run.BranchName, &run.ErrorMessage,
		&run.ApprovedMockupRef,
		&run.StartedAt, &run.FinishedAt, &run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning agent run: %w", err)
	}
	return run, nil
}

func (r *AgentRunRepository) Insert(ctx context.Context, idIssue, idUserBot, idProject int64, stagePlan json.RawMessage) (*model.AgentRun, error) {
	row := extctx.GetDb(ctx, r.pool).QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING %s`, agentRunColumns),
		idIssue, idUserBot, idProject, constants.PhaseQueued, stagePlan,
	)
	return scanAgentRun(row)
}

func (r *AgentRunRepository) UpdateStagePlan(ctx context.Context, idRun int64, stagePlan json.RawMessage) (*model.AgentRun, error) {
	row := extctx.GetDb(ctx, r.pool).QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.run SET stage_plan = $2 WHERE id_run = $1
		RETURNING %s`, agentRunColumns),
		idRun, stagePlan,
	)
	return scanAgentRun(row)
}

// Locks the row for the rest of the transaction; use before a read-modify-write.
func (r *AgentRunRepository) LoadByIdForUpdate(ctx context.Context, idRun int64) (*model.AgentRun, error) {
	row := extctx.GetDb(ctx, r.pool).QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run WHERE id_run = $1 FOR UPDATE`, agentRunColumns),
		idRun,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	return run, err
}

func (r *AgentRunRepository) LoadById(ctx context.Context, idRun int64) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run WHERE id_run = $1`, agentRunColumns),
		idRun,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	return run, err
}

func (r *AgentRunRepository) LoadActiveByIssue(ctx context.Context, idIssue int64) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run
		WHERE id_issue = $1
		  AND phase NOT IN ('done', 'failed', 'cancelled')
		ORDER BY created_at DESC
		LIMIT 1`, agentRunColumns),
		idIssue,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func (r *AgentRunRepository) LoadLatestByIssue(ctx context.Context, idIssue int64) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run
		WHERE id_issue = $1
		ORDER BY created_at DESC
		LIMIT 1`, agentRunColumns),
		idIssue,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func (r *AgentRunRepository) LoadByProject(ctx context.Context, idProject int64, limit int) ([]*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run
		WHERE id_project = $1
		ORDER BY created_at DESC
		LIMIT $2`, agentRunColumns),
		idProject, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying runs by project: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, scanErr := scanAgentRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating runs by project: %w", err)
	}
	return runs, nil
}

func (r *AgentRunRepository) LoadActiveByBot(ctx context.Context, idUserBot int64, phases []string) ([]*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	var rows pgx.Rows
	var err error
	if len(phases) > 0 {
		rows, err = db.Query(ctx, fmt.Sprintf(`
			SELECT %s FROM agent.run
			WHERE id_user_bot = $1
			  AND phase = ANY($2)
			ORDER BY created_at DESC`, agentRunColumns),
			idUserBot, phases,
		)
	} else {
		rows, err = db.Query(ctx, fmt.Sprintf(`
			SELECT %s FROM agent.run
			WHERE id_user_bot = $1
			  AND phase NOT IN ('done', 'failed', 'cancelled')
			ORDER BY created_at DESC`, agentRunColumns),
			idUserBot,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("querying active runs by bot: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, scanErr := scanAgentRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating active runs by bot: %w", err)
	}
	return runs, nil
}

func (r *AgentRunRepository) LoadActiveByBotAndProject(ctx context.Context, idUserBot, idProject int64) ([]*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run
		WHERE id_user_bot = $1
		  AND id_project = $2
		  AND phase NOT IN ('done', 'failed', 'cancelled')
		ORDER BY created_at DESC`, agentRunColumns),
		idUserBot, idProject,
	)
	if err != nil {
		return nil, fmt.Errorf("querying active runs by bot and project: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, scanErr := scanAgentRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating active runs by bot and project: %w", err)
	}
	return runs, nil
}

// LoadActiveBotIds returns distinct id_user_bot for runs that are not terminal.
// Used by the scheduler to know which bots to consider on each tick.
func (r *AgentRunRepository) LoadActiveBotIds(ctx context.Context) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT DISTINCT id_user_bot
		FROM agent.run
		WHERE phase NOT IN ('done', 'failed', 'cancelled')`)
	if err != nil {
		return nil, fmt.Errorf("querying active bot ids: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("scanning active bot id: %w", scanErr)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating active bot ids: %w", err)
	}
	return ids, nil
}

// LoadNextEligible returns the next run for a bot needing a stage dispatched. Order:
// manual queue_position ASC NULLS LAST, then severity order_rank ASC (lower = higher
// priority), then created_at ASC. Excludes passive (awaiting_*) and terminal phases,
// which wait for user action before the scheduler can act again.
//
// Columns are r.-qualified because the JOIN exposes id_issue on both agent.run and
// issues.issue, making the bare list ambiguous.
func (r *AgentRunRepository) LoadNextEligible(ctx context.Context, idUserBot int64) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	// order_rank lives on projects.project_issue_severity, not issues.severity, so we
	// join through it. Issues without severity get a sentinel rank to sort last.
	row := db.QueryRow(ctx, `
		SELECT r.id_run, r.id_issue, r.id_user_bot, r.id_project, r.id_git_integration,
		       r.phase, r.stage_plan, r.queue_position,
		       r.pr_url, r.pr_host_type, r.pr_id, r.branch_name, r.error_message,
		       r.approved_mockup_ref,
		       r.started_at, r.finished_at, r.created_at
		FROM agent.run r
		LEFT JOIN issues.issue i ON i.id_issue = r.id_issue
		LEFT JOIN projects.project_issue_severity pis
		       ON pis.id_severity = i.id_severity
		      AND pis.id_project = r.id_project
		WHERE r.id_user_bot = $1
		  AND r.phase IN ('queued', 'in_progress')
		  AND i.assigned_to = r.id_user_bot
		ORDER BY r.queue_position ASC NULLS LAST,
		         COALESCE(pis.order_rank, 1000000) ASC,
		         r.created_at ASC
		LIMIT 1`,
		idUserBot,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

// ReassignBot re-points a non-terminal run to a different executor bot for manual
// hand-off/resume: completed stages are preserved and the scheduler routes the next
// stage to the new bot's gateway. Returns nil if the run is already terminal.
func (r *AgentRunRepository) ReassignBot(ctx context.Context, idRun, idUserBot int64) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.run
		SET id_user_bot = $2
		WHERE id_run = $1 AND phase NOT IN ('done', 'failed', 'cancelled')
		RETURNING %s`, agentRunColumns),
		idRun, idUserBot,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

// TransitionPhase atomically transitions the run phase and writes a run_event.
// Returns ErrPhaseMismatch if the current phase != fromPhase.
func (r *AgentRunRepository) TransitionPhase(
	ctx context.Context,
	idRun int64,
	fromPhase, toPhase, actorType string,
	idUser *int64,
	reason string,
) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)

	var finishedAt *time.Time
	if constants.TerminalPhases[toPhase] {
		now := time.Now()
		finishedAt = &now
	}
	var startedAt *time.Time
	if toPhase == constants.PhaseInProgress {
		now := time.Now()
		startedAt = &now
	}

	row := db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.run
		SET phase = $3,
		    started_at  = COALESCE(started_at, $4),
		    finished_at = COALESCE(finished_at, $5)
		WHERE id_run = $1 AND phase = $2
		RETURNING %s`, agentRunColumns),
		idRun, fromPhase, toPhase, startedAt, finishedAt,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPhaseMismatch
	}
	if err != nil {
		return nil, err
	}

	reasonPtr := &reason
	if reason == "" {
		reasonPtr = nil
	}
	fromPhasePtr := &fromPhase
	toPhasePtr := &toPhase
	_, err = db.Exec(ctx, `
		INSERT INTO agent.run_event (id_run, from_phase, to_phase, actor_type, id_user, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		idRun, fromPhasePtr, toPhasePtr, actorType, idUser, reasonPtr,
	)
	if err != nil {
		return run, fmt.Errorf("inserting run event: %w", err)
	}

	if r.eventMirror != nil {
		r.eventMirror.ApplyMirror(ctx, run.IdProject, run.IdIssue, toPhase)
	}

	return run, nil
}

func (r *AgentRunRepository) SetQueuePosition(ctx context.Context, idRun int64, position int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE agent.run SET queue_position = $2 WHERE id_run = $1`,
		idRun, position,
	)
	if err != nil {
		return fmt.Errorf("setting queue position: %w", err)
	}
	return nil
}

// SetPrInfo writes PR fields onto the run and transitions in_progress → pr_open
// atomically. Returns ErrPhaseMismatch if the run is not currently in_progress.
func (r *AgentRunRepository) SetPrInfo(ctx context.Context, idRun int64, dto model.SetRunPrReq) (*model.AgentRun, error) {
	return r.SetPrInfoFrom(ctx, idRun, dto, constants.PhaseInProgress)
}

// SetPrInfoFrom writes PR info and moves the run to pr_open from fromPhase, clearing any
// stale failure stamp (finished_at, error_message). fromPhase is `in_progress` for a
// normal completion, or `failed` when reconciling a crash-orphaned run: a
// still-live gateway's late completion is accepted instead of rolled back. Returns
// ErrPhaseMismatch if the run is no longer at fromPhase (e.g. a concurrent Restart),
// which the caller treats as a no-op.
func (r *AgentRunRepository) SetPrInfoFrom(ctx context.Context, idRun int64, dto model.SetRunPrReq, fromPhase string) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.run
		SET pr_url = $2, pr_id = $3, pr_host_type = $4, branch_name = $5, id_git_integration = $6,
		    phase = $7, finished_at = NULL, error_message = NULL
		WHERE id_run = $1 AND phase = $8
		RETURNING %s`, agentRunColumns),
		idRun, dto.PrUrl, dto.PrId, dto.PrHostType, dto.BranchName, dto.IdGitIntegration,
		constants.PhasePrOpen, fromPhase,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPhaseMismatch
	}
	// A competing active run (e.g. concurrent Restart) makes pr_open violate
	// agent_run_one_active_per_issue → treat as a superseded phase mismatch, not an error.
	if isUniqueViolation(err) {
		return nil, ErrPhaseMismatch
	}
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(ctx, `
		INSERT INTO agent.run_event (id_run, from_phase, to_phase, actor_type, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		idRun, fromPhase, constants.PhasePrOpen, constants.ActorTypeAgent, "PR opened",
	)
	if err != nil {
		return run, fmt.Errorf("inserting run event: %w", err)
	}

	if r.eventMirror != nil {
		r.eventMirror.ApplyMirror(ctx, run.IdProject, run.IdIssue, constants.PhasePrOpen)
	}

	return run, nil
}

// ReconcileToPhase moves a run from fromPhase to a non-terminal toPhase, clearing a
// stale failure stamp (finished_at, error_message). Reconciles a crash-orphaned run
// whose late non-PR stage completion advances the pipeline. Returns
// ErrPhaseMismatch on a concurrent move (e.g. Restart) — a no-op for the caller. toPhase
// must be non-terminal (caller guarantees this via decideNextRunPhase).
func (r *AgentRunRepository) ReconcileToPhase(ctx context.Context, idRun int64, fromPhase, toPhase, actorType, reason string) (*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	row := db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE agent.run
		SET phase = $3, finished_at = NULL, error_message = NULL
		WHERE id_run = $1 AND phase = $2
		RETURNING %s`, agentRunColumns),
		idRun, fromPhase, toPhase,
	)
	run, err := scanAgentRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPhaseMismatch
	}
	// A competing active run makes toPhase violate agent_run_one_active_per_issue →
	// superseded phase mismatch, not a hard error.
	if isUniqueViolation(err) {
		return nil, ErrPhaseMismatch
	}
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(ctx, `
		INSERT INTO agent.run_event (id_run, from_phase, to_phase, actor_type, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		idRun, fromPhase, toPhase, actorType, reason,
	)
	if err != nil {
		return run, fmt.Errorf("inserting run event: %w", err)
	}
	return run, nil
}

// SetGitIntegration stamps the resolved git_integration onto the run so the PR-creation
// path at complete_stage knows which repo/credentials to use. Idempotent.
func (r *AgentRunRepository) SetGitIntegration(ctx context.Context, idRun, idGitIntegration int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE agent.run SET id_git_integration = $2 WHERE id_run = $1`,
		idRun, idGitIntegration,
	)
	if err != nil {
		return fmt.Errorf("setting run git integration: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) SetErrorMessage(ctx context.Context, idRun int64, msg string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE agent.run SET error_message = $2 WHERE id_run = $1`,
		idRun, msg,
	)
	if err != nil {
		return fmt.Errorf("setting run error message: %w", err)
	}
	return nil
}

// SetApprovedMockupRef records which mockup the user chose when a design stage had
// multiple ```mockup blocks. Drives the selected/rejected badges on reload and feeds
// into the next stage's prompt.
func (r *AgentRunRepository) SetApprovedMockupRef(ctx context.Context, idRun int64, ref string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE agent.run SET approved_mockup_ref = $2 WHERE id_run = $1`,
		idRun, ref,
	)
	if err != nil {
		return fmt.Errorf("setting run approved mockup ref: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) LoadEvents(ctx context.Context, idRun int64) ([]*model.AgentRunEvent, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_event, id_run, from_phase, to_phase, actor_type, id_user, reason, created_at
		FROM agent.run_event
		WHERE id_run = $1
		ORDER BY created_at ASC`,
		idRun,
	)
	if err != nil {
		return nil, fmt.Errorf("querying run events: %w", err)
	}
	defer rows.Close()

	var events []*model.AgentRunEvent
	for rows.Next() {
		ev := &model.AgentRunEvent{}
		if scanErr := rows.Scan(
			&ev.IdEvent, &ev.IdRun, &ev.FromPhase, &ev.ToPhase,
			&ev.ActorType, &ev.IdUser, &ev.Reason, &ev.CreatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scanning run event: %w", scanErr)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run events: %w", err)
	}
	return events, nil
}

func (r *AgentRunRepository) InsertEvent(
	ctx context.Context,
	idRun int64,
	fromPhase, toPhase, actorType string,
	idUser *int64,
	reason string,
) error {
	db := extctx.GetDb(ctx, r.pool)
	var fromPtr, toPtr, reasonPtr *string
	if fromPhase != "" {
		fromPtr = &fromPhase
	}
	if toPhase != "" {
		toPtr = &toPhase
	}
	if reason != "" {
		reasonPtr = &reason
	}
	_, err := db.Exec(ctx, `
		INSERT INTO agent.run_event (id_run, from_phase, to_phase, actor_type, id_user, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		idRun, fromPtr, toPtr, actorType, idUser, reasonPtr,
	)
	if err != nil {
		return fmt.Errorf("inserting run event: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) CountEvents(ctx context.Context, idRun int64) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int64
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent.run_event WHERE id_run = $1`,
		idRun,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting run events: %w", err)
	}
	return count, nil
}

// LoadPrOpenRuns returns runs in pr_open phase that have a pr_id set, for the merge poller.
func (r *AgentRunRepository) LoadPrOpenRuns(ctx context.Context, limit int) ([]*model.AgentRun, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, fmt.Sprintf(`
		SELECT %s FROM agent.run
		WHERE phase = 'pr_open' AND pr_id IS NOT NULL
		ORDER BY id_run
		LIMIT $1`, agentRunColumns),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying pr-open runs: %w", err)
	}
	defer rows.Close()

	var runs []*model.AgentRun
	for rows.Next() {
		run, scanErr := scanAgentRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pr-open runs: %w", err)
	}
	return runs, nil
}
