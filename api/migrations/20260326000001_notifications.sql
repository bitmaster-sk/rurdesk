-- +goose Up
CREATE SCHEMA IF NOT EXISTS notification;

CREATE TABLE notification.notification (
    id_notification BIGSERIAL PRIMARY KEY,
    id_user         BIGINT NOT NULL REFERENCES users.user(id_user) ON DELETE CASCADE,
    type            VARCHAR(32) NOT NULL,
    id_project      BIGINT REFERENCES projects.project(id_project) ON DELETE CASCADE,
    actor_name      VARCHAR(255),
    actor_gravatar  VARCHAR(255),
    actor_avatar_bg VARCHAR(32),
    ref_type        VARCHAR(32),
    ref_id          VARCHAR(64),
    ref_title       VARCHAR(512),
    ref_public_id   BIGINT,
    body            TEXT,
    is_read         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON notification.notification (id_user, is_read, created_at DESC);
CREATE INDEX ON notification.notification (id_user, id_project, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS notification.notification;
DROP SCHEMA IF EXISTS notification;
