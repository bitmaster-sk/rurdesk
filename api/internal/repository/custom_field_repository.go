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

type CustomFieldRepository struct {
	pool *pgxpool.Pool
}

func NewCustomFieldRepository(pool *pgxpool.Pool) *CustomFieldRepository {
	return &CustomFieldRepository{pool: pool}
}

const customFieldColumns = `
	id_custom_field, id_project, key, name, field_type, is_required, required_since,
	default_value, order_rank, archived_at
`

func (r *CustomFieldRepository) LoadCustomFields(ctx context.Context, idsProject []int64) ([]*model.CustomField, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT `+customFieldColumns+`
		FROM issues.custom_field
		WHERE id_project = ANY($1)
		ORDER BY id_project, order_rank, id_custom_field
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying custom fields: %w", err)
	}
	fields, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.CustomField])
	if err != nil {
		return nil, fmt.Errorf("collecting custom fields: %w", err)
	}
	return r.attachOptions(ctx, fields)
}

func (r *CustomFieldRepository) LoadCustomFieldsByProject(ctx context.Context, idProject int64) ([]*model.CustomField, error) {
	return r.LoadCustomFields(ctx, []int64{idProject})
}

func (r *CustomFieldRepository) LoadCustomField(ctx context.Context, idProject, idCustomField int64) (*model.CustomField, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT `+customFieldColumns+`
		FROM issues.custom_field
		WHERE id_project = $1 AND id_custom_field = $2
	`, idProject, idCustomField)
	if err != nil {
		return nil, fmt.Errorf("querying custom field: %w", err)
	}
	field, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.CustomField])
	if err != nil {
		return nil, fmt.Errorf("collecting custom field: %w", err)
	}
	withOptions, err := r.attachOptions(ctx, []*model.CustomField{field})
	if err != nil {
		return nil, err
	}
	return withOptions[0], nil
}

func (r *CustomFieldRepository) attachOptions(ctx context.Context, fields []*model.CustomField) ([]*model.CustomField, error) {
	if len(fields) == 0 {
		return fields, nil
	}
	ids := make([]int64, len(fields))
	byId := make(map[int64]*model.CustomField, len(fields))
	for i, field := range fields {
		ids[i] = field.IdCustomField
		field.Options = []model.CustomFieldOption{}
		byId[field.IdCustomField] = field
	}

	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_option, id_custom_field, label, order_rank
		FROM issues.custom_field_option
		WHERE id_custom_field = ANY($1)
		ORDER BY id_custom_field, order_rank, id_option
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("querying custom field options: %w", err)
	}
	options, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.CustomFieldOption])
	if err != nil {
		return nil, fmt.Errorf("collecting custom field options: %w", err)
	}
	for _, option := range options {
		if field, ok := byId[option.IdCustomField]; ok {
			field.Options = append(field.Options, option)
		}
	}
	return fields, nil
}

func (r *CustomFieldRepository) LoadCustomFieldUsage(ctx context.Context, idProject, idCustomField int64) (*model.CustomFieldUsage, error) {
	db := extctx.GetDb(ctx, r.pool)
	usage := &model.CustomFieldUsage{OptionUsage: map[int64]int{}}
	err := db.QueryRow(ctx, `
		SELECT count(*)
		FROM issues.issue_custom_value v
		INNER JOIN issues.issue i ON i.id_issue = v.id_issue
		WHERE v.id_custom_field = $2 AND i.id_project = $1
	`, idProject, idCustomField).Scan(&usage.Issues)
	if err != nil {
		return nil, fmt.Errorf("querying custom field usage: %w", err)
	}

	err = db.QueryRow(ctx, `
		SELECT count(*)
		FROM issues.issue i
		WHERE i.id_project = $1
		  AND NOT EXISTS (
			SELECT 1 FROM issues.issue_custom_value v
			WHERE v.id_issue = i.id_issue AND v.id_custom_field = $2
		  )
	`, idProject, idCustomField).Scan(&usage.IssuesMissing)
	if err != nil {
		return nil, fmt.Errorf("querying custom field missing usage: %w", err)
	}

	rows, err := db.Query(ctx, `
		SELECT id_option, count(*)
		FROM issues.issue_custom_value
		WHERE id_custom_field = $1 AND id_option IS NOT NULL
		GROUP BY id_option
	`, idCustomField)
	if err != nil {
		return nil, fmt.Errorf("querying custom field option usage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var idOption int64
		var count int
		if err := rows.Scan(&idOption, &count); err != nil {
			return nil, fmt.Errorf("scanning option usage: %w", err)
		}
		usage.OptionUsage[idOption] = count
	}
	return usage, rows.Err()
}

func (r *CustomFieldRepository) ExistsKey(ctx context.Context, idProject int64, key string) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM issues.custom_field WHERE id_project = $1 AND key = $2)
	`, idProject, key).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking custom field key: %w", err)
	}
	return exists, nil
}

