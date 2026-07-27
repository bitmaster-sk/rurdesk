-- +goose Up

CREATE TABLE issues.issue_participant (
    id_issue_participant     BIGSERIAL PRIMARY KEY,
    id_issue                 BIGINT NOT NULL REFERENCES issues.issue(id_issue) ON DELETE CASCADE,
    id_user                  BIGINT NOT NULL REFERENCES users.user(id_user) ON DELETE CASCADE,
    source                   VARCHAR(20) NOT NULL, -- creator | assignee | comment | mention | manual
    has_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    added_at                 TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    added_by                 BIGINT NULL REFERENCES users.user(id_user) ON DELETE SET NULL,

    CONSTRAINT uq_issue_participant UNIQUE (id_issue, id_user)
);
ALTER TABLE issues.issue_participant OWNER TO rurdesk;

CREATE INDEX idx_participant_issue ON issues.issue_participant (id_issue);
CREATE INDEX idx_participant_user  ON issues.issue_participant (id_user);

-- +goose Down

DROP TABLE IF EXISTS issues.issue_participant;
