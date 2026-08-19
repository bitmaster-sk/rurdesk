package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var issueSortMap = map[string]string{
	"idIssue":        "iss.id_issue",
	"title":          "iss.title",
	"createAt":       "iss.create_at",
	"updateAt":       "iss.update_at",
	"tracked":        "iss.tracked",
	"estimated":      "iss.estimated",
	"scheduledAt":    "iss.scheduled_at",
	"severity":       "pis.order_rank",
	"state":          "pit.order_rank",
	"assignedToName": "ass.name",
	"qualityScore":   "iq.score",
}

type LoadIssueFilter struct {
	IdProject     *int64
	IdIssuePublic *int64
	IdIssue       *int64
}

func (f *LoadIssueFilter) IsValid() bool {
	return !(f.IdProject == nil && f.IdIssuePublic == nil && f.IdIssue == nil)
}

type IssueRepository struct {
	pool *pgxpool.Pool
}

func NewIssueRepository(pool *pgxpool.Pool) *IssueRepository {
	return &IssueRepository{pool: pool}
}

func (r *IssueRepository) LoadIssues(ctx context.Context, f *model.LoadIssuesFilter) ([]*model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)

	// No cursor here, so a window resolves against now — right for a live view.
	f, _ = resolveWithinWindows(f, nil)

	var (
		sb   strings.Builder
		args []any
		idx  = 1
	)

	sb.WriteString(`
		SELECT
			iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state,
			iss.id_severity, iss.title, iss.description, iss.create_at, iss.update_at,
			iss.create_by, iss.update_by, iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
			iq.score AS quality_score, iss.id_git_integration, iss.mr_id, iss.gantt_rank, iss.id_sprint, iss.points, iss.carryover_count,
			(SELECT count(*) FROM issues.issue_relation r
			   WHERE r.id_issue_from = iss.id_issue OR r.id_issue_to = iss.id_issue) AS relation_count
		FROM issues.issue iss
		LEFT JOIN projects.project_issue_severity pis ON pis.id_project = iss.id_project AND pis.id_severity = iss.id_severity
		LEFT JOIN projects.project_issue_state pit ON pit.id_project = iss.id_project AND pit.id_state = iss.id_state
		LEFT JOIN users.user ass ON ass.id_user = iss.assigned_to
		LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue
	`)

	if f.IdProject != 0 {
		fmt.Fprintf(&sb, "WHERE iss.id_project = $%d ", idx)
		args = append(args, f.IdProject)
		idx++
	} else {
		sb.WriteString("WHERE 1=1 ")
	}

	r.appendIssueFilters(&sb, &args, &idx, f)

	if f.Order != nil {
		col, ok := issueSortMap[f.Order.Column]
		if ok {
			dir := "DESC"
			if f.Order.Direction == "asc" {
				dir = "ASC"
			}
			fmt.Fprintf(&sb, "ORDER BY %s %s NULLS LAST ", col, dir)
		}
	}

	if f.Limit != nil {
		limit := *f.Limit
		if limit < 1 {
			limit = 1
		}
		if limit > 200 {
			limit = 200
		}
		fmt.Fprintf(&sb, "LIMIT $%d ", idx)
		args = append(args, limit)
		idx++
	}

	if f.Offset != nil && *f.Offset > 0 {
		fmt.Fprintf(&sb, "OFFSET $%d ", idx)
		args = append(args, *f.Offset)
		idx++
	}

	_ = idx

	rows, err := db.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("querying issues: %w", err)
	}
	issues, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[model.Issue])
	if err != nil {
		return nil, fmt.Errorf("collecting issues: %w", err)
	}
	return issues, nil
}

