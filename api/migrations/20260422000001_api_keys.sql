-- +goose Up
CREATE TABLE users.api_key (
    id_api_key          BIGSERIAL PRIMARY KEY,
    id_user             BIGINT NOT NULL REFERENCES users.user(id_user) ON DELETE CASCADE,
    key_hash            CHAR(64) NOT NULL UNIQUE,
    name                VARCHAR(100) NOT NULL,
    rate_limit_override INT,
    created_at          TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    expires_at          TIMESTAMP WITHOUT TIME ZONE,
    last_used_at        TIMESTAMP WITHOUT TIME ZONE
);
CREATE INDEX ON users.api_key(id_user);

-- +goose Down
DROP TABLE users.api_key;
