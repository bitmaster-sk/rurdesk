-- +goose Up
-- Support HasAgentActivity (admin delete guard) and FK checks on users.user deletes.
CREATE INDEX run_id_user_bot_idx ON agent.run(id_user_bot);
CREATE INDEX task_id_user_bot_idx ON agent.task(id_user_bot);
CREATE INDEX run_event_id_user_idx ON agent.run_event(id_user);

-- +goose Down
DROP INDEX agent.run_event_id_user_idx;
DROP INDEX agent.task_id_user_bot_idx;
DROP INDEX agent.run_id_user_bot_idx;