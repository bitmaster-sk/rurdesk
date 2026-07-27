-- +goose Up

-- 1) Drop indexes that key on phase values being removed.
DROP INDEX IF EXISTS agent.agent_run_one_active_per_issue;
DROP INDEX IF EXISTS agent.run_id_user_bot_phase_active_idx;

-- 2) Reset run phases. We have no backward compatibility — open runs are
--    cancelled, not migrated. New runs use the new phase vocabulary.
UPDATE agent.run
SET phase = 'cancelled',
    error_message = COALESCE(error_message, '') ||
        CASE WHEN error_message IS NOT NULL AND error_message <> '' THEN '; ' ELSE '' END ||
        'auto-cancelled by agent-state-machine-redesign migration'
WHERE phase NOT IN ('merged', 'failed', 'cancelled');

UPDATE agent.run SET phase = 'done' WHERE phase = 'merged';

-- 3) Schema changes on agent.run.
ALTER TABLE agent.run DROP COLUMN plan_stage;
ALTER TABLE agent.run DROP COLUMN re_review_count;
ALTER TABLE agent.run DROP COLUMN run_external_id;
ALTER TABLE agent.run DROP COLUMN last_heartbeat_at;

ALTER TABLE agent.run ADD COLUMN stage_plan jsonb NOT NULL DEFAULT '{"stages":[]}'::jsonb;
ALTER TABLE agent.run ALTER COLUMN stage_plan DROP DEFAULT;

-- 4) New table for stage attempts.
CREATE TABLE agent.task (
    id_task            bigserial PRIMARY KEY,
    id_run             bigint      NOT NULL REFERENCES agent.run(id_run),
    stage              text        NOT NULL,
    attempt_no         int         NOT NULL,
    status             text        NOT NULL DEFAULT 'pending',
    id_output_message  bigint      REFERENCES messages.message(id_message),
    error_reason       text,
    error_detail       text,
    tokens_used        int,
    duration_ms        int,
    tool_calls_count   int,
    started_at         timestamptz,
    finished_at        timestamptz,
    last_heartbeat_at  timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id_run, stage, attempt_no)
);

CREATE INDEX task_id_run_idx ON agent.task(id_run);
CREATE INDEX task_status_heartbeat_idx ON agent.task(status, last_heartbeat_at)
    WHERE status = 'active';

-- 5) Re-create the partial indexes on agent.run with the new phase vocabulary.
CREATE UNIQUE INDEX agent_run_one_active_per_issue
    ON agent.run(id_issue)
    WHERE phase IN ('queued', 'in_progress', 'awaiting_input', 'awaiting_approval', 'pr_open');

CREATE INDEX run_id_user_bot_phase_active_idx
    ON agent.run(id_user_bot, phase)
    WHERE phase IN ('queued', 'in_progress', 'awaiting_input', 'awaiting_approval', 'pr_open');

-- The messages.message_kind enum is extended in the sibling migration
-- 20260526120100_message_kind_new_values.sql (cannot share a transaction
-- with the rest of the schema changes because ALTER TYPE ADD VALUE is
-- non-transactional in PostgreSQL).

-- +goose Down

-- This is a destructive forward migration; the Down path leaves the DB
-- empty of agent state and is intended only for fresh dev resets.
DROP INDEX IF EXISTS agent.run_id_user_bot_phase_active_idx;
DROP INDEX IF EXISTS agent.agent_run_one_active_per_issue;
DROP INDEX IF EXISTS agent.task_status_heartbeat_idx;
DROP INDEX IF EXISTS agent.task_id_run_idx;
DROP TABLE IF EXISTS agent.task;
ALTER TABLE agent.run DROP COLUMN stage_plan;
ALTER TABLE agent.run ADD COLUMN plan_stage text;
ALTER TABLE agent.run ADD COLUMN re_review_count int NOT NULL DEFAULT 0;
ALTER TABLE agent.run ADD COLUMN run_external_id text;
ALTER TABLE agent.run ADD COLUMN last_heartbeat_at timestamptz;

CREATE UNIQUE INDEX agent_run_one_active_per_issue
    ON agent.run(id_issue)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'awaiting_input', 'implementing', 'pr_open');
CREATE INDEX run_id_user_bot_phase_active_idx
    ON agent.run(id_user_bot, phase)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'awaiting_input', 'implementing', 'pr_open');
