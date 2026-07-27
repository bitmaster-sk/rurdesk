-- +goose Up
CREATE TYPE messages.message_kind AS ENUM ('comment', 'plan', 'clarification');

ALTER TABLE messages.message
    ADD COLUMN message_kind messages.message_kind NOT NULL DEFAULT 'comment';

-- Backfill: any existing message body starting with [plan:simple] or
-- [plan:detailed] becomes a plan message. Body text is left intact — only
-- the kind label is set. UI keys off the column, not the body prefix.
UPDATE messages.message
SET message_kind = 'plan'
WHERE message LIKE '[plan:simple]%'
   OR message LIKE '[plan:detailed]%';

-- Add awaiting_input to the active-phase filter sets. agent.run.phase remains
-- a VARCHAR (consistent with the existing schema), so no enum to extend; only
-- the partial-index predicates need to learn about the new phase.
--
-- The heartbeat index is *not* extended — awaiting_input is a paused state,
-- the agent process is not running and is not expected to heartbeat.
--
-- The two partial indexes being replaced are dropped by querying pg_indexes
-- rather than guessing their auto-generated names (the original CREATE INDEX
-- statements in 20260423110000_agent_schema.sql did not specify a name).
DROP INDEX IF EXISTS agent.agent_run_one_active_per_issue;

-- +goose StatementBegin
DO $$
DECLARE
    idx_name TEXT;
BEGIN
    FOR idx_name IN
        SELECT indexname FROM pg_indexes
        WHERE schemaname = 'agent'
          AND tablename = 'run'
          AND indexname <> 'run_pkey'
          AND indexdef LIKE '%(id_user_bot, phase)%'
    LOOP
        EXECUTE format('DROP INDEX agent.%I', idx_name);
    END LOOP;
END $$;
-- +goose StatementEnd

CREATE UNIQUE INDEX agent_run_one_active_per_issue
    ON agent.run(id_issue)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'awaiting_input', 'implementing', 'pr_open');

CREATE INDEX run_id_user_bot_phase_active_idx
    ON agent.run(id_user_bot, phase)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'awaiting_input', 'implementing', 'pr_open');

-- +goose Down
DROP INDEX IF EXISTS agent.agent_run_one_active_per_issue;
DROP INDEX IF EXISTS agent.run_id_user_bot_phase_active_idx;

CREATE UNIQUE INDEX agent_run_one_active_per_issue
    ON agent.run(id_issue)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'implementing', 'pr_open');

CREATE INDEX ON agent.run(id_user_bot, phase)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'implementing', 'pr_open');

ALTER TABLE messages.message DROP COLUMN message_kind;
DROP TYPE messages.message_kind;
