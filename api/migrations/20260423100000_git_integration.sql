-- +goose Up
CREATE TABLE projects.git_integration (
    id_git_integration BIGSERIAL PRIMARY KEY,
    id_project         BIGINT NOT NULL REFERENCES projects.project(id_project) ON DELETE CASCADE,
    name               VARCHAR(100) NOT NULL,
    host_type          VARCHAR(20) NOT NULL,
    base_url           TEXT NOT NULL,
    repo_path          TEXT NOT NULL,
    access_token_enc   BYTEA NOT NULL,
    token_nonce        BYTEA NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT git_integration_host_type_check
        CHECK (host_type IN ('github', 'gitlab', 'gitea')),
    CONSTRAINT git_integration_unique_repo
        UNIQUE (id_project, host_type, base_url, repo_path)
);
CREATE INDEX git_integration_id_project_idx ON projects.git_integration(id_project);

-- +goose Down
DROP TABLE projects.git_integration;
