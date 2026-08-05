-- +goose Up
CREATE TABLE issues.saved_view (
    id_saved_view bigserial PRIMARY KEY,
    id_project    bigint NOT NULL REFERENCES projects.project(id_project) ON DELETE CASCADE,
    name          text NOT NULL,
    view_type     text NOT NULL,
    config        jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_shared     boolean NOT NULL DEFAULT false,
    create_at     timestamp NOT NULL DEFAULT (now() at time zone 'utc'),
    create_by     bigint NOT NULL REFERENCES users.user(id_user) ON DELETE CASCADE,
    update_at     timestamp NOT NULL DEFAULT (now() at time zone 'utc'),
    update_by     bigint REFERENCES users.user(id_user) ON DELETE SET NULL,
    CONSTRAINT saved_view_type_chk CHECK (view_type IN ('table', 'kanban', 'calendar', 'gantt'))
);
CREATE INDEX idx_saved_view_project ON issues.saved_view(id_project);

-- +goose Down
DROP TABLE issues.saved_view;
