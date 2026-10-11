-- +goose Up
ALTER TABLE files.attachment ADD COLUMN id_space bigint REFERENCES wiki.space (id_space) ON DELETE CASCADE;
ALTER TABLE files.attachment DROP CONSTRAINT attachment_scope_chk;
ALTER TABLE files.attachment ADD CONSTRAINT attachment_scope_chk
    CHECK (num_nonnulls(id_issue, id_project, id_team, id_user_to, id_space) = 1);

CREATE INDEX idx_attachment_space ON files.attachment (id_space) WHERE id_space IS NOT NULL;

CREATE TABLE wiki.page_attachment (
    id_page       bigint NOT NULL REFERENCES wiki.page (id_page) ON DELETE CASCADE,
    id_attachment uuid NOT NULL REFERENCES files.attachment (id_attachment) ON DELETE CASCADE,
    PRIMARY KEY (id_page, id_attachment)
);
ALTER TABLE wiki.page_attachment OWNER TO rurdesk;

CREATE INDEX idx_page_attachment_attachment ON wiki.page_attachment (id_attachment);

CREATE TABLE issues.issue_attachment (
    id_issue      bigint NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_attachment uuid NOT NULL REFERENCES files.attachment (id_attachment) ON DELETE CASCADE,
    PRIMARY KEY (id_issue, id_attachment)
);
ALTER TABLE issues.issue_attachment OWNER TO rurdesk;

CREATE INDEX idx_issue_attachment_attachment ON issues.issue_attachment (id_attachment);

-- +goose Down
DROP TABLE issues.issue_attachment;
DROP TABLE wiki.page_attachment;
DELETE FROM files.attachment WHERE id_space IS NOT NULL;
ALTER TABLE files.attachment DROP CONSTRAINT attachment_scope_chk;
ALTER TABLE files.attachment DROP COLUMN id_space;
ALTER TABLE files.attachment ADD CONSTRAINT attachment_scope_chk
    CHECK (num_nonnulls(id_issue, id_project, id_team, id_user_to) = 1);
