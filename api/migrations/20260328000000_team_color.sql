-- +goose Up
ALTER TABLE users.team ADD COLUMN color VARCHAR(32) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users.team DROP COLUMN color;
