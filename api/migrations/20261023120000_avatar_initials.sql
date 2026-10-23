-- +goose Up
-- +goose StatementBegin
ALTER TABLE users.user ADD COLUMN avatar_initials varchar(2) NULL;
ALTER TABLE notification.notification ADD COLUMN actor_avatar_initials varchar(2) NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users.user DROP COLUMN avatar_initials;
ALTER TABLE notification.notification DROP COLUMN actor_avatar_initials;
-- +goose StatementEnd
