-- +goose Up
ALTER TABLE users.user ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;
CREATE INDEX user_is_active_idx ON users.user(is_active) WHERE is_active = FALSE;

-- +goose Down
DROP INDEX IF EXISTS users.user_is_active_idx;
ALTER TABLE users.user DROP COLUMN is_active;