func (r *CustomFieldRepository) InsertCustomField(ctx context.Context, field *model.CustomField) (*model.CustomField, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO issues.custom_field (id_project, key, name, field_type, is_required, required_since, default_value, order_rank)
		SELECT $1, $2, $3, $4, $5, $6, $7, COALESCE(MAX(order_rank), 0) + 1
		FROM issues.custom_field WHERE id_project = $1
		RETURNING id_custom_field, order_rank
	`, field.IdProject, field.Key, field.Name, field.FieldType, field.IsRequired, field.RequiredSince,
		jsonbOrNull(field.DefaultValue)).
		Scan(&field.IdCustomField, &field.OrderRank)
	if err != nil {
		return nil, fmt.Errorf("inserting custom field: %w", err)
	}
	return field, nil
}

func (r *CustomFieldRepository) UpdateCustomField(ctx context.Context, field *model.CustomField) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.custom_field
		SET name = $3, is_required = $4, required_since = $5, archived_at = $6, default_value = $7
		WHERE id_custom_field = $1 AND id_project = $2
	`, field.IdCustomField, field.IdProject, field.Name, field.IsRequired, field.RequiredSince, field.ArchivedAt,
		jsonbOrNull(field.DefaultValue))
	if err != nil {
		return fmt.Errorf("updating custom field: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) MoveCustomField(ctx context.Context, field *model.CustomField) error {
	db := extctx.GetDb(ctx, r.pool)
	var oldOrderRank int
	err := db.QueryRow(ctx, `
		UPDATE issues.custom_field new SET
			order_rank = $3
		FROM issues.custom_field old
		WHERE
			old.id_custom_field = new.id_custom_field AND
			old.id_custom_field = $1 AND
			old.id_project = $2
		RETURNING old.order_rank
	`, field.IdCustomField, field.IdProject, field.OrderRank).Scan(&oldOrderRank)
	if err != nil {
		return fmt.Errorf("moving custom field: %w", err)
	}
	return r.reorder(ctx, field.IdProject, field.IdCustomField, oldOrderRank)
}

func (r *CustomFieldRepository) DeleteCustomField(ctx context.Context, idProject, idCustomField int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM issues.custom_field WHERE id_custom_field = $1 AND id_project = $2
	`, idCustomField, idProject)
	if err != nil {
		return fmt.Errorf("deleting custom field: %w", err)
	}
	return r.reorder(ctx, idProject, idCustomField, 0)
}

func (r *CustomFieldRepository) reorder(ctx context.Context, idProject, idCustomField int64, oldOrderRank int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.custom_field des SET
			order_rank = src.order_rank
		FROM (
			SELECT
				src_cf.id_custom_field,
				ROW_NUMBER() OVER (
					ORDER BY
						order_rank,
						CASE
							WHEN src_cf.id_custom_field = $1 AND src_cf.order_rank > $3 THEN 1
							WHEN src_cf.id_custom_field = $1 AND src_cf.order_rank < $3 THEN -1
							ELSE 0
						END
				) AS order_rank
			FROM issues.custom_field src_cf
			WHERE src_cf.id_project = $2
		) src
		WHERE des.id_custom_field = src.id_custom_field AND des.id_project = $2
	`, idCustomField, idProject, oldOrderRank)
	if err != nil {
		return fmt.Errorf("reordering custom fields: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) InsertOption(ctx context.Context, idCustomField int64, label string, orderRank int) (*model.CustomFieldOption, error) {
	db := extctx.GetDb(ctx, r.pool)
	option := &model.CustomFieldOption{IdCustomField: idCustomField, Label: label, OrderRank: orderRank}
	err := db.QueryRow(ctx, `
		INSERT INTO issues.custom_field_option (id_custom_field, label, order_rank)
		VALUES ($1, $2, $3)
		RETURNING id_option
	`, idCustomField, label, orderRank).Scan(&option.IdOption)
	if err != nil {
		return nil, fmt.Errorf("inserting custom field option: %w", err)
	}
	return option, nil
}

func (r *CustomFieldRepository) UpdateOption(ctx context.Context, idCustomField, idOption int64, label string, orderRank int) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.custom_field_option SET label = $3, order_rank = $4
		WHERE id_option = $1 AND id_custom_field = $2
	`, idOption, idCustomField, label, orderRank)
	if err != nil {
		return fmt.Errorf("updating custom field option: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) DeleteOption(ctx context.Context, idCustomField, idOption int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM issues.custom_field_option WHERE id_option = $1 AND id_custom_field = $2
	`, idOption, idCustomField)
	if err != nil {
		return fmt.Errorf("deleting custom field option: %w", err)
	}
	return nil
}

func (r *CustomFieldRepository) ReassignOptionValues(ctx context.Context, idCustomField, idOptionOld int64, idOptionNew *int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE issues.issue_custom_value SET id_option = $3
		WHERE id_custom_field = $1 AND id_option = $2
	`, idCustomField, idOptionOld, idOptionNew)
	if err != nil {
		return fmt.Errorf("reassigning custom field option values: %w", err)
	}
	return nil
}

// An empty RawMessage must reach the column as SQL NULL, not as an empty jsonb
// document, which would fail to parse on the way back.
func jsonbOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
