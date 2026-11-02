package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueLinkRepository struct {
	pool *pgxpool.Pool
}

func NewIssueLinkRepository(pool *pgxpool.Pool) *IssueLinkRepository {
	return &IssueLinkRepository{pool: pool}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (r *IssueLinkRepository) Search(ctx context.Context, idsProject []int64, idProjectFirst int64, query string, limit int) ([]*model.IssueLinkOption, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT iss.id_project, prj.name AS project_name, iss.id_issue_public, iss.title
		FROM issues.issue iss
		INNER JOIN projects.project prj ON prj.id_project = iss.id_project
		WHERE iss.id_project = ANY($1)
			AND ($2 = '' OR iss.id_issue_public::text LIKE $3 || '%' OR iss.title ILIKE '%' || $3 || '%')
		ORDER BY
			iss.id_project = $4 DESC,
			iss.id_issue_public::text = $2 DESC,
			iss.id_issue_public::text LIKE $3 || '%' DESC,
			iss.create_at DESC,
			iss.id_issue DESC
		LIMIT $5
	`, idsProject, query, likeEscaper.Replace(query), idProjectFirst, limit)
	if err != nil {
		return nil, fmt.Errorf("querying issue link options: %w", err)
	}
	options, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.IssueLinkOption])
	if err != nil {
		return nil, fmt.Errorf("collecting issue link options: %w", err)
	}
	return options, nil
}

func (r *IssueLinkRepository) LoadByRefs(ctx context.Context, idsProject []int64, refs []model.IssueLinkRef) ([]*model.IssueLinkIssue, error) {
	db := extctx.GetDb(ctx, r.pool)
	refProjects := make([]int64, len(refs))
	refIssues := make([]int64, len(refs))
	for i, ref := range refs {
		refProjects[i] = ref.IdProject
		refIssues[i] = ref.IdIssuePublic
	}
	rows, err := db.Query(ctx, `
		SELECT iss.id_project, iss.id_issue_public, iss.title, iss.id_state, iss.id_severity, iss.id_issue_type, iss.assigned_to
		FROM issues.issue iss
		INNER JOIN unnest($2::bigint[], $3::bigint[]) AS ref(id_project, id_issue_public)
			ON ref.id_project = iss.id_project AND ref.id_issue_public = iss.id_issue_public
		WHERE iss.id_project = ANY($1)
		ORDER BY iss.id_project, iss.id_issue_public
	`, idsProject, refProjects, refIssues)
	if err != nil {
		return nil, fmt.Errorf("querying linked issues: %w", err)
	}
	issues, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.IssueLinkIssue])
	if err != nil {
		return nil, fmt.Errorf("collecting linked issues: %w", err)
	}
	return issues, nil
}
