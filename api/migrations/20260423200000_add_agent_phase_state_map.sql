-- +goose Up
CREATE TABLE projects.agent_phase_state_map (
    id_project   BIGINT NOT NULL REFERENCES projects.project(id_project) ON DELETE CASCADE,
    phase        VARCHAR(30) NOT NULL,
    id_state     BIGINT REFERENCES issues.state(id_state) ON DELETE SET NULL,
    PRIMARY KEY (id_project, phase)
);

-- +goose Down
DROP TABLE IF EXISTS projects.agent_phase_state_map;
