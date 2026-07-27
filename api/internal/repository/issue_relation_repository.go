package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueRelationRepository struct {
	pool *pgxpool.Pool
}

func NewIssueRelationRepository(pool *pgxpool.Pool) *IssueRelationRepository {
	return &IssueRelationRepository{pool: pool}
}

// labelFor returns the human-readable label and its inverse for a given relation.
func labelFor(relationType string, relationSubType *string, direction string) (label, inverseLabel string) {
	switch relationType {
	case model.RelationTypeHierarchy:
		// Label is the linked issue's role relative to the current one: outbound means
		// current is the parent, inbound means current is the child.
		if direction == model.RelationDirectionOutbound {
			return "child", "parent"
		}
		return "parent", "child"
	case model.RelationTypeSchedule:
		sub := ""
		if relationSubType != nil {
			sub = *relationSubType
		}
		switch sub {
		case model.RelationSubTypeFinishToStart:
			if direction == model.RelationDirectionOutbound {
				return "schedules", "scheduled_by"
			}
			return "scheduled_by", "schedules"
		case model.RelationSubTypeStartToStart:
			return "starts_with", "starts_with"
		case model.RelationSubTypeFinishToFinish:
			return "finishes_with", "finishes_with"
		case model.RelationSubTypeStartToFinish:
			if direction == model.RelationDirectionOutbound {
				return "triggers_end_of", "end_triggered_by"
			}
			return "end_triggered_by", "triggers_end_of"
		}
	case model.RelationTypeDuplicates:
		return "duplicates", "duplicates"
	case model.RelationTypeRelatesTo:
		return "relates_to", "relates_to"
	}
	return relationType, relationType
}

