-- +goose Up
ALTER TABLE users.user ADD COLUMN is_bot BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX ON users.user(is_bot) WHERE is_bot = TRUE;

-- +goose Down
ALTER TABLE users.user DROP COLUMN is_bot;
