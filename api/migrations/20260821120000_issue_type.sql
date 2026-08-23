-- +goose Up
CREATE TABLE issues.issue_type (
    id_issue_type   bigserial PRIMARY KEY,
    id_project      bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    name            character varying(20) NOT NULL,
    protected       boolean NOT NULL DEFAULT false,
    order_rank      int NOT NULL DEFAULT 0
);
ALTER TABLE issues.issue_type OWNER TO rurdesk;

CREATE INDEX idx_issue_type_project_order ON issues.issue_type (id_project, order_rank);

ALTER TABLE issues.issue
    ADD COLUMN id_issue_type bigint REFERENCES issues.issue_type (id_issue_type) ON DELETE SET NULL;

CREATE INDEX idx_issue_id_issue_type ON issues.issue (id_issue_type);

ALTER TABLE projects.project
    ADD COLUMN id_issue_type_default bigint REFERENCES issues.issue_type (id_issue_type) ON DELETE SET NULL;

INSERT INTO issues.issue_type (id_project, name, order_rank)
SELECT p.id_project, seed.name, seed.order_rank
FROM projects.project p
CROSS JOIN (VALUES ('Bug', 1), ('Feature', 2), ('Task', 3)) AS seed(name, order_rank);

-- +goose Down
ALTER TABLE projects.project DROP COLUMN id_issue_type_default;
DROP INDEX IF EXISTS issues.idx_issue_id_issue_type;
ALTER TABLE issues.issue DROP COLUMN id_issue_type;
DROP TABLE issues.issue_type;