func (r *IssueRelationRepository) LoadRelations(ctx context.Context, f *model.LoadRelationsFilter) ([]*model.ReadIssueRelationRes, error) {
	db := extctx.GetDb(ctx, r.pool)

	args := []any{f.IdsProject}
	idx := 2

	var outboundIssueFilter, inboundIssueFilter string
	if len(f.IdsIssue) > 0 {
		outboundIssueFilter = fmt.Sprintf("AND r.id_issue_from = ANY($%d)", idx)
		inboundIssueFilter = fmt.Sprintf("AND r.id_issue_to = ANY($%d)", idx)
		args = append(args, f.IdsIssue)
		idx++
	}
	// UNION ALL is intentional: each half targets a different indexed column
	// (idx_relation_project_from / _to) for an index scan. An OR on
	// (id_issue_from OR id_issue_to) can't use those and forces a seq/bitmap scan.
	q := fmt.Sprintf(`
		SELECT
			r.id_issue_relation,
			r.relation_type,
			r.relation_sub_type,
			r.lag_minutes,
			'%s'               AS direction,
			r.id_issue_from,
			fi.id_issue_public AS from_public,
			fi.title           AS from_title,
			fi.id_severity     AS from_id_severity,
			fi.id_state        AS from_id_state,
			fi.assigned_to     AS from_assigned_to,
			fi.update_at       AS from_update_at,
			fiq.score          AS from_quality_score,
			r.id_issue_to,
			ti.id_issue_public AS to_public,
			ti.title           AS to_title,
			ti.id_severity     AS to_id_severity,
			ti.id_state        AS to_id_state,
			ti.assigned_to     AS to_assigned_to,
			ti.update_at       AS to_update_at,
			tiq.score          AS to_quality_score,
			r.created_at,
			r.created_by
		FROM
			issues.issue_relation r
			JOIN  issues.issue         fi  ON fi.id_issue  = r.id_issue_from
			JOIN  issues.issue         ti  ON ti.id_issue  = r.id_issue_to
			LEFT JOIN issues.issue_quality fiq ON fiq.id_issue = r.id_issue_from
			LEFT JOIN issues.issue_quality tiq ON tiq.id_issue = r.id_issue_to
		WHERE
			r.id_project = ANY($1) %s

		UNION ALL

		SELECT
			r.id_issue_relation,
			r.relation_type,
			r.relation_sub_type,
			r.lag_minutes,
			'%s'               AS direction,
			r.id_issue_from,
			fi.id_issue_public AS from_public,
			fi.title           AS from_title,
			fi.id_severity     AS from_id_severity,
			fi.id_state        AS from_id_state,
			fi.assigned_to     AS from_assigned_to,
			fi.update_at       AS from_update_at,
			fiq.score          AS from_quality_score,
			r.id_issue_to,
			ti.id_issue_public AS to_public,
			ti.title           AS to_title,
			ti.id_severity     AS to_id_severity,
			ti.id_state        AS to_id_state,
			ti.assigned_to     AS to_assigned_to,
			ti.update_at       AS to_update_at,
			tiq.score          AS to_quality_score,
			r.created_at,
			r.created_by
		FROM
			issues.issue_relation r
			JOIN  issues.issue         fi  ON fi.id_issue  = r.id_issue_from
			JOIN  issues.issue         ti  ON ti.id_issue  = r.id_issue_to
			LEFT JOIN issues.issue_quality fiq ON fiq.id_issue = r.id_issue_from
			LEFT JOIN issues.issue_quality tiq ON tiq.id_issue = r.id_issue_to
		WHERE
			r.id_project = ANY($1) %s
	`, model.RelationDirectionOutbound, outboundIssueFilter,
		model.RelationDirectionInbound, inboundIssueFilter)

	_ = idx // used above when IdsIssue is non-empty

	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying relations: %w", err)
	}
	defer rows.Close()

	var views []*model.ReadIssueRelationRes
	for rows.Next() {
		var (
			idIssueRelation  int64
			relationType     string
			relationSubType  *string
			lagMinutes       *int64
			direction        string
			idIssueFrom      int64
			fromPublic       int64
			fromTitle        string
			fromIdSeverity   *int64
			fromIdState      *int64
			fromAssignedTo   *int64
			fromUpdateAt     time.Time
			fromQualityScore *int64
			idIssueTo        int64
			toPublic         int64
			toTitle          string
			toIdSeverity     *int64
			toIdState        *int64
			toAssignedTo     *int64
			toUpdateAt       time.Time
			toQualityScore   *int64
			createdAt        time.Time
			createdBy        int64
		)
		if err := rows.Scan(
			&idIssueRelation, &relationType, &relationSubType, &lagMinutes,
			&direction,
			&idIssueFrom, &fromPublic, &fromTitle, &fromIdSeverity, &fromIdState, &fromAssignedTo, &fromUpdateAt, &fromQualityScore,
			&idIssueTo, &toPublic, &toTitle, &toIdSeverity, &toIdState, &toAssignedTo, &toUpdateAt, &toQualityScore,
			&createdAt, &createdBy,
		); err != nil {
			return nil, fmt.Errorf("scanning relation: %w", err)
		}
		label, inverseLabel := labelFor(relationType, relationSubType, direction)
		views = append(views, &model.ReadIssueRelationRes{
			IdIssueRelation: idIssueRelation,
			RelationType:    relationType,
			RelationSubType: relationSubType,
			LagMinutes:      lagMinutes,
			Direction:       direction,
			Label:           label,
			InverseLabel:    inverseLabel,
			From:            model.IssueRelationRef{IdIssuePublic: fromPublic, Title: fromTitle, IdSeverity: fromIdSeverity, IdState: fromIdState, AssignedTo: fromAssignedTo, UpdateAt: fromUpdateAt, QualityScore: fromQualityScore},
			To:              model.IssueRelationRef{IdIssuePublic: toPublic, Title: toTitle, IdSeverity: toIdSeverity, IdState: toIdState, AssignedTo: toAssignedTo, UpdateAt: toUpdateAt, QualityScore: toQualityScore},
			CreatedAt:       createdAt,
			CreatedBy:       createdBy,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating relations: %w", err)
	}
	if views == nil {
		views = []*model.ReadIssueRelationRes{}
	}
	return views, nil
}

// LoadRelationById returns both the outbound and inbound views for a single relation row.
func (r *IssueRelationRepository) LoadRelationById(ctx context.Context, idIssueRelation int64) ([]*model.ReadIssueRelationRes, error) {
	db := extctx.GetDb(ctx, r.pool)
	q := fmt.Sprintf(`
		SELECT
			r.id_issue_relation,
			r.relation_type,
			r.relation_sub_type,
			r.lag_minutes,
			'%s'               AS direction,
			r.id_issue_from,
			fi.id_issue_public AS from_public,
			fi.title           AS from_title,
			fi.id_severity     AS from_id_severity,
			fi.id_state        AS from_id_state,
			fi.assigned_to     AS from_assigned_to,
			fi.update_at       AS from_update_at,
			fiq.score          AS from_quality_score,
			r.id_issue_to,
			ti.id_issue_public AS to_public,
			ti.title           AS to_title,
			ti.id_severity     AS to_id_severity,
			ti.id_state        AS to_id_state,
			ti.assigned_to     AS to_assigned_to,
			ti.update_at       AS to_update_at,
			tiq.score          AS to_quality_score,
			r.created_at,
			r.created_by
		FROM
			issues.issue_relation r
			JOIN  issues.issue         fi  ON fi.id_issue = r.id_issue_from
			JOIN  issues.issue         ti  ON ti.id_issue = r.id_issue_to
			LEFT JOIN issues.issue_quality fiq ON fiq.id_issue = r.id_issue_from
			LEFT JOIN issues.issue_quality tiq ON tiq.id_issue = r.id_issue_to
		WHERE r.id_issue_relation = $1

		UNION ALL

		SELECT
			r.id_issue_relation,
			r.relation_type,
			r.relation_sub_type,
			r.lag_minutes,
			'%s'               AS direction,
			r.id_issue_from,
			fi.id_issue_public AS from_public,
			fi.title           AS from_title,
			fi.id_severity     AS from_id_severity,
			fi.id_state        AS from_id_state,
			fi.assigned_to     AS from_assigned_to,
			fi.update_at       AS from_update_at,
			fiq.score          AS from_quality_score,
			r.id_issue_to,
			ti.id_issue_public AS to_public,
			ti.title           AS to_title,
			ti.id_severity     AS to_id_severity,
			ti.id_state        AS to_id_state,
			ti.assigned_to     AS to_assigned_to,
			ti.update_at       AS to_update_at,
			tiq.score          AS to_quality_score,
			r.created_at,
			r.created_by
		FROM
			issues.issue_relation r
			JOIN  issues.issue         fi  ON fi.id_issue = r.id_issue_from
			JOIN  issues.issue         ti  ON ti.id_issue = r.id_issue_to
			LEFT JOIN issues.issue_quality fiq ON fiq.id_issue = r.id_issue_from
			LEFT JOIN issues.issue_quality tiq ON tiq.id_issue = r.id_issue_to
		WHERE r.id_issue_relation = $1
	`, model.RelationDirectionOutbound, model.RelationDirectionInbound)

	rows, err := db.Query(ctx, q, idIssueRelation)
	if err != nil {
		return nil, fmt.Errorf("querying relation by id: %w", err)
	}
	defer rows.Close()

	var views []*model.ReadIssueRelationRes
	for rows.Next() {
		var (
			idRelation       int64
			relationType     string
			relationSubType  *string
			lagMinutes       *int64
			direction        string
			idIssueFrom      int64
			fromPublic       int64
			fromTitle        string
			fromIdSeverity   *int64
			fromIdState      *int64
			fromAssignedTo   *int64
			fromUpdateAt     time.Time
			fromQualityScore *int64
			idIssueTo        int64
			toPublic         int64
			toTitle          string
			toIdSeverity     *int64
			toIdState        *int64
			toAssignedTo     *int64
			toUpdateAt       time.Time
			toQualityScore   *int64
			createdAt        time.Time
			createdBy        int64
		)
		if err := rows.Scan(
			&idRelation, &relationType, &relationSubType, &lagMinutes,
			&direction,
			&idIssueFrom, &fromPublic, &fromTitle, &fromIdSeverity, &fromIdState, &fromAssignedTo, &fromUpdateAt, &fromQualityScore,
			&idIssueTo, &toPublic, &toTitle, &toIdSeverity, &toIdState, &toAssignedTo, &toUpdateAt, &toQualityScore,
			&createdAt, &createdBy,
		); err != nil {
			return nil, fmt.Errorf("scanning relation by id: %w", err)
		}
		label, inverseLabel := labelFor(relationType, relationSubType, direction)
		views = append(views, &model.ReadIssueRelationRes{
			IdIssueRelation: idRelation,
			RelationType:    relationType,
			RelationSubType: relationSubType,
			LagMinutes:      lagMinutes,
			Direction:       direction,
			Label:           label,
			InverseLabel:    inverseLabel,
			From:            model.IssueRelationRef{IdIssuePublic: fromPublic, Title: fromTitle, IdSeverity: fromIdSeverity, IdState: fromIdState, AssignedTo: fromAssignedTo, UpdateAt: fromUpdateAt, QualityScore: fromQualityScore},
			To:              model.IssueRelationRef{IdIssuePublic: toPublic, Title: toTitle, IdSeverity: toIdSeverity, IdState: toIdState, AssignedTo: toAssignedTo, UpdateAt: toUpdateAt, QualityScore: toQualityScore},
			CreatedAt:       createdAt,
			CreatedBy:       createdBy,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating relation by id: %w", err)
	}
	return views, nil
}

func (r *IssueRelationRepository) InsertRelation(ctx context.Context, rel *model.IssueRelation) (*model.IssueRelation, error) {
	db := extctx.GetDb(ctx, r.pool)
	q := `
		INSERT INTO issues.issue_relation
			(id_project, id_issue_from, id_issue_to, relation_type, relation_sub_type, lag_minutes, created_by)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		RETURNING id_issue_relation, created_at
	`
	err := db.QueryRow(ctx, q,
		rel.IdProject, rel.IdIssueFrom, rel.IdIssueTo,
		rel.RelationType, rel.RelationSubType, rel.LagMinutes, rel.CreatedBy,
	).Scan(&rel.IdIssueRelation, &rel.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting relation: %w", err)
	}
	return rel, nil
}

func (r *IssueRelationRepository) LoadRelation(ctx context.Context, idRelation int64) (*model.IssueRelation, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_issue_relation, id_project, id_issue_from, id_issue_to,
		       relation_type, relation_sub_type, lag_minutes, created_at, created_by
		FROM issues.issue_relation
		WHERE id_issue_relation = $1
	`, idRelation)
	if err != nil {
		return nil, fmt.Errorf("querying relation: %w", err)
	}
	rel, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.IssueRelation])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collecting relation: %w", err)
	}
	return rel, nil
}

func (r *IssueRelationRepository) UpdateRelation(ctx context.Context, idRelation int64, lagMinutes *int64) (*model.IssueRelation, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		UPDATE issues.issue_relation
		SET lag_minutes = $2
		WHERE id_issue_relation = $1
		RETURNING id_issue_relation, id_project, id_issue_from, id_issue_to,
		          relation_type, relation_sub_type, lag_minutes, created_at, created_by
	`, idRelation, lagMinutes)
	if err != nil {
		return nil, fmt.Errorf("updating relation: %w", err)
	}
	rel, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.IssueRelation])
	if err != nil {
		return nil, fmt.Errorf("collecting updated relation: %w", err)
	}
	return rel, nil
}

