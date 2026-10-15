-- +goose Up
ALTER TABLE wiki.change_proposal
    ALTER COLUMN id_run DROP NOT NULL,
    ALTER COLUMN id_issue DROP NOT NULL,
    ADD COLUMN create_by bigint REFERENCES users.user (id_user) ON DELETE SET NULL;

UPDATE wiki.change_proposal p SET create_by = r.id_user_agent FROM agent.run r WHERE r.id_run = p.id_run;

CREATE UNIQUE INDEX uq_change_proposal_author_page ON wiki.change_proposal (create_by, id_page)
    WHERE id_run IS NULL AND kind <> 'create' AND decision = 'open';
CREATE UNIQUE INDEX uq_change_proposal_author_slug ON wiki.change_proposal (create_by, id_space, slug)
    WHERE id_run IS NULL AND kind = 'create' AND decision = 'open';

-- +goose Down
DROP INDEX wiki.uq_change_proposal_author_slug;
DROP INDEX wiki.uq_change_proposal_author_page;
DELETE FROM wiki.change_proposal WHERE id_run IS NULL OR id_issue IS NULL;
ALTER TABLE wiki.change_proposal
    DROP COLUMN create_by,
    ALTER COLUMN id_issue SET NOT NULL,
    ALTER COLUMN id_run SET NOT NULL;
