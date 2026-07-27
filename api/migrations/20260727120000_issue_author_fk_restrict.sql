-- +goose Up
-- create_by/update_by/created_by are NOT NULL, so ON DELETE SET NULL could never
-- succeed: deleting a user with authoring history aborted with a not-null
-- violation. RESTRICT states the rule; the delete handler answers 409.
-- issues.issue.assigned_to stays SET NULL — that column is nullable.
ALTER TABLE issues.issue
    DROP CONSTRAINT issue_create_by_fkey,
    ADD CONSTRAINT issue_create_by_fkey
        FOREIGN KEY (create_by) REFERENCES users.user (id_user) ON DELETE RESTRICT,
    DROP CONSTRAINT issue_update_by_fkey,
    ADD CONSTRAINT issue_update_by_fkey
        FOREIGN KEY (update_by) REFERENCES users.user (id_user) ON DELETE RESTRICT;

ALTER TABLE issues.issue_relation
    DROP CONSTRAINT issue_relation_created_by_fkey,
    ADD CONSTRAINT issue_relation_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users.user (id_user) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE issues.issue
    DROP CONSTRAINT issue_create_by_fkey,
    ADD CONSTRAINT issue_create_by_fkey
        FOREIGN KEY (create_by) REFERENCES users.user (id_user) ON DELETE SET NULL,
    DROP CONSTRAINT issue_update_by_fkey,
    ADD CONSTRAINT issue_update_by_fkey
        FOREIGN KEY (update_by) REFERENCES users.user (id_user) ON DELETE SET NULL;

ALTER TABLE issues.issue_relation
    DROP CONSTRAINT issue_relation_created_by_fkey,
    ADD CONSTRAINT issue_relation_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users.user (id_user) ON DELETE SET NULL;