func (r *IssueRelationRepository) DeleteRelation(ctx context.Context, idRelation, idProject int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`DELETE FROM issues.issue_relation WHERE id_issue_relation = $1 AND id_project = $2`,
		idRelation, idProject,
	)
	if err != nil {
		return fmt.Errorf("deleting relation: %w", err)
	}
	return nil
}

// BulkInsertRelations inserts all relations in a loop. Must be called inside a transaction.
func (r *IssueRelationRepository) BulkInsertRelations(ctx context.Context, relations []model.IssueRelation) error {
	db := extctx.GetDb(ctx, r.pool)
	for _, rel := range relations {
		_, err := db.Exec(ctx, `
			INSERT INTO issues.issue_relation
				(id_project, id_issue_from, id_issue_to, relation_type, relation_sub_type, lag_minutes, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
			rel.IdProject, rel.IdIssueFrom, rel.IdIssueTo,
			rel.RelationType, rel.RelationSubType, rel.LagMinutes, rel.CreatedBy,
		)
		if err != nil {
			return fmt.Errorf("bulk inserting relation: %w", err)
		}
	}
	return nil
}

// HasCycle reports whether adding edge from→to for relationType would create a cycle.
// Only valid for "hierarchy" and "schedule". Call inside the same transaction as InsertRelation.
func (r *IssueRelationRepository) HasCycle(ctx context.Context, relationType string, from, to int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	q := `
		WITH RECURSIVE reachable(id_reach, depth) AS (
			SELECT id_issue_to, 1
			FROM issues.issue_relation
			WHERE id_issue_from = $1 AND relation_type = $2

			UNION ALL

			SELECT r.id_issue_to, rc.depth + 1
			FROM issues.issue_relation r
			INNER JOIN reachable rc ON r.id_issue_from = rc.id_reach
			WHERE r.relation_type = $2
			  AND rc.depth < 100
		)
		SELECT 1 FROM reachable WHERE id_reach = $3 LIMIT 1
	`
	var flag int
	err := db.QueryRow(ctx, q, to, relationType, from).Scan(&flag)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking relation cycle: %w", err)
	}
	return true, nil
}
