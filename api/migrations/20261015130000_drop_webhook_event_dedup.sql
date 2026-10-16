-- +goose Up
DROP TABLE IF EXISTS agent.webhook_event_dedup;

-- +goose Down
CREATE TABLE agent.webhook_event_dedup (
    uuid_event UUID PRIMARY KEY,
    id_run     BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent.webhook_event_dedup(created_at);
