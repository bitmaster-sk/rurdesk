package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WikiProposalRepository struct {
	pool *pgxpool.Pool
}

func NewWikiProposalRepository(pool *pgxpool.Pool) *WikiProposalRepository {
	return &WikiProposalRepository{pool: pool}
}

const wikiProposalSelect = `
	SELECT p.id_proposal, p.id_run, r.id_user_agent, r.phase AS run_phase,
	       p.id_issue, i.id_issue_public, i.title AS issue_title, p.id_project,
	       p.id_space, s.kind AS space_kind, p.kind, p.id_page, p.slug, p.title, p.summary, p.body,
	       p.id_parent, p.parent_slug, parent.title AS parent_title, p.reason, p.base_version,
	       p.agent_access, p.decision, p.decided_by, p.decided_at, p.decision_note, p.result_version,
	       page.version_no AS page_version, page.title AS page_title, page_parent.title AS page_parent_title,
	       (page.id_page IS NOT NULL AND page.deleted_at IS NULL) AS is_page_live,
	       p.create_at, p.update_at
	FROM wiki.change_proposal p
	JOIN agent.run r ON r.id_run = p.id_run
	JOIN issues.issue i ON i.id_issue = p.id_issue
	JOIN wiki.space s ON s.id_space = p.id_space
	LEFT JOIN wiki.page page ON page.id_page = p.id_page
	LEFT JOIN wiki.page parent ON parent.id_page = p.id_parent
	LEFT JOIN wiki.page page_parent ON page_parent.id_page = page.id_parent`

func (r *WikiProposalRepository) Upsert(ctx context.Context, draft *model.WikiProposalDraft) (int64, bool, error) {
	target := fmt.Sprintf("(id_run, id_page) WHERE kind <> '%s'", constants.WikiProposalCreate)
	if draft.Kind == constants.WikiProposalCreate {
		target = fmt.Sprintf("(id_run, id_space, slug) WHERE kind = '%s'", constants.WikiProposalCreate)
	}
	db := extctx.GetDb(ctx, r.pool)
	var idProposal int64
	var inserted bool
	err := db.QueryRow(ctx, `
		INSERT INTO wiki.change_proposal
			(id_run, id_issue, id_project, id_space, kind, id_page, slug, title, summary, body,
			 id_parent, parent_slug, reason, base_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT `+target+` DO UPDATE SET
			kind = EXCLUDED.kind, slug = EXCLUDED.slug, title = EXCLUDED.title, summary = EXCLUDED.summary,
			body = EXCLUDED.body, id_parent = EXCLUDED.id_parent, parent_slug = EXCLUDED.parent_slug,
			reason = EXCLUDED.reason, base_version = EXCLUDED.base_version,
			decision = $15, decided_by = NULL, decided_at = NULL, decision_note = NULL, agent_access = NULL,
			update_at = (now() at time zone 'utc')
		WHERE wiki.change_proposal.decision = ANY($16)
		RETURNING id_proposal, (xmax = 0)
	`, draft.IdRun, draft.IdIssue, draft.IdProject, draft.IdSpace, draft.Kind, draft.IdPage, draft.Slug,
		draft.Title, draft.Summary, draft.Body, draft.IdParent, draft.ParentSlug, draft.Reason, draft.BaseVersion,
		constants.WikiProposalOpen, constants.WikiProposalRevisable,
	).Scan(&idProposal, &inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, ErrWikiProposalDecided
	}
	if err != nil {
		return 0, false, fmt.Errorf("upserting wiki proposal: %w", err)
	}
	return idProposal, inserted, nil
}

func (r *WikiProposalRepository) Load(ctx context.Context, filter model.WikiProposalFilter) ([]*model.WikiProposal, error) {
	var where []string
	var args []any
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if filter.IdProject != nil {
		add("p.id_project = $%d", *filter.IdProject)
	}
	if filter.IdIssue != nil {
		add("p.id_issue = $%d", *filter.IdIssue)
	}
	if filter.IdRun != nil {
		add("p.id_run = $%d", *filter.IdRun)
	}
	if filter.IdsSpace != nil {
		add("p.id_space = ANY($%d)", filter.IdsSpace)
	}
	if filter.Decisions != nil {
		add("p.decision = ANY($%d)", filter.Decisions)
	}
	if filter.OnlyLive {
		add("r.phase <> ALL($%d)", []string{constants.PhaseFailed, constants.PhaseCancelled})
	}
	query := wikiProposalSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY p.create_at, p.id_proposal"
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying wiki proposals: %w", err)
	}
	return collectWikiProposals(rows)
}

