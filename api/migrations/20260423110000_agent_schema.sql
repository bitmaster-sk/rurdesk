-- +goose Up
CREATE SCHEMA IF NOT EXISTS agent;

CREATE TABLE agent.bot_gateway (
    id_bot_gateway BIGSERIAL PRIMARY KEY,
    id_user_bot    BIGINT NOT NULL REFERENCES users.user(id_user) ON DELETE CASCADE,
    gateway_url    TEXT NOT NULL,
    adapter_type   VARCHAR(50) NOT NULL,
    max_concurrent INT NOT NULL DEFAULT 1,
    webhook_secret BYTEA NOT NULL,
    config_json    JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX ON agent.bot_gateway(id_user_bot);

CREATE TABLE agent.run (
    id_run              BIGSERIAL PRIMARY KEY,
    id_issue            BIGINT NOT NULL REFERENCES issues.issue(id_issue) ON DELETE CASCADE,
    id_user_bot         BIGINT NOT NULL REFERENCES users.user(id_user),
    id_project          BIGINT NOT NULL REFERENCES projects.project(id_project) ON DELETE CASCADE,
    id_git_integration  BIGINT REFERENCES projects.git_integration(id_git_integration) ON DELETE SET NULL,
    phase               VARCHAR(30) NOT NULL DEFAULT 'queued',
    plan_stage          VARCHAR(20),
    re_review_count     INT NOT NULL DEFAULT 0,
    queue_position      INT,
    run_external_id     TEXT,
    pr_url              TEXT,
    pr_host_type        VARCHAR(20),
    pr_id               TEXT,
    branch_name         TEXT,
    error_message       TEXT,
    last_heartbeat_at   TIMESTAMPTZ,
    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent.run(id_issue);
CREATE INDEX ON agent.run(id_user_bot, phase)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'implementing', 'pr_open');
CREATE INDEX ON agent.run(phase, last_heartbeat_at)
    WHERE phase IN ('pickup', 'planning', 'implementing');
CREATE UNIQUE INDEX agent_run_one_active_per_issue ON agent.run(id_issue)
    WHERE phase IN ('queued', 'pickup', 'planning', 'awaiting_approval', 'implementing', 'pr_open');

CREATE TABLE agent.run_event (
    id_event   BIGSERIAL PRIMARY KEY,
    id_run     BIGINT NOT NULL REFERENCES agent.run(id_run) ON DELETE CASCADE,
    from_phase VARCHAR(30),
    to_phase   VARCHAR(30),
    actor_type VARCHAR(20) NOT NULL,
    id_user    BIGINT REFERENCES users.user(id_user),
    reason     TEXT,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent.run_event(id_run, created_at);

CREATE TABLE agent.webhook_event_dedup (
    uuid_event UUID PRIMARY KEY,
    id_run     BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent.webhook_event_dedup(created_at);

-- +goose Down
DROP TABLE IF EXISTS agent.webhook_event_dedup;
DROP TABLE IF EXISTS agent.run_event;
DROP TABLE IF EXISTS agent.run;
DROP TABLE IF EXISTS agent.bot_gateway;
DROP SCHEMA IF EXISTS agent CASCADE;
