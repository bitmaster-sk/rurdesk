-- +goose Up
CREATE TABLE issues.tag (
    id_tag          bigserial PRIMARY KEY,
    id_project      bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    name            character varying(40) NOT NULL,
    color           character varying(20) NOT NULL,
    order_rank      int NOT NULL DEFAULT 0
);
ALTER TABLE issues.tag OWNER TO rurdesk;

CREATE UNIQUE INDEX tag_project_name_uidx ON issues.tag (id_project, lower(name));
CREATE INDEX idx_tag_project_order ON issues.tag (id_project, order_rank);

CREATE TABLE issues.issue_tag (
    id_issue        bigint NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_tag          bigint NOT NULL REFERENCES issues.tag (id_tag) ON DELETE CASCADE,
    PRIMARY KEY (id_issue, id_tag)
);
ALTER TABLE issues.issue_tag OWNER TO rurdesk;

CREATE INDEX idx_issue_tag_id_tag ON issues.issue_tag (id_tag);

-- +goose Down
DROP TABLE issues.issue_tag;
DROP TABLE issues.tag;
