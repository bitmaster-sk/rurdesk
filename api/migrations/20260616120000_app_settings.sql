-- +goose Up
CREATE TABLE projects.app_settings (
    key        VARCHAR(120) PRIMARY KEY,
    value      JSONB        NOT NULL,
    update_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

INSERT INTO projects.app_settings (key, value) VALUES
    ('pagination.table_page_size',         '50'),
    ('pagination.kanban_page_size',        '20'),
    ('pagination.gantt_backlog_page_size', '30');

-- +goose Down
DROP TABLE IF EXISTS projects.app_settings;
