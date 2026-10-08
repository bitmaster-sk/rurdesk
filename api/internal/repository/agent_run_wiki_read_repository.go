package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentRunWikiReadRepository struct {
	pool *pgxpool.Pool
}

func NewAgentRunWikiReadRepository(pool *pgxpool.Pool) *AgentRunWikiReadRepository {
	return &AgentRunWikiReadRepository{pool: pool}
}

func (r *AgentRunWikiReadRepository) Insert(ctx context.Context, reads []*model.AgentRunWikiRead) error {
	if len(reads) == 0 {
		return nil
	}
	n := len(reads)
	idsRun, idsTask, stages, idsCall := make([]int64, n), make([]*int64, n), make([]string, n), make([]string, n)
	sources, idsPage, spaceKinds := make([]string, n), make([]*int64, n), make([]*string, n)
	slugs, titles, versions, tokens, queries := make([]*string, n), make([]*string, n), make([]*int64, n), make([]int64, n), make([]*string, n)
	pages := make([]*string, n)
	for i, read := range reads {
		idsRun[i], idsTask[i], stages[i], idsCall[i] = read.IdRun, read.IdTask, read.Stage, read.IdCall.String()
		sources[i], idsPage[i] = string(read.Source), read.IdPage
		if read.SpaceKind != nil {
			kind := string(*read.SpaceKind)
			spaceKinds[i] = &kind
		}
		slugs[i], titles[i], queries[i] = read.Slug, read.Title, read.Query
		if read.VersionNo != nil {
			version := int64(*read.VersionNo)
			versions[i] = &version
		}
		tokens[i] = int64(read.Tokens)
		if read.Pages != nil {
			encoded, err := json.Marshal(read.Pages)
			if err != nil {
				return fmt.Errorf("encoding wiki index pages: %w", err)
			}
			text := string(encoded)
			pages[i] = &text
		}
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO agent.run_wiki_read
			(id_run, id_task, stage, id_call, source, id_page, space_kind, slug, title, version_no, tokens, query, pages)
		SELECT * FROM unnest($1::bigint[], $2::bigint[], $3::text[], $4::uuid[], $5::text[], $6::bigint[],
		                     $7::text[], $8::text[], $9::text[], $10::int[], $11::int[], $12::text[], $13::jsonb[])
		ON CONFLICT DO NOTHING
	`, idsRun, idsTask, stages, idsCall, sources, idsPage, spaceKinds, slugs, titles, versions, tokens, queries, pages)
	if err != nil {
		return fmt.Errorf("inserting agent run wiki reads: %w", err)
	}
	return nil
}

func (r *AgentRunWikiReadRepository) LoadByRun(ctx context.Context, idRun int64) ([]*model.AgentRunWikiRead, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_read, id_run, id_task, stage, id_call, source, id_page, space_kind, slug, title,
		       version_no, tokens, query, pages, create_at
		FROM agent.run_wiki_read
		WHERE id_run = $1
		ORDER BY create_at, id_read
	`, idRun)
	if err != nil {
		return nil, fmt.Errorf("querying agent run wiki reads: %w", err)
	}
	reads, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.AgentRunWikiRead])
	if err != nil {
		return nil, fmt.Errorf("collecting agent run wiki reads: %w", err)
	}
	return reads, nil
}