// appendIssueFilters writes the shared WHERE clauses for f, advancing *idx and *args.
// Caller must already have written the project predicate. Shared by list, page, count,
// and grouped queries — add new filters only here.
func (r *IssueRepository) appendIssueFilters(sb *strings.Builder, args *[]any, idx *int, f *model.LoadIssuesFilter) {
	if len(f.IdsSeverity) > 0 {
		if f.SeverityUnset {
			fmt.Fprintf(sb, "AND (iss.id_severity = ANY($%d) OR iss.id_severity IS NULL) ", *idx)
		} else {
			fmt.Fprintf(sb, "AND iss.id_severity = ANY($%d) ", *idx)
		}
		*args = append(*args, f.IdsSeverity)
		(*idx)++
	}

	if len(f.IdsState) > 0 {
		if f.StateUnset {
			fmt.Fprintf(sb, "AND (iss.id_state = ANY($%d) OR iss.id_state IS NULL) ", *idx)
		} else {
			fmt.Fprintf(sb, "AND iss.id_state = ANY($%d) ", *idx)
		}
		*args = append(*args, f.IdsState)
		(*idx)++
	}

	if len(f.IdsAssignedTo) > 0 {
		if f.AssignedToUnset {
			fmt.Fprintf(sb, "AND (iss.assigned_to = ANY($%d) OR iss.assigned_to IS NULL) ", *idx)
		} else {
			fmt.Fprintf(sb, "AND iss.assigned_to = ANY($%d) ", *idx)
		}
		*args = append(*args, f.IdsAssignedTo)
		(*idx)++
	}

	if len(f.IdsIssuePublic) > 0 {
		fmt.Fprintf(sb, "AND iss.id_issue_public = ANY($%d) ", *idx)
		*args = append(*args, f.IdsIssuePublic)
		(*idx)++
	}

	if f.Search != nil {
		fmt.Fprintf(sb, "AND to_tsvector('english', coalesce(iss.title, '') || ' ' || coalesce(iss.description, '')) @@ plainto_tsquery('english', $%d) ", *idx)
		*args = append(*args, *f.Search)
		(*idx)++
	} else if f.Title != "" {
		fmt.Fprintf(sb, "AND iss.title ILIKE $%d ", *idx)
		*args = append(*args, "%"+f.Title+"%")
		(*idx)++
	}

	if f.ExcludeFinalStates {
		sb.WriteString("AND NOT EXISTS (SELECT 1 FROM issues.state s WHERE s.id_state = iss.id_state AND s.final = TRUE) ")
	}

	if !f.CreateAtFrom.IsZero() {
		fmt.Fprintf(sb, "AND iss.create_at >= $%d ", *idx)
		*args = append(*args, f.CreateAtFrom)
		(*idx)++
	}
	if !f.CreateAtTo.IsZero() {
		fmt.Fprintf(sb, "AND iss.create_at <= $%d ", *idx)
		*args = append(*args, f.CreateAtTo)
		(*idx)++
	}
	if !f.UpdateAtFrom.IsZero() {
		fmt.Fprintf(sb, "AND iss.update_at >= $%d ", *idx)
		*args = append(*args, f.UpdateAtFrom)
		(*idx)++
	}
	if !f.UpdateAtTo.IsZero() {
		fmt.Fprintf(sb, "AND iss.update_at <= $%d ", *idx)
		*args = append(*args, f.UpdateAtTo)
		(*idx)++
	}
	if !f.ScheduledAtFrom.IsZero() {
		fmt.Fprintf(sb, "AND iss.scheduled_at >= $%d ", *idx)
		*args = append(*args, f.ScheduledAtFrom)
		(*idx)++
	}
	if !f.ScheduledAtTo.IsZero() {
		fmt.Fprintf(sb, "AND iss.scheduled_at <= $%d ", *idx)
		*args = append(*args, f.ScheduledAtTo)
		(*idx)++
	}
	if f.ScheduledAtUnset {
		sb.WriteString("AND iss.scheduled_at IS NULL ")
	}

	if f.AssignedToNull {
		sb.WriteString("AND iss.assigned_to IS NULL ")
	}

	if f.SprintUnset {
		sb.WriteString("AND iss.id_sprint IS NULL ")
	} else if f.IdSprint != nil {
		fmt.Fprintf(sb, "AND iss.id_sprint = $%d ", *idx)
		*args = append(*args, *f.IdSprint)
		(*idx)++
	}
}

