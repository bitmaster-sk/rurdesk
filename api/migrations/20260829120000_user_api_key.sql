-- +goose Up
-- Denormalises users.user.is_bot: a partial unique index cannot reach another
-- table. Defaults true so existing rows, all of which predate personal tokens,
-- are backfilled as agent keys.
ALTER TABLE users.api_key ADD COLUMN is_agent BOOLEAN NOT NULL DEFAULT true;

-- Agent keys keep their one-per-user guarantee (AdminController.CreateAgentApiKey
-- detects the 409 by catching this index's violation); personal tokens are exempt.
DROP INDEX IF EXISTS users.api_key_id_user_key;
CREATE UNIQUE INDEX api_key_agent_one_per_user ON users.api_key(id_user) WHERE is_agent;
CREATE INDEX api_key_id_user_idx ON users.api_key(id_user);

-- +goose Down
DELETE FROM users.api_key a
USING users.api_key b
WHERE a.id_user = b.id_user
  AND a.id_api_key < b.id_api_key;

DROP INDEX IF EXISTS users.api_key_agent_one_per_user;
DROP INDEX IF EXISTS users.api_key_id_user_idx;
CREATE UNIQUE INDEX api_key_id_user_key ON users.api_key(id_user);
ALTER TABLE users.api_key DROP COLUMN is_agent;
