package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomFieldValueRepository struct {
	pool *pgxpool.Pool
}

func NewCustomFieldValueRepository(pool *pgxpool.Pool) *CustomFieldValueRepository {
	return &CustomFieldValueRepository{pool: pool}
}

// Must stay one statement: a loop of Execs would cost a round trip per value.
func (r *CustomFieldValueRepository) UpsertValues(ctx context.Context, values []model.IssueCustomValue) error {
	if len(values) == 0 {
		return nil
	}

	idsIssue := make([]int64, len(values))
	idsCustomField := make([]int64, len(values))
	fieldTypes := make([]string, len(values))
	texts := make([]*string, len(values))
	numbers := make([]*float64, len(values))
	dates := make([]*time.Time, len(values))
	bools := make([]*bool, len(values))
	idsOption := make([]*int64, len(values))

	for i, value := range values {
		idsIssue[i] = value.IdIssue
		idsCustomField[i] = value.IdCustomField
		fieldTypes[i] = string(value.FieldType)
		texts[i] = value.ValueText
		numbers[i] = value.ValueNumber
		dates[i] = value.ValueDate
		bools[i] = value.ValueBool
		idsOption[i] = value.IdOption
	}

	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.issue_custom_value
			(id_issue, id_custom_field, field_type, value_text, value_number, value_date, value_bool, id_option)
		SELECT * FROM unnest(
			$1::bigint[], $2::bigint[], $3::text[], $4::text[],
			$5::numeric[], $6::timestamp[], $7::boolean[], $8::bigint[]
		)
		ON CONFLICT (id_issue, id_custom_field) DO UPDATE SET
			value_text = EXCLUDED.value_text,
			value_number = EXCLUDED.value_number,
			value_date = EXCLUDED.value_date,
			value_bool = EXCLUDED.value_bool,
			id_option = EXCLUDED.id_option
	`, idsIssue, idsCustomField, fieldTypes, texts, numbers, dates, bools, idsOption)
	if err != nil {
		return fmt.Errorf("upserting custom values: %w", err)
	}
	return nil
}

// Copies the source issue's required values onto a new issue. A split inserts children
// straight through the repository, so without this they would be born missing them.
func (r *CustomFieldValueRepository) CopyRequiredValues(ctx context.Context, idIssueFrom, idIssueTo int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.issue_custom_value
			(id_issue, id_custom_field, field_type, value_text, value_number, value_date, value_bool, id_option)
		SELECT $2, v.id_custom_field, v.field_type, v.value_text, v.value_number, v.value_date, v.value_bool, v.id_option
		FROM issues.issue_custom_value v
		INNER JOIN issues.custom_field f ON f.id_custom_field = v.id_custom_field
		WHERE v.id_issue = $1 AND f.is_required AND f.archived_at IS NULL
		ON CONFLICT (id_issue, id_custom_field) DO NOTHING
	`, idIssueFrom, idIssueTo)
	if err != nil {
		return fmt.Errorf("copying required custom values: %w", err)
	}
	return nil
}

func (r *CustomFieldValueRepository) DeleteValuesOfOption(ctx context.Context, idCustomField, idOption int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM issues.issue_custom_value WHERE id_custom_field = $1 AND id_option = $2
	`, idCustomField, idOption)
	if err != nil {
		return fmt.Errorf("deleting custom field option values: %w", err)
	}
	return nil
}

func (r *CustomFieldValueRepository) BackfillValues(ctx context.Context, idProject int64, value model.IssueCustomValue) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		INSERT INTO issues.issue_custom_value
			(id_issue, id_custom_field, field_type, value_text, value_number, value_date, value_bool, id_option)
		SELECT i.id_issue, $2, $3, $4, $5, $6, $7, $8
		FROM issues.issue i
		WHERE i.id_project = $1
		ON CONFLICT (id_issue, id_custom_field) DO NOTHING
	`, idProject, value.IdCustomField, string(value.FieldType),
		value.ValueText, value.ValueNumber, value.ValueDate, value.ValueBool, value.IdOption)
	if err != nil {
		return fmt.Errorf("inserting missing custom values: %w", err)
	}
	return nil
}

func (r *CustomFieldValueRepository) DeleteValues(ctx context.Context, idIssue int64, idsCustomField []int64) error {
	if len(idsCustomField) == 0 {
		return nil
	}
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		DELETE FROM issues.issue_custom_value WHERE id_issue = $1 AND id_custom_field = ANY($2)
	`, idIssue, idsCustomField)
	if err != nil {
		return fmt.Errorf("deleting custom values: %w", err)
	}
	return nil
}