// issueRow wraps an Issue plus the captured sort value used to build the next cursor.
type issueRow struct {
	model.Issue
	SortVal any `db:"__sort_val"`
}

// CountIssues returns the exact number of issues matching the filter (same WHERE as the list).
func (r *IssueRepository) CountIssues(ctx context.Context, f *model.LoadIssuesFilter) (int, error) {
	db := extctx.GetDb(ctx, r.pool)

	// Same pinned instant as the page query, so Total matches the traversal.
	var cur *issueCursor
	if f.Cursor != nil && *f.Cursor != "" {
		decoded, err := decodeCursor(*f.Cursor)
		if err != nil {
			return 0, err
		}
		cur = decoded
	}
	f, _ = resolveWithinWindows(f, cur)

	var sb strings.Builder
	args := []any{}
	idx := 1
	sb.WriteString(`SELECT count(*) FROM issues.issue iss
		LEFT JOIN projects.project_issue_severity pis ON pis.id_project = iss.id_project AND pis.id_severity = iss.id_severity
		LEFT JOIN projects.project_issue_state pit ON pit.id_project = iss.id_project AND pit.id_state = iss.id_state
		LEFT JOIN users.user ass ON ass.id_user = iss.assigned_to
		LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue `)
	if f.IdProject != 0 {
		fmt.Fprintf(&sb, "WHERE iss.id_project = $%d ", idx)
		args = append(args, f.IdProject)
		idx++
	} else {
		sb.WriteString("WHERE 1=1 ")
	}
	r.appendIssueFilters(&sb, &args, &idx, f)
	var total int
	if err := db.QueryRow(ctx, sb.String(), args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("counting issues: %w", err)
	}
	return total, nil
}

