-- +goose Up
CREATE TABLE issues.sprint (
    id_sprint  bigserial PRIMARY KEY,
    id_project bigint NOT NULL REFERENCES projects.project(id_project) ON DELETE CASCADE,
    name       text NOT NULL,
    start_at   timestamp NOT NULL,
    end_at     timestamp NOT NULL,
    state      text NOT NULL DEFAULT 'planned',
    create_at  timestamp NOT NULL DEFAULT (now() at time zone 'utc'),
    create_by  bigint,
    update_at  timestamp NOT NULL DEFAULT (now() at time zone 'utc'),
    update_by  bigint,
    CONSTRAINT sprint_state_chk CHECK (state IN ('planned', 'closed'))
);
CREATE INDEX idx_sprint_project ON issues.sprint(id_project);

ALTER TABLE issues.issue
    ADD COLUMN id_sprint bigint REFERENCES issues.sprint(id_sprint) ON DELETE SET NULL,
    ADD COLUMN points int,
    ADD COLUMN carryover_count int NOT NULL DEFAULT 0;
CREATE INDEX idx_issue_sprint ON issues.issue(id_sprint);

-- +goose Down
ALTER TABLE issues.issue DROP COLUMN carryover_count, DROP COLUMN points, DROP COLUMN id_sprint;
DROP TABLE issues.sprint;
