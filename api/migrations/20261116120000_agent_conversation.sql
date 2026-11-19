-- +goose Up
CREATE TABLE agent.conversation (
    id_conversation       bigserial PRIMARY KEY,
    id_user               bigint NOT NULL REFERENCES users.user (id_user) ON DELETE CASCADE,
    id_user_agent         bigint NOT NULL REFERENCES users.user (id_user),
    id_project            bigint NULL REFERENCES projects.project (id_project) ON DELETE SET NULL,
    id_run                bigint NULL REFERENCES agent.run (id_run) ON DELETE SET NULL,
    title                 text NOT NULL DEFAULT '',
    head_commit           text NULL,
    is_locked             boolean NOT NULL DEFAULT false,
    -- A task draft the owner asked for: next_turn = the next turn to start is the
    -- draft; after_turn = the queued or running reply turn goes first.
    task_draft_request    text NULL CHECK (task_draft_request IN ('next_turn', 'after_turn')),
    read_at               timestamptz NULL,
    last_agent_message_at timestamptz NULL,
    -- Set while the owner has the conversation pinned; pinned conversations list first.
    pinned_at             timestamptz NULL,
    -- The model the agent answers with; NULL means the gateway's default model.
    model                 text NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE agent.conversation OWNER TO rurdesk;

CREATE INDEX conversation_user_updated_idx ON agent.conversation (id_user, updated_at DESC);
-- One chat run per conversation: the run token of a chat turn resolves to exactly one conversation.
CREATE UNIQUE INDEX conversation_id_run_uidx ON agent.conversation (id_run) WHERE id_run IS NOT NULL;

-- Context projects only; the main project is conversation.id_project.
CREATE TABLE agent.conversation_project (
    id_conversation bigint NOT NULL REFERENCES agent.conversation (id_conversation) ON DELETE CASCADE,
    id_project      bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    PRIMARY KEY (id_conversation, id_project)
);
ALTER TABLE agent.conversation_project OWNER TO rurdesk;

CREATE TABLE agent.conversation_message (
    id_message      bigserial PRIMARY KEY,
    id_conversation bigint NOT NULL REFERENCES agent.conversation (id_conversation) ON DELETE CASCADE,
    author          text NOT NULL CHECK (author IN ('user', 'agent', 'system')),
    id_user         bigint NULL,
    kind            text NOT NULL DEFAULT 'text' CHECK (kind IN ('text', 'task_draft', 'error', 'code_refreshed')),
    body            text NOT NULL,
    id_task         bigint NULL REFERENCES agent.task (id_task) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE agent.conversation_message OWNER TO rurdesk;

CREATE INDEX conversation_message_conversation_idx ON agent.conversation_message (id_conversation, id_message);

-- The agent answer the pending task draft quotes; set together with task_draft_request.
ALTER TABLE agent.conversation
    ADD COLUMN task_draft_message_id bigint NULL REFERENCES agent.conversation_message (id_message) ON DELETE SET NULL;

ALTER TABLE agent.run
    DROP CONSTRAINT run_kind_check,
    ADD CONSTRAINT run_kind_check CHECK (kind IN ('issue', 'release_notes', 'chat')),
    DROP CONSTRAINT run_subject_check,
    ADD CONSTRAINT run_subject_check CHECK (
        (kind = 'issue' AND id_issue IS NOT NULL AND id_release IS NULL)
        OR (kind = 'release_notes' AND id_release IS NOT NULL AND id_issue IS NULL)
        OR (kind = 'chat' AND id_issue IS NULL AND id_release IS NULL)
    ),
    ALTER COLUMN id_project DROP NOT NULL,
    ADD CONSTRAINT run_project_check CHECK (kind = 'chat' OR id_project IS NOT NULL),
    ADD COLUMN run_token_hash bytea NULL,
    ADD COLUMN run_token_expires_at timestamptz NULL;

CREATE UNIQUE INDEX agent_run_run_token_hash_uidx ON agent.run (run_token_hash);

-- Mode of a chat turn, fixed when the turn starts and kept by its re-queued attempts.
ALTER TABLE agent.task ADD COLUMN chat_mode text NULL CHECK (chat_mode IN ('reply', 'task_draft'));

-- +goose Down
ALTER TABLE agent.conversation DROP COLUMN task_draft_message_id;
DROP INDEX IF EXISTS agent.conversation_id_run_uidx;
DROP TABLE agent.conversation_message;
DROP TABLE agent.conversation_project;
DROP TABLE agent.conversation;

DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE kind = 'chat');
DELETE FROM agent.run WHERE kind = 'chat';
ALTER TABLE agent.task DROP COLUMN IF EXISTS chat_mode;

DROP INDEX agent.agent_run_run_token_hash_uidx;
ALTER TABLE agent.run
    DROP COLUMN run_token_expires_at,
    DROP COLUMN run_token_hash,
    DROP CONSTRAINT run_project_check,
    ALTER COLUMN id_project SET NOT NULL,
    DROP CONSTRAINT run_subject_check,
    ADD CONSTRAINT run_subject_check CHECK (
        (kind = 'issue' AND id_issue IS NOT NULL AND id_release IS NULL)
        OR (kind = 'release_notes' AND id_release IS NOT NULL AND id_issue IS NULL)
    ),
    DROP CONSTRAINT run_kind_check,
    ADD CONSTRAINT run_kind_check CHECK (kind IN ('issue', 'release_notes'));
