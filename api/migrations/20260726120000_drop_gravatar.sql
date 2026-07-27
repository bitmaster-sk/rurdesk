-- +goose Up
-- Gravatar support is gone. It made every browser fetch avatars from
-- gravatar.com on each render and had the API post an MD5 of every registering
-- user's email there — third-party exposure a self-hosted tracker should not
-- create. Avatars are now the built-in initials on the user's chosen colour.
ALTER TABLE users.user DROP COLUMN IF EXISTS gravatar;
ALTER TABLE notification.notification DROP COLUMN IF EXISTS actor_gravatar;

-- +goose Down
ALTER TABLE users.user ADD COLUMN gravatar character varying(250);
ALTER TABLE notification.notification ADD COLUMN actor_gravatar VARCHAR(255);