// LoadIssuesPage returns up to limit issues plus the cursor for the next page (nil = last page).
func (r *IssueRepository) LoadIssuesPage(ctx context.Context, f *model.LoadIssuesFilter, limit int) ([]*model.Issue, *string, error) {
	db := extctx.GetDb(ctx, r.pool)

	sortKey := "updateAt"
	dir := "desc"
	if f.Order != nil && f.Order.Column != "" {
		sortKey = f.Order.Column
		if f.Order.Direction == "asc" {
			dir = "asc"
		}
	}
	resolvedKey, sc := sortColumnFor(sortKey)

	// Decoded before the WHERE is built — it carries the window's pinned instant.
	var cur *issueCursor
	if f.Cursor != nil && *f.Cursor != "" {
		decoded, err := decodeCursor(*f.Cursor)
		if err != nil {
			return nil, nil, err
		}
		cur = decoded
	}
	f, ref := resolveWithinWindows(f, cur)

	var sb strings.Builder
	args := []any{}
	idx := 1
	fmt.Fprintf(&sb, `
		SELECT iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state,
			iss.id_severity, iss.title, iss.description, iss.create_at, iss.update_at,
			iss.create_by, iss.update_by, iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
			iq.score AS quality_score, iss.id_git_integration, iss.mr_id, iss.gantt_rank, iss.id_sprint, iss.points, iss.carryover_count,
			(SELECT count(*) FROM issues.issue_relation rr WHERE rr.id_issue_from = iss.id_issue OR rr.id_issue_to = iss.id_issue) AS relation_count,
			%s AS __sort_val
		FROM issues.issue iss
		LEFT JOIN projects.project_issue_severity pis ON pis.id_project = iss.id_project AND pis.id_severity = iss.id_severity
		LEFT JOIN projects.project_issue_state pit ON pit.id_project = iss.id_project AND pit.id_state = iss.id_state
		LEFT JOIN users.user ass ON ass.id_user = iss.assigned_to
		LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue `, sc.expr)

	fmt.Fprintf(&sb, "WHERE iss.id_project = $%d ", idx)
	args = append(args, f.IdProject)
	idx++
	r.appendIssueFilters(&sb, &args, &idx, f)

	if cur != nil {
		pred, cArgs, nIdx, err := buildKeysetPredicate(cur, idx)
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(&sb, "AND %s ", pred)
		args = append(args, cArgs...)
		idx = nIdx
	}

	sqlDir := "DESC"
	if dir == "asc" {
		sqlDir = "ASC"
	}
	fmt.Fprintf(&sb, "ORDER BY %s %s NULLS LAST, iss.id_issue %s LIMIT $%d", sc.expr, sqlDir, sqlDir, idx)
	args = append(args, limit+1) // probe one extra row to detect a next page

	rows, err := db.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("querying issues page: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[issueRow])
	if err != nil {
		return nil, nil, fmt.Errorf("collecting issues page: %w", err)
	}

	var next *string
	if len(collected) > limit {
		last := collected[limit-1]
		c := issueCursor{
			Col: resolvedKey, Dir: dir, Val: normalizeSortVal(last.SortVal), Id: last.IdIssue,
			WithinRef: withinRef(f, ref),
		}
		enc, err := encodeCursor(c)
		if err != nil {
			return nil, nil, err
		}
		next = &enc
		collected = collected[:limit]
	}

	items := make([]*model.Issue, len(collected))
	for i, row := range collected {
		iss := row.Issue
		items[i] = &iss
	}
	return items, next, nil
}

// groupedRow wraps an Issue with the window-function bookkeeping columns.
type groupedRow struct {
	model.Issue
	Rn         int `db:"__rn"`
	GroupTotal int `db:"__group_total"`
	SortVal    any `db:"__sort_val"`
}

var groupByAllow = map[string]string{
	"state":      "iss.id_state",
	"assignedTo": "iss.assigned_to",
	"sprint":     "iss.id_sprint",
}

func derefInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func groupKeyOf(iss model.Issue, keys []string) map[string]any {
	m := map[string]any{}
	for _, k := range keys {
		switch k {
		case "state":
			m["idState"] = derefInt64(iss.IdState)
		case "assignedTo":
			m["assignedTo"] = derefInt64(iss.AssignedTo)
		case "sprint":
			m["idSprint"] = derefInt64(iss.IdSprint)
		}
	}
	return m
}

// LoadIssuesGrouped returns top-N issues per group plus each group's total, in one window-fn query.
func (r *IssueRepository) LoadIssuesGrouped(ctx context.Context, f *model.LoadIssuesFilter, groupKeys []string, perGroup int) ([]*model.IssueGroupRes, error) {
	db := extctx.GetDb(ctx, r.pool)
	cols := make([]string, 0, len(groupKeys))
	for _, k := range groupKeys {
		col, ok := groupByAllow[k]
		if !ok {
			return nil, fmt.Errorf("invalid groupBy %q", k)
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("groupBy requires at least one key")
	}
	partition := strings.Join(cols, ", ")
	outerOrder := strings.ReplaceAll(partition, "iss.", "")

	// No incoming cursor, but the per-group cursors emitted below are fed to the flat
	// paged query, so they must carry the same instant or the window unpins there.
	f, ref := resolveWithinWindows(f, nil)

	// Resolve the requested sort (default updateAt DESC) for the ROW_NUMBER window, which
	// picks and orders the top-N per group. sc.expr comes from the allow-list and dir is a
	// literal ASC/DESC, so neither is user-controlled SQL.
	sortKey := "updateAt"
	dir := "DESC"
	cursorDir := "desc"
	if f.Order != nil && f.Order.Column != "" {
		sortKey = f.Order.Column
		if f.Order.Direction == "asc" {
			dir = "ASC"
			cursorDir = "asc"
		}
	}
	resolvedKey, sc := sortColumnFor(sortKey)
	windowOrder := fmt.Sprintf("%s %s NULLS LAST, iss.id_issue %s", sc.expr, dir, dir)

	var sb strings.Builder
	args := []any{}
	idx := 1
	fmt.Fprintf(&sb, `
		SELECT * FROM (
			SELECT iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state,
				iss.id_severity, iss.title, iss.description, iss.create_at, iss.update_at,
				iss.create_by, iss.update_by, iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
				iq.score AS quality_score, iss.id_git_integration, iss.mr_id, iss.gantt_rank, iss.id_sprint, iss.points, iss.carryover_count,
				(SELECT count(*) FROM issues.issue_relation rr WHERE rr.id_issue_from = iss.id_issue OR rr.id_issue_to = iss.id_issue) AS relation_count,
				%s AS __sort_val,
				ROW_NUMBER() OVER (PARTITION BY %s ORDER BY %s) AS __rn,
				COUNT(*)     OVER (PARTITION BY %s) AS __group_total
			FROM issues.issue iss
			LEFT JOIN projects.project_issue_severity pis ON pis.id_project = iss.id_project AND pis.id_severity = iss.id_severity
			LEFT JOIN projects.project_issue_state pit ON pit.id_project = iss.id_project AND pit.id_state = iss.id_state
			LEFT JOIN users.user ass ON ass.id_user = iss.assigned_to
			LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue
			WHERE iss.id_project = $%d `, sc.expr, partition, windowOrder, partition, idx)
	args = append(args, f.IdProject)
	idx++
	r.appendIssueFilters(&sb, &args, &idx, f)
	fmt.Fprintf(&sb, ") t WHERE __rn <= $%d ORDER BY %s, __rn", idx, outerOrder)
	args = append(args, perGroup)

	rows, err := db.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("querying grouped issues: %w", err)
	}
	scanned, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[groupedRow])
	if err != nil {
		return nil, fmt.Errorf("collecting grouped issues: %w", err)
	}

	order := []string{}
	byKey := map[string]*model.IssueGroupRes{}
	for _, row := range scanned {
		key := groupKeyOf(row.Issue, groupKeys)
		ks := fmt.Sprint(key)
		g, ok := byKey[ks]
		if !ok {
			g = &model.IssueGroupRes{Key: key, Items: []*model.Issue{}, Total: row.GroupTotal}
			byKey[ks] = g
			order = append(order, ks)
		}
		iss := row.Issue
		g.Items = append(g.Items, &iss)
		if row.GroupTotal > perGroup && row.Rn == perGroup {
			// The cursor must speak the same sort the window ordered by — the client
			// feeds it straight to the flat paged query, which filters with it.
			// Mismatched, the next page filters on one column while ordering by
			// another: rows past the boundary vanish and shown rows repeat.
			c := issueCursor{
				Col: resolvedKey, Dir: cursorDir, Val: normalizeSortVal(row.SortVal), Id: iss.IdIssue,
				WithinRef: withinRef(f, ref),
			}
			if enc, err := encodeCursor(c); err == nil {
				g.NextCursor = &enc
			}
		}
	}

	out := make([]*model.IssueGroupRes, 0, len(order))
	for _, ks := range order {
		out = append(out, byKey[ks])
	}
	return out, nil
}

