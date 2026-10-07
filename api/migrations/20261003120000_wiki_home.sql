-- +goose Up
ALTER TABLE wiki.space ADD COLUMN id_home_page bigint REFERENCES wiki.page (id_page) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE wiki.space DROP COLUMN id_home_page;
