-- +goose Up
ALTER TABLE projects.project ADD COLUMN color VARCHAR(32) NOT NULL DEFAULT '#6b7280';

-- +goose Down
ALTER TABLE projects.project DROP COLUMN color;
