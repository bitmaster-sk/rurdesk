-- +goose Up
-- The wiki always-read token limit becomes one global app setting instead of a
-- per-project column. Copy the largest configured limit so pages that fit
-- before the move still fit after it; the 12000 default needs no row.
-- ON CONFLICT DO NOTHING: a row can only pre-exist on a database that already
-- ran the new code, and an admin-set value then wins over the inferred one.
INSERT INTO projects.app_settings (key, value)
SELECT 'wiki.always_token_limit', to_jsonb(MAX(wiki_always_token_limit))
FROM projects.project
HAVING MAX(wiki_always_token_limit) <> 12000
ON CONFLICT (key) DO NOTHING;

ALTER TABLE projects.project DROP COLUMN wiki_always_token_limit;

-- +goose Down
ALTER TABLE projects.project ADD COLUMN wiki_always_token_limit int NOT NULL DEFAULT 12000;