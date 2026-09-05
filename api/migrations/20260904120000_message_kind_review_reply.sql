-- +goose NO TRANSACTION
-- +goose Up
ALTER TYPE messages.message_kind ADD VALUE IF NOT EXISTS 'review_reply';

-- +goose Down
-- A Postgres enum value cannot be removed without recreating the type, so
-- Down is a no-op; rolling back this migration alone is not meaningful.
SELECT 1;
