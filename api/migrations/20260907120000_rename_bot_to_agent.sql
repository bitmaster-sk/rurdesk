-- +goose Up
-- After this, users.user.is_agent and users.api_key.is_agent are two
-- identically named columns in different tables; the api_key one stays a
-- denormalisation of the user one.
ALTER TABLE users.user RENAME COLUMN is_bot TO is_agent;
ALTER INDEX users.user_is_bot_idx RENAME TO user_is_agent_idx;

ALTER TABLE agent.bot_gateway RENAME COLUMN id_bot_gateway TO id_gateway;
ALTER TABLE agent.bot_gateway RENAME COLUMN id_user_bot TO id_user_agent;
ALTER TABLE agent.bot_gateway RENAME CONSTRAINT bot_gateway_pkey TO gateway_pkey;
ALTER TABLE agent.bot_gateway RENAME CONSTRAINT bot_gateway_id_user_bot_fkey TO gateway_id_user_agent_fkey;
ALTER INDEX agent.bot_gateway_id_user_bot_idx RENAME TO gateway_id_user_agent_idx;
ALTER SEQUENCE agent.bot_gateway_id_bot_gateway_seq RENAME TO gateway_id_gateway_seq;
ALTER TABLE agent.bot_gateway RENAME TO gateway;

ALTER TABLE agent.run RENAME COLUMN id_user_bot TO id_user_agent;
ALTER TABLE agent.run RENAME CONSTRAINT run_id_user_bot_fkey TO run_id_user_agent_fkey;
ALTER INDEX agent.run_id_user_bot_idx RENAME TO run_id_user_agent_idx;
ALTER INDEX agent.run_id_user_bot_phase_active_idx RENAME TO run_id_user_agent_phase_active_idx;

ALTER TABLE agent.task RENAME COLUMN id_user_bot TO id_user_agent;
ALTER TABLE agent.task RENAME CONSTRAINT task_id_user_bot_fkey TO task_id_user_agent_fkey;
ALTER INDEX agent.task_id_user_bot_idx RENAME TO task_id_user_agent_idx;

-- +goose Down
ALTER INDEX agent.task_id_user_agent_idx RENAME TO task_id_user_bot_idx;
ALTER TABLE agent.task RENAME CONSTRAINT task_id_user_agent_fkey TO task_id_user_bot_fkey;
ALTER TABLE agent.task RENAME COLUMN id_user_agent TO id_user_bot;

ALTER INDEX agent.run_id_user_agent_phase_active_idx RENAME TO run_id_user_bot_phase_active_idx;
ALTER INDEX agent.run_id_user_agent_idx RENAME TO run_id_user_bot_idx;
ALTER TABLE agent.run RENAME CONSTRAINT run_id_user_agent_fkey TO run_id_user_bot_fkey;
ALTER TABLE agent.run RENAME COLUMN id_user_agent TO id_user_bot;

ALTER TABLE agent.gateway RENAME TO bot_gateway;
ALTER SEQUENCE agent.gateway_id_gateway_seq RENAME TO bot_gateway_id_bot_gateway_seq;
ALTER INDEX agent.gateway_id_user_agent_idx RENAME TO bot_gateway_id_user_bot_idx;
ALTER TABLE agent.bot_gateway RENAME CONSTRAINT gateway_id_user_agent_fkey TO bot_gateway_id_user_bot_fkey;
ALTER TABLE agent.bot_gateway RENAME CONSTRAINT gateway_pkey TO bot_gateway_pkey;
ALTER TABLE agent.bot_gateway RENAME COLUMN id_user_agent TO id_user_bot;
ALTER TABLE agent.bot_gateway RENAME COLUMN id_gateway TO id_bot_gateway;

ALTER INDEX users.user_is_agent_idx RENAME TO user_is_bot_idx;
ALTER TABLE users.user RENAME COLUMN is_agent TO is_bot;
