-- +goose Up
CREATE TABLE issues.issue_history (
    id_issue_history bigserial PRIMARY KEY,
    id_issue         bigint NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_change        uuid NOT NULL,
    field            character varying(40) NOT NULL,
    old_value        jsonb NULL,
    new_value        jsonb NULL,
    id_user          bigint NULL REFERENCES users.user (id_user) ON DELETE SET NULL,
    source           character varying(10) NOT NULL CHECK (source IN ('user', 'system')),
    create_at        timestamp without time zone NOT NULL DEFAULT (now() at time zone 'utc')
);
ALTER TABLE issues.issue_history OWNER TO rurdesk;

CREATE INDEX idx_issue_history_issue ON issues.issue_history (id_issue, create_at);
CREATE INDEX idx_issue_history_issue_field ON issues.issue_history (id_issue, field, id_issue_history DESC);

-- +goose Down
DROP TABLE issues.issue_history;
