-- +goose Up

-- Highest id_message of the issue conversation the task was dispatched with; issue comments above it
-- are feedback the agent has not seen yet.
ALTER TABLE agent.task ADD COLUMN id_message_seen BIGINT;

-- Tasks dispatched before this column saw the conversation as it was when they started.
UPDATE agent.task t
SET id_message_seen = seen.id_message
FROM (
    SELECT t2.id_task, MAX(ism.id_message) AS id_message
    FROM agent.task t2
    INNER JOIN agent.run r ON r.id_run = t2.id_run
    INNER JOIN messages.issue_message ism ON ism.id_issue_to = r.id_issue
    INNER JOIN messages.message msg ON msg.id_message = ism.id_message
    WHERE t2.started_at IS NOT NULL
      AND msg.created_at <= t2.started_at
    GROUP BY t2.id_task
) seen
WHERE seen.id_task = t.id_task;

-- +goose Down

ALTER TABLE agent.task DROP COLUMN id_message_seen;
