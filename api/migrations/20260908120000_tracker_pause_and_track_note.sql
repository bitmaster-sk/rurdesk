-- +goose Up
ALTER TABLE issues.tracker ADD COLUMN paused_at TIMESTAMP WITHOUT TIME ZONE;
ALTER TABLE issues.tracker ADD COLUMN paused_seconds BIGINT NOT NULL DEFAULT 0;
ALTER TABLE issues.tracker ADD CONSTRAINT tracker_paused_seconds_non_negative CHECK (paused_seconds >= 0);

ALTER TABLE issues.track ADD COLUMN note TEXT;

-- +goose Down
ALTER TABLE issues.track DROP COLUMN note;

ALTER TABLE issues.tracker DROP CONSTRAINT tracker_paused_seconds_non_negative;
ALTER TABLE issues.tracker DROP COLUMN paused_seconds;
ALTER TABLE issues.tracker DROP COLUMN paused_at;
