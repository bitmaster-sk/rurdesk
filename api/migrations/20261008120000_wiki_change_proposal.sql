-- +goose Up
CREATE TABLE wiki.change_proposal (
    id_proposal    bigserial PRIMARY KEY,
    id_run         bigint NOT NULL REFERENCES agent.run (id_run) ON DELETE CASCADE,
    id_issue       bigint NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_project     bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    id_space       bigint NOT NULL REFERENCES wiki.space (id_space) ON DELETE CASCADE,
    kind           text NOT NULL,
    id_page        bigint REFERENCES wiki.page (id_page) ON DELETE SET NULL,
    slug           character varying(120) NOT NULL,
    title          character varying(200) NOT NULL,
    summary        character varying(500) NOT NULL DEFAULT '',
    body           text,
    id_parent      bigint REFERENCES wiki.page (id_page) ON DELETE SET NULL,
    parent_slug    character varying(120),
    reason         text NOT NULL,
    base_version   int,
    agent_access   text,
    decision       text NOT NULL DEFAULT 'open',
    decided_by     bigint REFERENCES users.user (id_user) ON DELETE SET NULL,
    decided_at     timestamp WITHOUT TIME ZONE,
    decision_note  text,
    result_version int,
    create_at      timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    update_at      timestamp WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
    CONSTRAINT change_proposal_kind_chk CHECK (kind IN ('create', 'update', 'move', 'delete')),
    CONSTRAINT change_proposal_decision_chk CHECK (decision IN ('open', 'approved', 'needs_resolving', 'accepted', 'rejected')),
    CONSTRAINT change_proposal_agent_access_chk CHECK (agent_access IN ('always', 'on_demand', 'hidden')),
    CONSTRAINT change_proposal_body_chk CHECK ((kind IN ('create', 'update')) = (body IS NOT NULL)),
    CONSTRAINT change_proposal_base_chk CHECK ((kind = 'create') = (base_version IS NULL)),
    CONSTRAINT change_proposal_decided_chk CHECK ((decision = 'open') = (decided_at IS NULL))
);
ALTER TABLE wiki.change_proposal OWNER TO rurdesk;

CREATE UNIQUE INDEX uq_change_proposal_run_page ON wiki.change_proposal (id_run, id_page)
    WHERE kind <> 'create';
CREATE UNIQUE INDEX uq_change_proposal_run_slug ON wiki.change_proposal (id_run, id_space, slug)
    WHERE kind = 'create';
CREATE INDEX idx_change_proposal_issue ON wiki.change_proposal (id_issue, create_at);
CREATE INDEX idx_change_proposal_project_open ON wiki.change_proposal (id_project)
    WHERE decision IN ('open', 'approved', 'needs_resolving');

-- +goose Down
DROP TABLE wiki.change_proposal;
