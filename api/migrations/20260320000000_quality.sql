-- +goose Up
CREATE TABLE issues.issue_quality (
    id_issue      BIGINT PRIMARY KEY REFERENCES issues.issue(id_issue) ON DELETE CASCADE,
    score         SMALLINT NOT NULL CHECK (score BETWEEN 0 AND 100),
    report        JSONB NOT NULL,
    content_hash  CHAR(64) NOT NULL,
    checked_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    checked_by    BIGINT NOT NULL REFERENCES users.user(id_user)
);

-- +goose Down
DROP TABLE IF EXISTS issues.issue_quality;