func (r *IssueRepository) LoadIssue(ctx context.Context, filter *LoadIssueFilter) (*model.Issue, error) {
	if !filter.IsValid() {
		return nil, fmt.Errorf("invalid load issue filter")
	}
	db := extctx.GetDb(ctx, r.pool)

	var rows pgx.Rows
	var err error

	if filter.IdIssuePublic != nil && filter.IdProject != nil {
		rows, err = db.Query(ctx, `
			SELECT iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state, iss.id_severity,
				iss.title, iss.description, iss.create_at, iss.update_at, iss.create_by, iss.update_by,
				iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
				iq.score AS quality_score, iss.id_git_integration, iss.mr_id, iss.gantt_rank, iss.id_sprint, iss.points, iss.carryover_count
			FROM issues.issue iss
			LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue
			WHERE iss.id_project = $1 AND iss.id_issue_public = $2
		`, *filter.IdProject, *filter.IdIssuePublic)
	} else {
		rows, err = db.Query(ctx, `
			SELECT iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state, iss.id_severity,
				iss.title, iss.description, iss.create_at, iss.update_at, iss.create_by, iss.update_by,
				iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
				iq.score AS quality_score, iss.id_git_integration, iss.mr_id, iss.gantt_rank, iss.id_sprint, iss.points, iss.carryover_count
			FROM issues.issue iss
			LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue
			WHERE iss.id_issue = $1
		`, *filter.IdIssue)
	}
	if err != nil {
		return nil, fmt.Errorf("querying issue: %w", err)
	}
	issue, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByNameLax[model.Issue])
	if err != nil {
		return nil, fmt.Errorf("collecting issue: %w", err)
	}
	return issue, nil
}

