-- +goose Up
-- Per-stage bot provenance: which bot (gateway) executed each stage attempt.
-- Nullable because a manual hand-off can change a run's executor, so the run's
-- current id_user_bot no longer identifies who ran earlier stages. Existing
-- rows stay NULL — the timeline simply omits provenance for them.
ALTER TABLE agent.task ADD COLUMN id_user_bot bigint REFERENCES users.user(id_user);

-- +goose Down
ALTER TABLE agent.task DROP COLUMN IF EXISTS id_user_bot;
