-- +goose Up
CREATE SCHEMA IF NOT EXISTS files;
ALTER SCHEMA files OWNER TO rurdesk;

CREATE TABLE files.attachment (
    id_attachment uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_name     character varying(255) NOT NULL,
    mime_type     character varying(255) NOT NULL,
    size          bigint NOT NULL,
    id_issue      bigint REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_project    bigint REFERENCES projects.project (id_project) ON DELETE CASCADE,
    id_team       bigint REFERENCES users.team (id_team) ON DELETE CASCADE,
    id_user_to    bigint REFERENCES users.user (id_user) ON DELETE CASCADE,
    create_by     bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    create_at     timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    CONSTRAINT attachment_scope_chk CHECK (num_nonnulls(id_issue, id_project, id_team, id_user_to) = 1)
);
ALTER TABLE files.attachment OWNER TO rurdesk;

CREATE INDEX idx_attachment_issue ON files.attachment (id_issue) WHERE id_issue IS NOT NULL;
CREATE INDEX idx_attachment_project ON files.attachment (id_project) WHERE id_project IS NOT NULL;
CREATE INDEX idx_attachment_team ON files.attachment (id_team) WHERE id_team IS NOT NULL;
CREATE INDEX idx_attachment_user_to ON files.attachment (id_user_to) WHERE id_user_to IS NOT NULL;
CREATE INDEX idx_attachment_create_at ON files.attachment (create_at);

CREATE TABLE files.attachment_content (
    id_attachment uuid PRIMARY KEY REFERENCES files.attachment (id_attachment) ON DELETE CASCADE,
    data          bytea NOT NULL
);
ALTER TABLE files.attachment_content OWNER TO rurdesk;

CREATE TABLE messages.message_attachment (
    id_message    bigint NOT NULL REFERENCES messages.message (id_message) ON DELETE CASCADE,
    id_attachment uuid NOT NULL REFERENCES files.attachment (id_attachment) ON DELETE CASCADE,
    PRIMARY KEY (id_message, id_attachment)
);
ALTER TABLE messages.message_attachment OWNER TO rurdesk;

CREATE INDEX idx_message_attachment_attachment ON messages.message_attachment (id_attachment);

-- +goose Down
DROP TABLE messages.message_attachment;
DROP SCHEMA files CASCADE;
