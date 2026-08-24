-- +goose Up
ALTER TABLE projects.agent_phase_state_map RENAME TO workflow_event_state_map;
ALTER TABLE projects.workflow_event_state_map RENAME COLUMN phase TO event;
ALTER TABLE issues.issue ADD COLUMN mr_state VARCHAR(10);

-- +goose Down
ALTER TABLE issues.issue DROP COLUMN IF EXISTS mr_state;
ALTER TABLE projects.workflow_event_state_map RENAME COLUMN event TO phase;
ALTER TABLE projects.workflow_event_state_map RENAME TO agent_phase_state_map;