func (r *IssueRepository) InsertIssue(ctx context.Context, issue *model.Issue) (*model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.issue (id_project, id_state, title, description, create_by, update_by, assigned_to, id_severity, estimated, scheduled_at, idempotency_key, points)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id_issue, id_issue_public, create_at, update_at
	`,
		issue.IdProject, issue.IdState, issue.Title, issue.Description,
		issue.CreateBy, issue.UpdateBy, issue.AssignedTo, issue.IdSeverity,
		issue.Estimated, issue.ScheduledAt, issue.IdempotencyKey, issue.Points,
	).Scan(&issue.IdIssue, &issue.IdIssuePublic, &issue.CreateAt, &issue.UpdateAt)
	if err != nil {
		return nil, fmt.Errorf("inserting issue: %w", err)
	}
	return issue, nil
}

func (r *IssueRepository) FindByIdempotencyKey(ctx context.Context, idUser int64, key string) (*model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state,
			iss.id_severity, iss.title, iss.description, iss.create_at, iss.update_at,
			iss.create_by, iss.update_by, iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
			iq.score AS quality_score, iss.id_sprint, iss.points, iss.carryover_count
		FROM issues.issue iss
		LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue
		WHERE iss.create_by = $1 AND iss.idempotency_key = $2
		  AND iss.create_at > (now() AT TIME ZONE 'utc') - INTERVAL '24 hours'
	`, idUser, key)
	if err != nil {
		return nil, fmt.Errorf("querying issue by idempotency key: %w", err)
	}
	issue, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByNameLax[model.Issue])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting issue by idempotency key: %w", err)
	}
	return issue, nil
}

func (r *IssueRepository) UpdateIssue(ctx context.Context, issue *model.Issue) (*model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		UPDATE issues.issue SET
			id_state          = $1,
			title             = $2,
			description       = $3,
			update_at         = now() at time zone 'utc',
			update_by         = $4,
			assigned_to       = $5,
			id_severity       = $6,
			estimated         = $7,
			scheduled_at      = $8,
			gantt_rank        = CASE WHEN $8::timestamp IS NULL THEN NULL ELSE gantt_rank END,
			id_git_integration = $9,
			mr_id             = $10,
			points            = $11
		WHERE id_issue = $12
		RETURNING update_at
	`,
		issue.IdState, issue.Title, issue.Description, issue.UpdateBy,
		issue.AssignedTo, issue.IdSeverity, issue.Estimated, issue.ScheduledAt,
		issue.IdGitIntegration, issue.MrId, issue.Points,
		issue.IdIssue,
	).Scan(&issue.UpdateAt)
	if err != nil {
		return nil, fmt.Errorf("updating issue %d: %w", issue.IdIssue, err)
	}
	return issue, nil
}

type GanttRankRow struct {
	IdIssue       int64   `db:"id_issue"`
	IdIssuePublic int64   `db:"id_issue_public"`
	GanttRank     *string `db:"gantt_rank"`
}

// LoadScheduledGanttRanks returns every scheduled issue's gantt_rank (nil when unset),
// used to resolve public ids and neighbour ranks for a reorder.
func (r *IssueRepository) LoadScheduledGanttRanks(ctx context.Context, idProject int64) ([]GanttRankRow, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_issue, id_issue_public, gantt_rank
		FROM issues.issue
		WHERE id_project = $1 AND scheduled_at IS NOT NULL
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying scheduled gantt ranks: %w", err)
	}
	ranks, err := pgx.CollectRows(rows, pgx.RowToStructByName[GanttRankRow])
	if err != nil {
		return nil, fmt.Errorf("collecting scheduled gantt ranks: %w", err)
	}
	return ranks, nil
}

// SetGanttRank writes only gantt_rank; update_at/update_by are deliberately left
// untouched — a reorder is an ordering change, not an audited edit (cf. LinkMr).
func (r *IssueRepository) SetGanttRank(ctx context.Context, idIssue int64, rank string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE issues.issue SET gantt_rank = $1 WHERE id_issue = $2`,
		rank, idIssue,
	)
	if err != nil {
		return fmt.Errorf("setting gantt rank for issue %d: %w", idIssue, err)
	}
	return nil
}

