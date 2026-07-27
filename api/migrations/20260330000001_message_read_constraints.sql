-- +goose Up
ALTER TABLE messages.user_read
    ADD CONSTRAINT uq_user_read UNIQUE (id_user, id_user_from);

ALTER TABLE messages.team_read
    ADD CONSTRAINT uq_team_read UNIQUE (id_user, id_team_to);

ALTER TABLE messages.project_read
    ADD CONSTRAINT uq_project_read UNIQUE (id_user, id_project_to);

-- +goose Down
ALTER TABLE messages.user_read DROP CONSTRAINT uq_user_read;
ALTER TABLE messages.team_read DROP CONSTRAINT uq_team_read;
ALTER TABLE messages.project_read DROP CONSTRAINT uq_project_read;
