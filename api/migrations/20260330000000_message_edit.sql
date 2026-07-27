-- +goose Up
ALTER TABLE messages.message ADD COLUMN updated_at TIMESTAMP WITHOUT TIME ZONE NULL;

-- +goose Down
ALTER TABLE messages.message DROP COLUMN updated_at;
