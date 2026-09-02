-- +goose Up
CREATE TABLE agent.task_thinking (
    id_task     bigint      NOT NULL REFERENCES agent.task(id_task) ON DELETE CASCADE,
    seq         int         NOT NULL,
    event_index int         NOT NULL,
    kind        text        NOT NULL,
    tool        text        NOT NULL DEFAULT '',
    text        text        NOT NULL,
    event_at    bigint      NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id_task, seq, event_index)
);

ALTER TABLE agent.task ADD COLUMN thinking_blob bytea;
ALTER TABLE agent.task ADD COLUMN thinking_tail text;
ALTER TABLE agent.task RENAME COLUMN id_output_message TO id_result_message;
ALTER TABLE agent.task RENAME CONSTRAINT task_id_output_message_fkey TO task_id_result_message_fkey;

-- +goose Down
ALTER TABLE agent.task RENAME CONSTRAINT task_id_result_message_fkey TO task_id_output_message_fkey;
ALTER TABLE agent.task RENAME COLUMN id_result_message TO id_output_message;
ALTER TABLE agent.task DROP COLUMN thinking_tail;
ALTER TABLE agent.task DROP COLUMN thinking_blob;
DROP TABLE agent.task_thinking;
