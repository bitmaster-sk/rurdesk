-- +goose Up
ALTER TABLE agent.task
    ADD COLUMN input_tokens INT,
    ADD COLUMN output_tokens INT,
    ADD COLUMN cache_read_tokens INT,
    ADD COLUMN cache_write_tokens INT,
    ADD COLUMN cost_usd NUMERIC(12,6),
    ADD COLUMN cost_estimated BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE agent.task
    DROP COLUMN input_tokens,
    DROP COLUMN output_tokens,
    DROP COLUMN cache_read_tokens,
    DROP COLUMN cache_write_tokens,
    DROP COLUMN cost_usd,
    DROP COLUMN cost_estimated;
