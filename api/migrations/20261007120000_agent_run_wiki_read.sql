-- +goose Up
CREATE TABLE agent.run_wiki_read (
    id_read    bigserial PRIMARY KEY,
    id_run     bigint      NOT NULL REFERENCES agent.run (id_run) ON DELETE CASCADE,
    id_task    bigint      REFERENCES agent.task (id_task) ON DELETE SET NULL,
    stage      text        NOT NULL,
    id_call    uuid        NOT NULL,
    source     text        NOT NULL,
    id_page    bigint      REFERENCES wiki.page (id_page) ON DELETE SET NULL,
    space_kind text,
    slug       text,
    title      text,
    version_no int,
    tokens     int         NOT NULL DEFAULT 0,
    query      text,
    pages      jsonb,
    create_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT run_wiki_read_source_chk
        CHECK (source IN ('prompt_always', 'prompt_linked', 'prompt_index', 'mcp_get', 'mcp_search')),
    CONSTRAINT run_wiki_read_page_chk
        CHECK (source IN ('mcp_search', 'prompt_index') OR (slug IS NOT NULL AND version_no IS NOT NULL)),
    CONSTRAINT run_wiki_read_index_chk
        CHECK ((source = 'prompt_index') = (pages IS NOT NULL))
);
ALTER TABLE agent.run_wiki_read OWNER TO rurdesk;

CREATE INDEX idx_run_wiki_read_run ON agent.run_wiki_read (id_run, create_at, id_read);
CREATE UNIQUE INDEX uq_run_wiki_read_prompt_page ON agent.run_wiki_read (id_task, source, id_page)
    WHERE source IN ('prompt_always', 'prompt_linked');
CREATE UNIQUE INDEX uq_run_wiki_read_prompt_index ON agent.run_wiki_read (id_task)
    WHERE source = 'prompt_index';

-- +goose Down
DROP TABLE agent.run_wiki_read;
