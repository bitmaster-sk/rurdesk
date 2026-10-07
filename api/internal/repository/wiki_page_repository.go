package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WikiPageRepository struct {
	pool *pgxpool.Pool
}

const wikiPageColumns = `
	id_page, id_space, id_parent, slug, title, summary, body, agent_access, rank, version_no,
	deleted_at, deleted_by, id_deleted_root, create_at, update_at, create_by, update_by
`

func NewWikiPageRepository(pool *pgxpool.Pool) *WikiPageRepository {
	return &WikiPageRepository{pool: pool}
}

func (r *WikiPageRepository) LoadPage(ctx context.Context, idPage int64) (*model.WikiPage, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT `+wikiPageColumns+` FROM wiki.page WHERE id_page = $1`, idPage)
	if err != nil {
		return nil, fmt.Errorf("querying wiki page: %w", err)
	}
	return r.collectPage(rows)
}

func (r *WikiPageRepository) LockPage(ctx context.Context, idPage int64) (*model.WikiPage, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT `+wikiPageColumns+` FROM wiki.page WHERE id_page = $1 FOR NO KEY UPDATE`, idPage)
	if err != nil {
		return nil, fmt.Errorf("locking wiki page: %w", err)
	}
	return r.collectPage(rows)
}

func (r *WikiPageRepository) LoadLivePageBySlug(ctx context.Context, idSpace int64, slug string) (*model.WikiPage, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT `+wikiPageColumns+` FROM wiki.page
		WHERE id_space = $1 AND slug = $2 AND deleted_at IS NULL
	`, idSpace, slug)
	if err != nil {
		return nil, fmt.Errorf("querying wiki page by slug: %w", err)
	}
	return r.collectPage(rows)
}

func (r *WikiPageRepository) IsSlugTaken(ctx context.Context, idSpace int64, slug string) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var taken bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM wiki.page WHERE id_space = $1 AND slug = $2 AND deleted_at IS NULL)
	`, idSpace, slug).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("checking wiki slug: %w", err)
	}
	return taken, nil
}

func (r *WikiPageRepository) LoadTree(ctx context.Context, idsSpace []int64, hideHidden bool) ([]*model.WikiTreeNode, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_page, id_space, id_parent, slug, title, agent_access, rank
		FROM wiki.page
		WHERE id_space = ANY($1) AND deleted_at IS NULL AND (NOT $2 OR agent_access <> 'hidden')
		ORDER BY id_space, rank, id_page
	`, idsSpace, hideHidden)
	if err != nil {
		return nil, fmt.Errorf("querying wiki tree: %w", err)
	}
	nodes, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiTreeNode])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki tree: %w", err)
	}
	return nodes, nil
}

func (r *WikiPageRepository) LoadLastSiblingRank(ctx context.Context, idSpace int64, idParent *int64) (*string, error) {
	db := extctx.GetDb(ctx, r.pool)
	var rank *string
	err := db.QueryRow(ctx, `
		SELECT max(rank) FROM wiki.page
		WHERE id_space = $1 AND id_parent IS NOT DISTINCT FROM $2 AND deleted_at IS NULL
	`, idSpace, idParent).Scan(&rank)
	if err != nil {
		return nil, fmt.Errorf("querying last wiki sibling rank: %w", err)
	}
	return rank, nil
}

func (r *WikiPageRepository) InsertPage(ctx context.Context, page *model.WikiPage, idUser int64) (*model.WikiPage, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		INSERT INTO wiki.page (id_space, id_parent, slug, title, summary, body, agent_access, rank, version_no, create_by, update_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, $9)
		RETURNING `+wikiPageColumns,
		page.IdSpace, page.IdParent, page.Slug, page.Title, page.Summary, page.Body, page.AgentAccess, page.Rank, idUser)
	if err != nil {
		return nil, fmt.Errorf("inserting wiki page: %w", err)
	}
	inserted, err := r.collectPage(rows)
	if isUniqueViolation(err) {
		return nil, errs.ErrWikiSlugTaken
	}
	return inserted, err
}

