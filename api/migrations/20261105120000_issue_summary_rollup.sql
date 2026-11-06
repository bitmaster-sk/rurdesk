-- +goose Up
ALTER TABLE issues.issue
    ADD COLUMN is_summary BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN rollup_estimated BIGINT NULL,
    ADD COLUMN rollup_tracked BIGINT NULL,
    ADD COLUMN backlog_children INT NOT NULL DEFAULT 0,
    ADD COLUMN pre_summary_estimated BIGINT NULL,
    ADD COLUMN pre_summary_scheduled_at TIMESTAMP WITHOUT TIME ZONE NULL;

UPDATE issues.issue p
SET pre_summary_estimated = p.estimated,
    pre_summary_scheduled_at = p.scheduled_at
WHERE EXISTS (
    SELECT 1 FROM issues.issue_relation r
    WHERE r.relation_type = 'hierarchy' AND r.id_issue_from = p.id_issue
);

-- Keep in sync with HierarchyRepository.RecomputeNodes.
-- +goose StatementBegin
DO $$
DECLARE
    changed INT;
    pass INT := 0;
BEGIN
    LOOP
        WITH targets AS (
            SELECT DISTINCT id_issue_from AS id_issue
            FROM issues.issue_relation
            WHERE relation_type = 'hierarchy'
        ),
        agg AS (
            SELECT
                r.id_issue_from AS id_parent,
                bool_or(c.scheduled_at IS NOT NULL) AS any_scheduled,
                min(c.scheduled_at) AS min_start,
                max(c.scheduled_at + make_interval(secs => COALESCE(NULLIF(c.estimated, 0), 3600))) AS max_end,
                sum(CASE WHEN c.rollup_estimated IS NOT NULL THEN c.rollup_estimated ELSE COALESCE(NULLIF(c.estimated, 0), 3600) END) AS rollup_estimated,
                sum(CASE WHEN c.rollup_tracked IS NOT NULL THEN c.rollup_tracked ELSE c.tracked END) AS rollup_tracked,
                sum((CASE WHEN c.scheduled_at IS NULL THEN 1 ELSE 0 END) + c.backlog_children) AS backlog_children
            FROM issues.issue_relation r
            INNER JOIN issues.issue c ON c.id_issue = r.id_issue_to
            WHERE r.relation_type = 'hierarchy'
            GROUP BY r.id_issue_from
        ),
        computed AS (
            SELECT
                p.id_issue,
                COALESCE(a.any_scheduled, FALSE) AS is_summary,
                CASE WHEN a.any_scheduled THEN a.min_start ELSE p.scheduled_at END AS scheduled_at,
                CASE WHEN a.any_scheduled THEN EXTRACT(EPOCH FROM a.max_end - a.min_start)::BIGINT ELSE p.estimated END AS estimated,
                a.rollup_estimated,
                CASE WHEN a.id_parent IS NULL THEN NULL ELSE a.rollup_tracked + p.tracked END AS rollup_tracked,
                COALESCE(a.backlog_children, 0) AS backlog_children
            FROM issues.issue p
            INNER JOIN targets t ON t.id_issue = p.id_issue
            LEFT JOIN agg a ON a.id_parent = p.id_issue
        )
        UPDATE issues.issue p
        SET is_summary = c.is_summary,
            scheduled_at = c.scheduled_at,
            estimated = c.estimated,
            rollup_estimated = c.rollup_estimated,
            rollup_tracked = c.rollup_tracked,
            backlog_children = c.backlog_children
        FROM computed c
        WHERE p.id_issue = c.id_issue
          AND (p.is_summary, p.scheduled_at, p.estimated, p.rollup_estimated, p.rollup_tracked, p.backlog_children)
              IS DISTINCT FROM (c.is_summary, c.scheduled_at, c.estimated, c.rollup_estimated, c.rollup_tracked, c.backlog_children);

        GET DIAGNOSTICS changed = ROW_COUNT;
        pass := pass + 1;
        EXIT WHEN changed = 0 OR pass >= 100;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
UPDATE issues.issue
SET estimated = pre_summary_estimated,
    scheduled_at = pre_summary_scheduled_at
WHERE is_summary
  AND (pre_summary_estimated IS NOT NULL OR pre_summary_scheduled_at IS NOT NULL);

ALTER TABLE issues.issue
    DROP COLUMN pre_summary_scheduled_at,
    DROP COLUMN pre_summary_estimated,
    DROP COLUMN backlog_children,
    DROP COLUMN rollup_tracked,
    DROP COLUMN rollup_estimated,
    DROP COLUMN is_summary;
