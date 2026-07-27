-- +goose Up
CREATE INDEX issues_issue_fts_idx ON issues.issue
    USING gin(to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '')));

ALTER TABLE issues.issue ADD COLUMN idempotency_key VARCHAR(80);
CREATE UNIQUE INDEX issues_issue_idempotency_key_idx
    ON issues.issue(create_by, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS issues.issues_issue_idempotency_key_idx;
ALTER TABLE issues.issue DROP COLUMN IF EXISTS idempotency_key;
DROP INDEX IF EXISTS issues.issues_issue_fts_idx;
