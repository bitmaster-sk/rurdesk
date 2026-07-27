-- +goose Up
ALTER TABLE issues.issue
    ADD COLUMN id_git_integration BIGINT NULL
        REFERENCES projects.git_integration(id_git_integration) ON DELETE SET NULL,
    ADD COLUMN mr_id VARCHAR(50) NULL,
    ADD CONSTRAINT issue_mr_link_consistency
        CHECK ((id_git_integration IS NULL) = (mr_id IS NULL));
CREATE INDEX issue_id_git_integration_idx
    ON issues.issue(id_git_integration)
    WHERE id_git_integration IS NOT NULL;

-- +goose Down
ALTER TABLE issues.issue
    DROP CONSTRAINT issue_mr_link_consistency,
    DROP COLUMN mr_id,
    DROP COLUMN id_git_integration;
