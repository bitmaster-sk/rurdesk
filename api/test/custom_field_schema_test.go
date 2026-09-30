package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomFieldSchema_RejectsValueOfWrongType(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-schema-type")

	pool := app.Pool
	ctx := context.Background()

	var idCustomField int64
	require.Nil(t, pool.QueryRow(ctx, `
		INSERT INTO issues.custom_field (id_project, key, name, field_type)
		VALUES ($1, 'impact', 'Impact', 'number')
		RETURNING id_custom_field
	`, idProject).Scan(&idCustomField))

	idIssue := createIssueWithType(t, app, token, idProject, "cf-schema", nil).IdIssue

	_, err := pool.Exec(ctx, `
		INSERT INTO issues.issue_custom_value (id_issue, id_custom_field, field_type, value_text)
		VALUES ($1, $2, 'number', 'not a number')
	`, idIssue, idCustomField)
	require.Error(t, err, "CHECK must reject a text value on a number field")
}

func TestCustomFieldSchema_RejectsMismatchedFieldType(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-schema-fk")

	pool := app.Pool
	ctx := context.Background()

	var idCustomField int64
	require.Nil(t, pool.QueryRow(ctx, `
		INSERT INTO issues.custom_field (id_project, key, name, field_type)
		VALUES ($1, 'note', 'Note', 'text')
		RETURNING id_custom_field
	`, idProject).Scan(&idCustomField))

	idIssue := createIssueWithType(t, app, token, idProject, "cf-schema-fk-issue", nil).IdIssue

	_, err := pool.Exec(ctx, `
		INSERT INTO issues.issue_custom_value (id_issue, id_custom_field, field_type, value_number)
		VALUES ($1, $2, 'number', 1)
	`, idIssue, idCustomField)
	require.Error(t, err, "composite FK must reject a field_type that disagrees with the definition")
}

func TestCustomFieldSchema_CascadesOnIssueDelete(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-schema-cascade")

	pool := app.Pool
	ctx := context.Background()

	var idCustomField int64
	require.Nil(t, pool.QueryRow(ctx, `
		INSERT INTO issues.custom_field (id_project, key, name, field_type)
		VALUES ($1, 'note', 'Note', 'text')
		RETURNING id_custom_field
	`, idProject).Scan(&idCustomField))

	issue := createIssueWithType(t, app, token, idProject, "cf-cascade", nil)
	_, err := pool.Exec(ctx, `
		INSERT INTO issues.issue_custom_value (id_issue, id_custom_field, field_type, value_text)
		VALUES ($1, $2, 'text', 'x')
	`, issue.IdIssue, idCustomField)
	require.Nil(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM issues.issue WHERE id_issue = $1`, issue.IdIssue)
	require.Nil(t, err)

	var count int
	require.Nil(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM issues.issue_custom_value WHERE id_custom_field = $1
	`, idCustomField).Scan(&count))
	require.Equal(t, 0, count)
}
