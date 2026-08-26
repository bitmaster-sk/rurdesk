-- +goose Up
CREATE TABLE agent.skill (
    id_skill    bigserial PRIMARY KEY,
    name        text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT '',
    content     text NOT NULL,
    -- Identity of a shipped skill; NULL = user-created. The shipped original text
    -- lives in the binary (api/internal/agent/skills/builtin/), not here.
    builtin_key text UNIQUE,
    -- Hash of the shipped version this row was last synced or restored to. Equal to
    -- the hash of the live fields => untouched and the startup sync may update it.
    builtin_checksum text,
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW()
);
ALTER TABLE agent.skill OWNER TO rurdesk;

CREATE TABLE agent.project_skill (
    id_project bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    id_skill   bigint NOT NULL REFERENCES agent.skill (id_skill) ON DELETE CASCADE,
    stage      character varying(30) NOT NULL CHECK (stage IN ('brainstorming', 'design', 'implementation_plan', 'implementation')),
    PRIMARY KEY (id_project, id_skill, stage)
);
ALTER TABLE agent.project_skill OWNER TO rurdesk;

CREATE INDEX idx_project_skill_project_stage ON agent.project_skill (id_project, stage);

-- +goose Down
DROP TABLE agent.project_skill;
DROP TABLE agent.skill;