func (r *WikiProposalRepository) CountReady(ctx context.Context, idProject int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int
	err := db.QueryRow(ctx, `
		SELECT count(*)::int
		FROM wiki.change_proposal p
		JOIN agent.run r ON r.id_run = p.id_run
		WHERE p.id_project = $1 AND ((p.decision = $2 AND r.phase = $3) OR p.decision = $4)
	`, idProject, constants.WikiProposalOpen, constants.PhaseDone, constants.WikiProposalNeedsResolving).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting ready wiki proposals: %w", err)
	}
	return count, nil
}

func (r *WikiProposalRepository) LoadOne(ctx context.Context, idProposal int64, lock bool) (*model.WikiProposal, error) {
	query := wikiProposalSelect + " WHERE p.id_proposal = $1"
	if lock {
		query += " FOR UPDATE OF p"
	}
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, query, idProposal)
	if err != nil {
		return nil, fmt.Errorf("querying wiki proposal: %w", err)
	}
	proposals, err := collectWikiProposals(rows)
	if err != nil {
		return nil, err
	}
	if len(proposals) == 0 {
		return nil, nil
	}
	return proposals[0], nil
}

func (r *WikiProposalRepository) Approve(ctx context.Context, idProposal int64, approval model.WikiProposalApproval) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE wiki.change_proposal
		SET decision = $2, decided_by = $3, decided_at = (now() at time zone 'utc'), title = $4, summary = $5,
		    body = COALESCE($6, body), base_version = COALESCE($7, base_version), agent_access = $8,
		    decision_note = $9, update_at = (now() at time zone 'utc')
		WHERE id_proposal = $1 AND decision = ANY($10)
	`, idProposal, constants.WikiProposalApproved, approval.IdUser, approval.Title, approval.Summary,
		approval.Body, approval.BaseVersion, approval.AgentAccess, approval.Note, constants.WikiProposalRevisable)
	if err != nil {
		return fmt.Errorf("approving wiki proposal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWikiProposalDecided
	}
	return nil
}

func (r *WikiProposalRepository) Decide(ctx context.Context, idProposal int64, decision constants.WikiProposalDecision, from []constants.WikiProposalDecision, idUser int64, note *string, idPage *int64, resultVersion *int) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `
		UPDATE wiki.change_proposal
		SET decision = $2, decided_by = $3, decided_at = (now() at time zone 'utc'), decision_note = $4,
		    id_page = COALESCE($5, id_page), result_version = $6, update_at = (now() at time zone 'utc')
		WHERE id_proposal = $1 AND decision = ANY($7)
	`, idProposal, decision, idUser, note, idPage, resultVersion, from)
	if err != nil {
		return fmt.Errorf("deciding wiki proposal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWikiProposalDecided
	}
	return nil
}

func (r *WikiProposalRepository) MarkNeedsResolving(ctx context.Context, idProposal int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE wiki.change_proposal SET decision = $2, update_at = (now() at time zone 'utc')
		WHERE id_proposal = $1 AND decision = $3
	`, idProposal, constants.WikiProposalNeedsResolving, constants.WikiProposalApproved)
	if err != nil {
		return fmt.Errorf("marking wiki proposal as needing resolution: %w", err)
	}
	return nil
}

func collectWikiProposals(rows pgx.Rows) ([]*model.WikiProposal, error) {
	proposals, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.WikiProposal])
	if err != nil {
		return nil, fmt.Errorf("collecting wiki proposals: %w", err)
	}
	for _, proposal := range proposals {
		proposal.State = constants.WikiProposalStateOf(proposal.Decision, proposal.RunPhase)
	}
	return proposals, nil
}
