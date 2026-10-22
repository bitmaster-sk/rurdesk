-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    projects_with_many TEXT;
BEGIN
    SELECT string_agg(id_project::TEXT, ', ' ORDER BY id_project)
    INTO projects_with_many
    FROM (
        SELECT id_project
        FROM projects.git_integration
        GROUP BY id_project
        HAVING COUNT(*) > 1
    ) AS duplicated;

    IF projects_with_many IS NOT NULL THEN
        RAISE EXCEPTION 'a project may have only one git integration; delete the extra integrations of projects % and run the migration again', projects_with_many;
    END IF;
END $$;
-- +goose StatementEnd

DROP INDEX projects.git_integration_id_project_idx;
CREATE UNIQUE INDEX git_integration_one_per_project ON projects.git_integration(id_project);

ALTER TABLE projects.git_integration ADD COLUMN base_branch TEXT;
ALTER TABLE agent.run ADD COLUMN base_branch TEXT;

-- +goose Down
ALTER TABLE agent.run DROP COLUMN base_branch;
ALTER TABLE projects.git_integration DROP COLUMN base_branch;

DROP INDEX projects.git_integration_one_per_project;
CREATE INDEX git_integration_id_project_idx ON projects.git_integration(id_project);
