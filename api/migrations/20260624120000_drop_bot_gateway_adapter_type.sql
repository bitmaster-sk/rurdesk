-- +goose Up
-- adapter_type was never read by the tracker: the gateway selects its adapter
-- from its own --adapter flag, the webhook payload doesn't carry it, and no
-- service branches on it. Drop the dead column.
ALTER TABLE agent.bot_gateway DROP COLUMN adapter_type;

-- +goose Down
ALTER TABLE agent.bot_gateway ADD COLUMN adapter_type VARCHAR(50) NOT NULL DEFAULT 'claude-code';
ALTER TABLE agent.bot_gateway ALTER COLUMN adapter_type DROP DEFAULT;
