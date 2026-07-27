-- +goose Up

ALTER TABLE issues.issue ADD COLUMN gantt_rank TEXT NULL;

-- Speeds up "does this project have any ranked scheduled issue" and rank-ordered reads.
CREATE INDEX idx_issue_gantt_rank ON issues.issue (id_project, gantt_rank) WHERE gantt_rank IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS issues.idx_issue_gantt_rank;
ALTER TABLE issues.issue DROP COLUMN IF EXISTS gantt_rank;
