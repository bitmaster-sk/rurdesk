-- +goose Up
ALTER TABLE projects.project_user
  ADD COLUMN role VARCHAR(10) NOT NULL DEFAULT 'member'
  CHECK (role IN ('viewer', 'member', 'owner'));

ALTER TABLE projects.project_team
  ADD COLUMN role VARCHAR(10) NOT NULL DEFAULT 'member'
  CHECK (role IN ('viewer', 'member', 'owner'));

-- Promote one user per project to owner.
-- The project table has no create_by column, so we pick the user with the
-- lowest id_user in each project (the creator is typically the first member added).
UPDATE projects.project_user pu
SET role = 'owner'
FROM (
    SELECT DISTINCT ON (id_project) id_project, id_user
    FROM projects.project_user
    ORDER BY id_project, id_user
) first_user
WHERE pu.id_project = first_user.id_project
  AND pu.id_user = first_user.id_user;

-- +goose Down
ALTER TABLE projects.project_user DROP COLUMN role;
ALTER TABLE projects.project_team DROP COLUMN role;
