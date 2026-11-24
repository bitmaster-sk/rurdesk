-- +goose Up

ALTER TABLE messages.message_anchor
    ADD COLUMN anchor_text       TEXT,
    ADD COLUMN quote             TEXT,
    ADD COLUMN id_thread_message BIGINT REFERENCES messages.message(id_message) ON DELETE CASCADE;

-- Lines normalize CRLF to LF and ignore one trailing LF.
-- Anchors on edited parents or past the end get an empty anchor_text (treated as outdated).
UPDATE messages.message_anchor a
SET anchor_text = CASE
        WHEN a.parent_version = src.version
         AND a.anchor_line_end <= COALESCE(array_length(src.lines, 1), 0)
            THEN array_to_string(src.lines[a.anchor_line_start:a.anchor_line_end], E'\n')
        ELSE ''
    END
FROM (
    SELECT p.id_message,
           p.version,
           string_to_array(regexp_replace(replace(p.message, E'\r\n', E'\n'), E'\n$', ''), E'\n') AS lines
    FROM messages.message p
) src
WHERE src.id_message = a.id_parent_message;

-- Rows sharing a range become one thread rooted at the oldest; replies take the root's anchor so they resolve with it.
UPDATE messages.message_anchor a
SET id_thread_message = grouped.id_root,
    parent_version    = root.parent_version,
    anchor_text       = root.anchor_text
FROM (
    SELECT ma.id_message,
           first_value(ma.id_message) OVER (
               PARTITION BY ma.id_parent_message, ma.anchor_line_start, ma.anchor_line_end
               ORDER BY m.created_at, ma.id_message
           ) AS id_root
    FROM messages.message_anchor ma
    INNER JOIN messages.message m ON m.id_message = ma.id_message
) grouped
INNER JOIN messages.message_anchor root ON root.id_message = grouped.id_root
WHERE a.id_message = grouped.id_message
  AND grouped.id_root <> a.id_message;

ALTER TABLE messages.message_anchor ALTER COLUMN anchor_text SET NOT NULL;

CREATE INDEX idx_message_anchor_thread ON messages.message_anchor(id_thread_message);

COMMENT ON COLUMN messages.message_anchor.anchor_text IS
    'Parent source lines anchor_line_start..anchor_line_end at creation; found again in an edited parent to relocate the anchor.';
COMMENT ON COLUMN messages.message_anchor.quote IS
    'Exact text the user selected; NULL anchors the whole line range.';
COMMENT ON COLUMN messages.message_anchor.id_thread_message IS
    'Thread root for a reply in a thread; NULL on the root. Replies copy the root''s anchor columns.';

CREATE TABLE messages.issue_read (
    id_user     BIGINT NOT NULL REFERENCES users.user (id_user) ON DELETE CASCADE,
    id_issue_to BIGINT NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    read_at     TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    CONSTRAINT uq_issue_read UNIQUE (id_issue_to, id_user)
);

COMMENT ON TABLE messages.issue_read IS
    'Last time each user read an issue''s conversation; later messages by others are unread.';

-- +goose Down
DROP TABLE IF EXISTS messages.issue_read;
DROP INDEX IF EXISTS messages.idx_message_anchor_thread;
ALTER TABLE messages.message_anchor
    DROP COLUMN id_thread_message,
    DROP COLUMN quote,
    DROP COLUMN anchor_text;