func (r *IssueRepository) UpdateIssueState(ctx context.Context, idIssue int64, idState int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE issues.issue SET id_state = $1, update_at = now() at time zone 'utc' WHERE id_issue = $2`,
		idState, idIssue,
	)
	if err != nil {
		return fmt.Errorf("updating issue %d state: %w", idIssue, err)
	}
	return nil
}

// LinkMr writes (idGitIntegration, mrId) without touching other editable fields. Lets a
// PR registered via the set_run_pr MCP tool show up in the issue UI's "Merge request"
// panel, which reads issues.issue.{id_git_integration, mr_id}, not agent.run.
//
// We deliberately don't write update_by/update_at: this is a system linkage from the
// agent gateway, not a user edit, and shouldn't pollute "Last updated" with a bot id.
func (r *IssueRepository) LinkMr(
	ctx context.Context, idIssue, idGitIntegration int64, mrId string,
) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE issues.issue SET id_git_integration = $1, mr_id = $2 WHERE id_issue = $3`,
		idGitIntegration, mrId, idIssue,
	)
	if err != nil {
		return fmt.Errorf("linking mr for issue %d: %w", idIssue, err)
	}
	return nil
}

func (r *IssueRepository) DeleteIssue(ctx context.Context, idProject int64, idIssuePublic int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`DELETE FROM issues.issue WHERE id_project = $1 AND id_issue_public = $2`,
		idProject, idIssuePublic)
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
	}
	return nil
}

