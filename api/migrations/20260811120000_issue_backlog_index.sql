-- +goose NO TRANSACTION
-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS issues.idx_issue_backlog_project;
CREATE INDEX CONCURRENTLY idx_issue_backlog_project
    ON issues.issue (id_project) WHERE id_sprint IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS issues.idx_issue_backlog_project;
