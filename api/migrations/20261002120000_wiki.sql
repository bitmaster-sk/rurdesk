-- +goose Up
CREATE SCHEMA IF NOT EXISTS wiki;
ALTER SCHEMA wiki OWNER TO rurdesk;

CREATE TABLE wiki.space (
    id_space   bigserial PRIMARY KEY,
    kind       text NOT NULL,
    id_project bigint REFERENCES projects.project (id_project) ON DELETE CASCADE,
    CONSTRAINT space_kind_chk CHECK (kind IN ('instance', 'project')),
    CONSTRAINT space_project_chk CHECK ((kind = 'instance') = (id_project IS NULL)),
    CONSTRAINT space_project_uq UNIQUE (id_project)
);
ALTER TABLE wiki.space OWNER TO rurdesk;

CREATE UNIQUE INDEX uq_space_instance ON wiki.space (kind) WHERE kind = 'instance';

INSERT INTO wiki.space (kind) VALUES ('instance');

CREATE TABLE wiki.page (
    id_page         bigserial PRIMARY KEY,
    id_space        bigint NOT NULL REFERENCES wiki.space (id_space) ON DELETE CASCADE,
    id_parent       bigint REFERENCES wiki.page (id_page) ON DELETE SET NULL,
    slug            character varying(120) NOT NULL,
    title           character varying(200) NOT NULL,
    summary         character varying(500) NOT NULL DEFAULT '',
    body            text NOT NULL DEFAULT '',
    agent_access    text NOT NULL DEFAULT 'on_demand',
    rank            text NOT NULL,
    version_no      int NOT NULL DEFAULT 1,
    search_vector   tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(summary, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(body, '')), 'C')
    ) STORED,
    deleted_at      timestamp WITHOUT TIME ZONE,
    deleted_by      bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    id_deleted_root bigint REFERENCES wiki.page (id_page) ON DELETE SET NULL,
    create_at       timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    update_at       timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    create_by       bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    update_by       bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    CONSTRAINT page_agent_access_chk CHECK (agent_access IN ('always', 'on_demand', 'hidden')),
    CONSTRAINT page_slug_chk CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
ALTER TABLE wiki.page OWNER TO rurdesk;

CREATE UNIQUE INDEX uq_page_slug_live ON wiki.page (id_space, slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_page_tree ON wiki.page (id_space, id_parent, rank) WHERE deleted_at IS NULL;
CREATE INDEX idx_page_trash ON wiki.page (id_space, deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_page_search ON wiki.page USING gin (search_vector);

CREATE TABLE wiki.page_version (
    id_version   bigserial PRIMARY KEY,
    id_page      bigint NOT NULL REFERENCES wiki.page (id_page) ON DELETE CASCADE,
    version_no   int NOT NULL,
    id_parent    bigint,
    title        character varying(200) NOT NULL,
    summary      character varying(500) NOT NULL,
    body         text NOT NULL,
    agent_access text NOT NULL,
    note         character varying(500) NOT NULL DEFAULT '',
    merged_from  int,
    create_at    timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    create_by    bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    CONSTRAINT page_version_uq UNIQUE (id_page, version_no)
);
ALTER TABLE wiki.page_version OWNER TO rurdesk;

CREATE TABLE wiki.page_draft (
    id_page      bigint NOT NULL REFERENCES wiki.page (id_page) ON DELETE CASCADE,
    id_user      bigint NOT NULL REFERENCES users.user (id_user) ON DELETE CASCADE,
    base_version int NOT NULL,
    title        character varying(200) NOT NULL,
    summary      character varying(500) NOT NULL,
    body         text NOT NULL,
    update_at    timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    PRIMARY KEY (id_page, id_user)
);
ALTER TABLE wiki.page_draft OWNER TO rurdesk;

CREATE TABLE wiki.page_link (
    id_page_from bigint NOT NULL REFERENCES wiki.page (id_page) ON DELETE CASCADE,
    id_space_to  bigint NOT NULL REFERENCES wiki.space (id_space) ON DELETE CASCADE,
    target_slug  character varying(120) NOT NULL,
    PRIMARY KEY (id_page_from, id_space_to, target_slug)
);
ALTER TABLE wiki.page_link OWNER TO rurdesk;

CREATE INDEX idx_page_link_target ON wiki.page_link (id_space_to, target_slug);

CREATE TABLE wiki.issue_page (
    id_issue bigint NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_page  bigint NOT NULL REFERENCES wiki.page (id_page) ON DELETE CASCADE,
    source   text NOT NULL,
    PRIMARY KEY (id_issue, id_page),
    CONSTRAINT issue_page_source_chk CHECK (source IN ('description', 'manual'))
);
ALTER TABLE wiki.issue_page OWNER TO rurdesk;

CREATE INDEX idx_issue_page_page ON wiki.issue_page (id_page);

ALTER TABLE projects.project ADD COLUMN wiki_always_token_limit int NOT NULL DEFAULT 12000;

-- +goose Down
ALTER TABLE projects.project DROP COLUMN wiki_always_token_limit;
DROP SCHEMA wiki CASCADE;