func (r *WikiPageRepository) UpdateContent(ctx context.Context, page *model.WikiPage, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE wiki.page
		SET title = $2, summary = $3, body = $4, agent_access = $5, version_no = $6,
		    update_at = (now() at time zone 'utc'), update_by = $7
		WHERE id_page = $1
	`, page.IdPage, page.Title, page.Summary, page.Body, page.AgentAccess, page.VersionNo, idUser)
	if err != nil {
		return fmt.Errorf("updating wiki page: %w", err)
	}
	return nil
}

func (r *WikiPageRepository) UpdatePosition(ctx context.Context, idPage int64, idParent *int64, rank string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `UPDATE wiki.page SET id_parent = $2, rank = $3 WHERE id_page = $1`, idPage, idParent, rank)
	if err != nil {
		return fmt.Errorf("updating wiki page position: %w", err)
	}
	return nil
}

func (r *WikiPageRepository) LoadLiveAncestorIds(ctx context.Context, idPage int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		WITH RECURSIVE up AS (
			SELECT id_page, id_parent FROM wiki.page WHERE id_page = $1
			UNION
			SELECT p.id_page, p.id_parent FROM wiki.page p INNER JOIN up ON p.id_page = up.id_parent
		)
		SELECT id_page FROM up
	`, idPage)
	if err != nil {
		return nil, fmt.Errorf("querying wiki ancestors: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki ancestors: %w", err)
	}
	return ids, nil
}

func (r *WikiPageRepository) LoadLiveSubtreeIds(ctx context.Context, idPage int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		WITH RECURSIVE down AS (
			SELECT id_page FROM wiki.page WHERE id_page = $1 AND deleted_at IS NULL
			UNION
			SELECT p.id_page FROM wiki.page p INNER JOIN down ON p.id_parent = down.id_page
			WHERE p.deleted_at IS NULL
		)
		SELECT id_page FROM down
	`, idPage)
	if err != nil {
		return nil, fmt.Errorf("querying wiki subtree: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki subtree: %w", err)
	}
	return ids, nil
}

func (r *WikiPageRepository) Trash(ctx context.Context, idRoot int64, idsPage []int64, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE wiki.page
		SET deleted_at = (now() at time zone 'utc'), deleted_by = $3, id_deleted_root = $1
		WHERE id_page = ANY($2) AND deleted_at IS NULL
	`, idRoot, idsPage, idUser)
	if err != nil {
		return fmt.Errorf("trashing wiki pages: %w", err)
	}
	return nil
}

func (r *WikiPageRepository) LoadTrashedPages(ctx context.Context, idRoot int64) ([]*model.WikiPage, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `SELECT `+wikiPageColumns+` FROM wiki.page WHERE id_deleted_root = $1`, idRoot)
	if err != nil {
		return nil, fmt.Errorf("querying trashed wiki pages: %w", err)
	}
	pages, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiPage])
	if err != nil {
		return nil, fmt.Errorf("collecting trashed wiki pages: %w", err)
	}
	return pages, nil
}

func (r *WikiPageRepository) Restore(ctx context.Context, idRoot int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE wiki.page SET deleted_at = NULL, deleted_by = NULL, id_deleted_root = NULL
		WHERE id_deleted_root = $1
	`, idRoot)
	if isUniqueViolation(err) {
		return errs.ErrWikiSlugTaken
	}
	if err != nil {
		return fmt.Errorf("restoring wiki pages: %w", err)
	}
	return nil
}

func (r *WikiPageRepository) LoadTrash(ctx context.Context, idsSpace []int64, hideHidden bool) ([]*model.WikiTrashItem, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT p.id_page, p.id_space, p.id_parent, p.title, p.deleted_at, p.deleted_by,
		       (SELECT count(*) FROM wiki.page d WHERE d.id_deleted_root = p.id_page AND d.id_page <> p.id_page)::int AS descendants,
		       (p.id_parent IS NULL OR EXISTS (
		           SELECT 1 FROM wiki.page parent WHERE parent.id_page = p.id_parent AND parent.deleted_at IS NULL
		       )) AS parent_alive
		FROM wiki.page p
		WHERE p.id_space = ANY($1) AND p.id_deleted_root = p.id_page AND (NOT $2 OR p.agent_access <> 'hidden')
		ORDER BY p.deleted_at DESC
	`, idsSpace, hideHidden)
	if err != nil {
		return nil, fmt.Errorf("querying wiki trash: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiTrashItem])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki trash: %w", err)
	}
	return items, nil
}

