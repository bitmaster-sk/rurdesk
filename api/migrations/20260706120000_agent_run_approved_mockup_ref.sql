-- +goose Up
ALTER TABLE agent.run ADD COLUMN approved_mockup_ref TEXT;

-- +goose Down
ALTER TABLE agent.run DROP COLUMN approved_mockup_ref;
