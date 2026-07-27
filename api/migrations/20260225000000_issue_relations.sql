-- +goose Up

CREATE TABLE issues.issue_relation (
    id_issue_relation  BIGSERIAL PRIMARY KEY,
    id_project         BIGINT NOT NULL,
    id_issue_from      BIGINT NOT NULL REFERENCES issues.issue(id_issue) ON DELETE CASCADE,
    id_issue_to        BIGINT NOT NULL REFERENCES issues.issue(id_issue) ON DELETE CASCADE,
    relation_type      VARCHAR(20) NOT NULL,
    relation_sub_type  VARCHAR(20) NULL,
    lag_minutes        BIGINT NULL,
    created_at         TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    created_by         BIGINT NOT NULL REFERENCES users.user(id_user) ON DELETE SET NULL,

    CONSTRAINT chk_no_self_relation CHECK (id_issue_from <> id_issue_to),
    CONSTRAINT chk_sub_type CHECK (
        (relation_type = 'schedule' AND relation_sub_type IS NOT NULL)
        OR (relation_type <> 'schedule' AND relation_sub_type IS NULL)
    ),
    CONSTRAINT chk_lag_only_on_schedule CHECK (
        lag_minutes IS NULL OR relation_type = 'schedule'
    )
);
ALTER TABLE issues.issue_relation OWNER TO rurdesk;

CREATE UNIQUE INDEX uq_relation_directional
    ON issues.issue_relation (id_issue_from, id_issue_to, relation_type, COALESCE(relation_sub_type, ''))
    WHERE relation_type IN ('hierarchy', 'schedule');

CREATE UNIQUE INDEX uq_relation_nondirectional
    ON issues.issue_relation (id_issue_from, id_issue_to, relation_type)
    WHERE relation_type IN ('duplicates', 'relates_to');

CREATE UNIQUE INDEX uq_hierarchy_single_parent
    ON issues.issue_relation (id_issue_to)
    WHERE relation_type = 'hierarchy';

CREATE INDEX idx_relation_project_from ON issues.issue_relation (id_project, id_issue_from);
CREATE INDEX idx_relation_project_to   ON issues.issue_relation (id_project, id_issue_to);
CREATE INDEX idx_relation_from_type    ON issues.issue_relation (id_issue_from, relation_type);
CREATE INDEX idx_relation_to_type      ON issues.issue_relation (id_issue_to, relation_type);

-- +goose Down

DROP TABLE IF EXISTS issues.issue_relation;
