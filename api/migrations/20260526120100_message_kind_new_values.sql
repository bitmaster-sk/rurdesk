-- +goose NO TRANSACTION
-- +goose Up
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'brainstorming_question';
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'brainstorming_complete';
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'design';
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'implementation_plan';
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'pull_request_pushed';
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'implementation_done';

-- +goose Down
-- The legacy 'plan' and 'clarification' values cannot be removed from a
-- Postgres enum without recreating the type. The new values likewise cannot
-- be removed once added. Down is therefore a no-op; rolling back this
-- migration alone is not meaningful.
SELECT 1;