func (r *WikiPageRepository) CountTrash(ctx context.Context, idsSpace []int64, hideHidden bool) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int
	err := db.QueryRow(ctx, `
		SELECT count(*)::int
		FROM wiki.page
		WHERE id_space = ANY($1) AND id_deleted_root = id_page AND (NOT $2 OR agent_access <> 'hidden')
	`, idsSpace, hideHidden).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting wiki trash: %w", err)
	}
	return count, nil
}

func (r *WikiPageRepository) Purge(ctx context.Context, idRoot int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM wiki.page WHERE id_deleted_root = $1`, idRoot)
	if err != nil {
		return fmt.Errorf("purging wiki pages: %w", err)
	}
	return nil
}

func (r *WikiPageRepository) PurgeTrashedBefore(ctx context.Context, before time.Time) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		DELETE FROM wiki.page
		WHERE id_deleted_root IN (
			SELECT id_page FROM wiki.page WHERE id_deleted_root = id_page AND deleted_at < $1
		)
	`, before)
	if err != nil {
		return 0, fmt.Errorf("purging expired wiki trash: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *WikiPageRepository) LoadLivePagesBySlugs(ctx context.Context, idsSpace []int64, slugs []string, hideHidden bool) ([]*model.WikiPageRef, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_page, id_space, slug, title FROM wiki.page
		WHERE id_space = ANY($1) AND slug = ANY($2) AND deleted_at IS NULL AND (NOT $3 OR agent_access <> 'hidden')
	`, idsSpace, slugs, hideHidden)
	if err != nil {
		return nil, fmt.Errorf("querying wiki pages by slugs: %w", err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiPageRef])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki pages by slugs: %w", err)
	}
	return refs, nil
}

func (r *WikiPageRepository) Search(ctx context.Context, idsSpace []int64, prefixQuery string, limit int, hideHidden bool) ([]*model.WikiSearchHit, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		WITH q AS (SELECT to_tsquery('simple', $2) AS query)
		SELECT p.id_page, p.id_space, p.slug, p.title, p.summary,
		       ts_headline('simple', p.body, q.query, 'MaxFragments=2, MaxWords=20, MinWords=5, StartSel=<<, StopSel=>>') AS snippet,
		       ts_rank_cd(p.search_vector, q.query) AS score
		FROM wiki.page p, q
		WHERE p.id_space = ANY($1) AND p.deleted_at IS NULL AND p.search_vector @@ q.query
		  AND (NOT $4 OR p.agent_access <> 'hidden')
		ORDER BY score DESC, p.title
		LIMIT $3
	`, idsSpace, prefixQuery, limit, hideHidden)
	if err != nil {
		return nil, fmt.Errorf("searching wiki: %w", err)
	}
	hits, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiSearchHit])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki search hits: %w", err)
	}
	return hits, nil
}

func (r *WikiPageRepository) SumAlwaysChars(ctx context.Context, idsSpace []int64, excludeIdPage int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	var total int
	err := db.QueryRow(ctx, `
		SELECT coalesce(sum(char_length(title) + char_length(body)), 0)::int
		FROM wiki.page
		WHERE id_space = ANY($1) AND agent_access = 'always' AND deleted_at IS NULL AND id_page <> $2
	`, idsSpace, excludeIdPage).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("summing always wiki pages: %w", err)
	}
	return total, nil
}

func (r *WikiPageRepository) SumAlwaysCharsBySpace(ctx context.Context, idsExcluded []int64) (map[int64]int, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_space, coalesce(sum(char_length(title) + char_length(body)), 0)::int
		FROM wiki.page
		WHERE agent_access = 'always' AND deleted_at IS NULL AND NOT (id_page = ANY($1))
		GROUP BY id_space
	`, append([]int64{}, idsExcluded...))
	if err != nil {
		return nil, fmt.Errorf("summing always wiki pages by space: %w", err)
	}
	defer rows.Close()
	sums := map[int64]int{}
	for rows.Next() {
		var idSpace int64
		var chars int
		if err := rows.Scan(&idSpace, &chars); err != nil {
			return nil, fmt.Errorf("scanning always wiki sums: %w", err)
		}
		sums[idSpace] = chars
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating always wiki sums: %w", err)
	}
	return sums, nil
}

func (r *WikiPageRepository) collectPage(rows pgx.Rows) (*model.WikiPage, error) {
	page, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.WikiPage])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting wiki page: %w", err)
	}
	return page, nil
}
