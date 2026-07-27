-- +goose Up

ALTER TABLE messages.message
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE messages.message_anchor (
    id_message          BIGINT PRIMARY KEY REFERENCES messages.message(id_message) ON DELETE CASCADE,
    id_parent_message   BIGINT NOT NULL REFERENCES messages.message(id_message) ON DELETE CASCADE,
    parent_version      INTEGER NOT NULL,
    anchor_line_start   INTEGER NOT NULL CHECK (anchor_line_start >= 1),
    anchor_line_end     INTEGER NOT NULL CHECK (anchor_line_end >= anchor_line_start)
);

CREATE INDEX idx_message_anchor_parent ON messages.message_anchor(id_parent_message);

COMMENT ON TABLE messages.message_anchor IS
    'Optional side-table: a child message that targets a line range of a parent message. '
    'parent_version captures the parent.version at anchor creation; on read we compare to the parent''s current version to flag outdated anchors. '
    'ON DELETE CASCADE on both FKs ensures anchor rows die with either side.';

-- +goose Down
DROP TABLE IF EXISTS messages.message_anchor;
ALTER TABLE messages.message DROP COLUMN version;
