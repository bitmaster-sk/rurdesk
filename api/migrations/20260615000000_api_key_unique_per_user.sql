-- +goose Up
-- Collapse API keys to one-per-bot (mirrors agent.bot_gateway's 1:1 model).
-- Dedupe first so the unique index can be created on existing data: keep the
-- most recently created key per user, drop the rest.
DELETE FROM users.api_key a
USING users.api_key b
WHERE a.id_user = b.id_user
  AND a.id_api_key < b.id_api_key;

DROP INDEX IF EXISTS users.api_key_id_user_idx;
CREATE UNIQUE INDEX api_key_id_user_key ON users.api_key(id_user);

-- +goose Down
DROP INDEX IF EXISTS users.api_key_id_user_key;
CREATE INDEX api_key_id_user_idx ON users.api_key(id_user);
