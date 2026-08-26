package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentOverviewRepository struct {
	pool *pgxpool.Pool
}

func NewAgentOverviewRepository(pool *pgxpool.Pool) *AgentOverviewRepository {
	return &AgentOverviewRepository{pool: pool}
}

func (r *AgentOverviewRepository) Load(ctx context.Context, idProject int64) ([]*model.AgentOverview, error) {
	db := extctx.GetDb(ctx, r.pool)

	// Project members only: an instance-wide list would tell a member which agents
	// exist in projects they cannot see.
	rows, err := db.Query(ctx, `
		SELECT usr.id_user
		FROM users.user usr
		WHERE usr.is_bot
		  AND (
			usr.id_user IN (
				SELECT pru.id_user 
				FROM projects.project_user pru 
				WHERE pru.id_project = $1
			) OR usr.id_user IN (
				SELECT ust.id_user
				FROM projects.project_team prt
				INNER JOIN users.user_team ust ON ust.id_team = prt.id_team
				WHERE prt.id_project = $1
			)
		  )
		ORDER BY usr.id_user
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying bot users: %w", err)
	}
	byBot := map[int64]*model.AgentOverview{}
	var out []*model.AgentOverview
	for rows.Next() {
		var idUser int64
		if err := rows.Scan(&idUser); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning bot user: %w", err)
		}
		entry := &model.AgentOverview{IdUserBot: idUser, QueuedIdsIssuePublic: []int64{}}
		byBot[idUser] = entry
		out = append(out, entry)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating bot users: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}

	if err := r.applyActive(ctx, idProject, byBot); err != nil {
		return nil, err
	}
	if err := r.applyQueue(ctx, idProject, byBot); err != nil {
		return nil, err
	}
	if err := r.applyCompletedToday(ctx, byBot); err != nil {
		return nil, err
	}
	if err := r.applyTaskCounters(ctx, byBot); err != nil {
		return nil, err
	}
	if err := r.applyAvgDuration(ctx, byBot); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AgentOverviewRepository) applyActive(ctx context.Context, idProject int64, byBot map[int64]*model.AgentOverview) error {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT t.id_user_bot, t.stage, r.id_project, i.id_issue_public
		FROM agent.task t
		JOIN agent.run r ON r.id_run = t.id_run
		JOIN issues.issue i ON i.id_issue = r.id_issue
		WHERE t.status = $1
	`, constants.TaskStatusActive)
	if err != nil {
		return fmt.Errorf("querying active tasks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idUserBot, idRunProject, idIssuePublic int64
		var stage string
		if err := rows.Scan(&idUserBot, &stage, &idRunProject, &idIssuePublic); err != nil {
			return fmt.Errorf("scanning active task: %w", err)
		}
		entry, ok := byBot[idUserBot]
		if !ok {
			continue
		}
		entry.IsBusy = true
		if idRunProject == idProject {
			entry.Current = &model.AgentCurrentRun{IdIssuePublic: idIssuePublic, Stage: stage}
		}
	}
	return rows.Err()
}

func (r *AgentOverviewRepository) applyQueue(ctx context.Context, idProject int64, byBot map[int64]*model.AgentOverview) error {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT r.id_user_bot, r.id_project, i.id_issue_public
		FROM agent.run r
		JOIN issues.issue i ON i.id_issue = r.id_issue
		WHERE r.phase = $1
		ORDER BY r.created_at
	`, constants.PhaseQueued)
	if err != nil {
		return fmt.Errorf("querying queued runs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idUserBot, idRunProject, idIssuePublic int64
		if err := rows.Scan(&idUserBot, &idRunProject, &idIssuePublic); err != nil {
			return fmt.Errorf("scanning queued run: %w", err)
		}
		entry, ok := byBot[idUserBot]
		if !ok {
			continue
		}
		entry.QueueCount++
		if idRunProject == idProject {
			entry.QueuedIdsIssuePublic = append(entry.QueuedIdsIssuePublic, idIssuePublic)
		}
	}
	return rows.Err()
}

func (r *AgentOverviewRepository) applyCompletedToday(ctx context.Context, byBot map[int64]*model.AgentOverview) error {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_user_bot, COUNT(*)
		FROM agent.run
		WHERE phase = $1 AND finished_at >= date_trunc('day', now())
		GROUP BY id_user_bot
	`, constants.PhaseDone)
	if err != nil {
		return fmt.Errorf("querying completed runs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idUserBot int64
		var count int
		if err := rows.Scan(&idUserBot, &count); err != nil {
			return fmt.Errorf("scanning completed run count: %w", err)
		}
		if entry, ok := byBot[idUserBot]; ok {
			entry.CompletedToday = count
		}
	}
	return rows.Err()
}

func (r *AgentOverviewRepository) applyTaskCounters(ctx context.Context, byBot map[int64]*model.AgentOverview) error {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_user_bot,
		       COALESCE(SUM(tokens_used), 0),
		       COUNT(*) FILTER (WHERE status = $1)
		FROM agent.task
		WHERE created_at >= now() - interval '7 days'
		GROUP BY id_user_bot
	`, constants.TaskStatusFailed)
	if err != nil {
		return fmt.Errorf("querying task counters: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idUserBot, tokens int64
		var failed int
		if err := rows.Scan(&idUserBot, &tokens, &failed); err != nil {
			return fmt.Errorf("scanning task counters: %w", err)
		}
		if entry, ok := byBot[idUserBot]; ok {
			entry.Tokens7d = tokens
			entry.FailedAttempts7d = failed
		}
	}
	return rows.Err()
}

func (r *AgentOverviewRepository) applyAvgDuration(ctx context.Context, byBot map[int64]*model.AgentOverview) error {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT id_user_bot,
		       AVG(EXTRACT(EPOCH FROM (finished_at - started_at)) * 1000)::bigint
		FROM agent.run
		WHERE phase = $1 AND finished_at >= now() - interval '7 days' AND started_at IS NOT NULL
		GROUP BY id_user_bot
	`, constants.PhaseDone)
	if err != nil {
		return fmt.Errorf("querying average run duration: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idUserBot int64
		var avgMs *int64
		if err := rows.Scan(&idUserBot, &avgMs); err != nil {
			return fmt.Errorf("scanning average run duration: %w", err)
		}
		if entry, ok := byBot[idUserBot]; ok {
			entry.AvgRunDurationMs7d = avgMs
		}
	}
	return rows.Err()
}
