-- +goose Up
ALTER TABLE users.user ALTER COLUMN password DROP NOT NULL;
ALTER TABLE users.user ADD COLUMN is_break_glass BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users.user ADD COLUMN last_login_at TIMESTAMPTZ;
ALTER TABLE users.user ADD COLUMN scim_external_id TEXT;
ALTER TABLE users.user ADD COLUMN scim_deleted_at TIMESTAMPTZ;

-- Rows whose lowercase form would collide with another account keep their case; an admin resolves them by hand.
UPDATE users.user AS u
SET email = lower(u.email)
WHERE u.email <> lower(u.email)
  AND NOT EXISTS (
      SELECT 1 FROM users.user AS other
      WHERE other.id_user <> u.id_user AND lower(other.email) = lower(u.email)
  );

CREATE INDEX user_email_lower_idx ON users.user (lower(email));
CREATE UNIQUE INDEX user_scim_external_id_idx ON users.user (scim_external_id) WHERE scim_external_id IS NOT NULL;

CREATE TABLE users.user_identity (
    id_user_identity BIGSERIAL PRIMARY KEY,
    id_user BIGINT NOT NULL REFERENCES users.user (id_user) ON DELETE CASCADE,
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    has_sent_groups BOOLEAN NOT NULL DEFAULT FALSE,
    create_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject),
    UNIQUE (id_user, issuer)
);

CREATE TABLE users.sso_config (
    kind TEXT PRIMARY KEY CHECK (kind IN ('draft', 'active')),
    protocol TEXT NOT NULL,
    issuer TEXT NOT NULL DEFAULT '',
    client_id TEXT NOT NULL DEFAULT '',
    client_secret_enc BYTEA,
    client_secret_nonce BYTEA,
    scopes_groups BOOLEAN NOT NULL DEFAULT FALSE,
    email_claim TEXT NOT NULL DEFAULT 'email',
    groups_claim TEXT NOT NULL DEFAULT 'groups',
    idp_metadata_url TEXT NOT NULL DEFAULT '',
    idp_metadata_xml TEXT NOT NULL DEFAULT '',
    idp_metadata_fetched_at TIMESTAMPTZ,
    name_attribute TEXT NOT NULL DEFAULT '',
    subject_attribute TEXT NOT NULL DEFAULT '',
    tested_hash TEXT,
    update_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    update_by BIGINT REFERENCES users.user (id_user) ON DELETE SET NULL
);

CREATE TABLE users.sso_settings (
    id_sso_settings SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id_sso_settings = 1),
    is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_enforced BOOLEAN NOT NULL DEFAULT FALSE,
    is_scim_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    allowed_domains TEXT[] NOT NULL DEFAULT '{}',
    is_auto_create BOOLEAN NOT NULL DEFAULT FALSE,
    admin_group TEXT NOT NULL DEFAULT '',
    sp_cert TEXT NOT NULL DEFAULT '',
    sp_key_enc BYTEA,
    sp_key_nonce BYTEA
);

INSERT INTO users.sso_settings DEFAULT VALUES;

CREATE TABLE users.sso_group_mapping (
    id_sso_group_mapping BIGSERIAL PRIMARY KEY,
    group_value TEXT NOT NULL,
    id_team BIGINT NOT NULL REFERENCES users.team (id_team) ON DELETE CASCADE,
    create_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_value, id_team)
);

CREATE TABLE users.scim_token (
    id_scim_token SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id_scim_token = 1),
    token_hash TEXT NOT NULL,
    create_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    create_by BIGINT REFERENCES users.user (id_user) ON DELETE SET NULL,
    last_used_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE users.scim_token;
DROP TABLE users.sso_group_mapping;
DROP TABLE users.sso_settings;
DROP TABLE users.sso_config;
DROP TABLE users.user_identity;
DROP INDEX users.user_scim_external_id_idx;
DROP INDEX users.user_email_lower_idx;
ALTER TABLE users.user DROP COLUMN scim_deleted_at;
ALTER TABLE users.user DROP COLUMN scim_external_id;
ALTER TABLE users.user DROP COLUMN last_login_at;
ALTER TABLE users.user DROP COLUMN is_break_glass;
UPDATE users.user SET password = '' WHERE password IS NULL;
ALTER TABLE users.user ALTER COLUMN password SET NOT NULL;
