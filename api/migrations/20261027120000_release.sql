-- +goose Up
CREATE TABLE issues.release (
    id_release      bigserial PRIMARY KEY,
    id_project      bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    id_tag          bigint REFERENCES issues.tag (id_tag) ON DELETE SET NULL,
    name            character varying(40) NOT NULL,
    issue_count     int NOT NULL DEFAULT 0,
    create_at       timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    create_by       bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    status          character varying(10) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'released')),
    notes           text NOT NULL DEFAULT '',
    recreate_tag    boolean NOT NULL DEFAULT true,
    open_issues     character varying(10) NOT NULL DEFAULT 'move' CHECK (open_issues IN ('move', 'keep')),
    released_at     timestamp WITHOUT TIME ZONE,
    released_by     bigint REFERENCES users.user (id_user) ON DELETE SET NULL
);
ALTER TABLE issues.release OWNER TO rurdesk;

CREATE UNIQUE INDEX release_project_name_uidx ON issues.release (id_project, lower(name));
CREATE UNIQUE INDEX release_draft_tag_uidx ON issues.release (id_tag) WHERE status = 'draft';

ALTER TABLE agent.run
    ADD COLUMN kind character varying(30) NOT NULL DEFAULT 'issue' CHECK (kind IN ('issue', 'release_notes')),
    ADD COLUMN id_release bigint REFERENCES issues.release (id_release) ON DELETE CASCADE,
    ALTER COLUMN id_issue DROP NOT NULL,
    ADD CONSTRAINT run_subject_check CHECK (
        (kind = 'issue' AND id_issue IS NOT NULL AND id_release IS NULL)
        OR (kind = 'release_notes' AND id_release IS NOT NULL AND id_issue IS NULL)
    );

CREATE INDEX run_id_release_idx ON agent.run (id_release);
CREATE UNIQUE INDEX agent_run_one_active_per_release ON agent.run (id_release)
    WHERE phase IN ('queued', 'in_progress', 'awaiting_input', 'awaiting_approval', 'pr_open');

ALTER TABLE agent.project_skill DROP CONSTRAINT project_skill_stage_check;

-- +goose Down
DELETE FROM agent.project_skill
WHERE stage NOT IN ('brainstorming', 'design', 'implementation_plan', 'implementation');
ALTER TABLE agent.project_skill
    ADD CONSTRAINT project_skill_stage_check CHECK (stage IN ('brainstorming', 'design', 'implementation_plan', 'implementation'));
DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_issue IS NULL);
DELETE FROM agent.run WHERE id_issue IS NULL;
ALTER TABLE agent.run
    DROP CONSTRAINT run_subject_check,
    DROP COLUMN id_release,
    DROP COLUMN kind,
    ALTER COLUMN id_issue SET NOT NULL;
DROP TABLE issues.release;
