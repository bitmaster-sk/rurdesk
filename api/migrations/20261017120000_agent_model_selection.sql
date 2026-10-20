-- +goose Up
ALTER TABLE agent.gateway ADD COLUMN models TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE agent.gateway ADD COLUMN model_default TEXT;
ALTER TABLE agent.gateway ADD COLUMN models_reported_at TIMESTAMPTZ;
ALTER TABLE agent.gateway ADD COLUMN max_turns_limit INT;
ALTER TABLE agent.gateway ADD COLUMN max_turns_plan INT CHECK (max_turns_plan > 0);
ALTER TABLE agent.gateway ADD COLUMN max_turns_implement INT CHECK (max_turns_implement > 0);

ALTER TABLE agent.task ADD COLUMN model TEXT;
ALTER TABLE agent.task ADD COLUMN max_turns INT;

-- +goose Down
ALTER TABLE agent.task DROP COLUMN max_turns;
ALTER TABLE agent.task DROP COLUMN model;

ALTER TABLE agent.gateway DROP COLUMN max_turns_implement;
ALTER TABLE agent.gateway DROP COLUMN max_turns_plan;
ALTER TABLE agent.gateway DROP COLUMN max_turns_limit;
ALTER TABLE agent.gateway DROP COLUMN models_reported_at;
ALTER TABLE agent.gateway DROP COLUMN model_default;
ALTER TABLE agent.gateway DROP COLUMN models;
