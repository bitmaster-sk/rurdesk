-- +goose Up
ALTER TABLE users.user ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX user_is_admin_idx ON users.user(is_admin) WHERE is_admin = TRUE;

-- +goose Down
DROP INDEX IF EXISTS users.user_is_admin_idx;
ALTER TABLE users.user DROP COLUMN is_admin;
