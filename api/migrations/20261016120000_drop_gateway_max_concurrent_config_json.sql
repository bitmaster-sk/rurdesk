-- +goose Up
ALTER TABLE agent.gateway DROP COLUMN max_concurrent;
ALTER TABLE agent.gateway DROP COLUMN config_json;

-- +goose Down
ALTER TABLE agent.gateway ADD COLUMN max_concurrent INT NOT NULL DEFAULT 1;
ALTER TABLE agent.gateway ADD COLUMN config_json JSONB NOT NULL DEFAULT '{}';