func (r *IssueRepository) LoadIssuesByIds(ctx context.Context, f *model.LoadIssuesFilter) ([]*model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT
			iss.id_issue, iss.id_issue_public, iss.id_project, iss.id_state,
			iss.id_severity, iss.title, iss.description, iss.create_at, iss.update_at,
			iss.create_by, iss.update_by, iss.assigned_to, iss.tracked, iss.estimated, iss.scheduled_at,
			iq.score AS quality_score, iss.id_git_integration, iss.mr_id, iss.gantt_rank, iss.id_sprint, iss.points, iss.carryover_count
		FROM issues.issue iss
		LEFT JOIN issues.issue_quality iq ON iq.id_issue = iss.id_issue
		WHERE iss.id_issue = ANY($1)
	`, f.IdsIssue)
	if err != nil {
		return nil, fmt.Errorf("querying issues by ids: %w", err)
	}
	issues, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[model.Issue])
	if err != nil {
		return nil, fmt.Errorf("collecting issues by ids: %w", err)
	}
	return issues, nil
}

// BulkInsertIssues inserts each issue and populates its generated fields via RETURNING.
// Must be called inside a transaction.
func (r *IssueRepository) BulkInsertIssues(ctx context.Context, issues []model.Issue) ([]model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)
	result := make([]model.Issue, len(issues))
	for i, iss := range issues {
		row := db.QueryRow(ctx, `
			INSERT INTO issues.issue
				(id_project, id_state, title, description, create_by, update_by, assigned_to, id_severity, estimated, scheduled_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id_issue, id_issue_public, create_at, update_at
		`,
			iss.IdProject, iss.IdState, iss.Title, iss.Description,
			iss.CreateBy, iss.UpdateBy, iss.AssignedTo, iss.IdSeverity,
			iss.Estimated, iss.ScheduledAt,
		)
		inserted := iss
		if err := row.Scan(&inserted.IdIssue, &inserted.IdIssuePublic, &inserted.CreateAt, &inserted.UpdateAt); err != nil {
			return nil, fmt.Errorf("bulk inserting issue: %w", err)
		}
		result[i] = inserted
	}
	return result, nil
}

// BulkUpdateIssues applies partial updates (only non-nil fields per entry) to multiple
// issues. Must be called within a transaction context.
func (r *IssueRepository) BulkUpdateIssues(
	ctx context.Context,
	idProject int64,
	entries []model.BulkEditIssueEntryReq,
	idUser int64,
) ([]*model.Issue, error) {
	db := extctx.GetDb(ctx, r.pool)

	publicIds := make([]int64, len(entries))
	for i, entry := range entries {
		publicIds[i] = entry.IdIssuePublic
	}

	existingIssues, err := r.LoadIssues(ctx, &model.LoadIssuesFilter{
		IdProject:      idProject,
		IdsIssuePublic: publicIds,
	})
	if err != nil {
		return nil, fmt.Errorf("loading issues for bulk update: %w", err)
	}

	issueMap := make(map[int64]*model.Issue, len(existingIssues))
	for _, issue := range existingIssues {
		issueMap[issue.IdIssuePublic] = issue
	}

	for _, entry := range entries {
		if _, ok := issueMap[entry.IdIssuePublic]; !ok {
			return nil, fmt.Errorf("issue %d not found in project %d", entry.IdIssuePublic, idProject)
		}
	}

	idIssues := make([]int64, 0, len(entries))
	estimatedValues := make([]int64, 0, len(entries))
	scheduledAtValues := make([]*time.Time, 0, len(entries))
	stateValues := make([]*int64, 0, len(entries))
	severityValues := make([]*int64, 0, len(entries))
	assignedToValues := make([]*int64, 0, len(entries))

	for _, entry := range entries {
		issue := issueMap[entry.IdIssuePublic]
		idIssues = append(idIssues, issue.IdIssue)

		if entry.Estimated != nil {
			estimatedValues = append(estimatedValues, *entry.Estimated)
		} else {
			estimatedValues = append(estimatedValues, issue.Estimated)
		}

		if entry.ScheduledAt != nil {
			scheduledAtValues = append(scheduledAtValues, entry.ScheduledAt)
		} else {
			scheduledAtValues = append(scheduledAtValues, issue.ScheduledAt)
		}

		if entry.IdState != nil {
			stateValues = append(stateValues, entry.IdState)
		} else {
			stateValues = append(stateValues, issue.IdState)
		}

		if entry.IdSeverity != nil {
			severityValues = append(severityValues, entry.IdSeverity)
		} else {
			severityValues = append(severityValues, issue.IdSeverity)
		}

		if entry.IdUserAssigned != nil {
			assignedToValues = append(assignedToValues, entry.IdUserAssigned)
		} else {
			assignedToValues = append(assignedToValues, issue.AssignedTo)
		}
	}

	_, err = db.Exec(ctx, `
		UPDATE issues.issue AS iss SET
			estimated    = batch.estimated,
			scheduled_at = batch.scheduled_at,
			id_state     = batch.id_state,
			id_severity  = batch.id_severity,
			assigned_to  = batch.assigned_to,
			update_at    = now() at time zone 'utc',
			update_by    = $7
		FROM UNNEST($1::bigint[], $2::bigint[], $3::timestamptz[], $4::bigint[], $5::bigint[], $6::bigint[])
			AS batch(id_issue, estimated, scheduled_at, id_state, id_severity, assigned_to)
		WHERE iss.id_issue = batch.id_issue
	`, idIssues, estimatedValues, scheduledAtValues, stateValues, severityValues, assignedToValues, idUser)
	if err != nil {
		return nil, fmt.Errorf("bulk updating issues: %w", err)
	}

	return r.LoadIssues(ctx, &model.LoadIssuesFilter{
		IdProject:      idProject,
		IdsIssuePublic: publicIds,
	})
}
