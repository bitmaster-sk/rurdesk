package test

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAfterCommit_RunsAfterCommit(t *testing.T) {
	app := Setup(t)

	var ran []string
	err := extctx.RunInTx(context.Background(), app.Pool, func(ctx context.Context) error {
		extctx.AfterCommit(ctx, func(context.Context) { ran = append(ran, "first") })
		extctx.AfterCommit(ctx, func(context.Context) { ran = append(ran, "second") })
		assert.Empty(t, ran, "callbacks must not run while the transaction is open")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, ran)
}

func TestAfterCommit_SkippedOnRollback(t *testing.T) {
	app := Setup(t)

	ran := false
	wanted := errors.New("transaction body failed")
	err := extctx.RunInTx(context.Background(), app.Pool, func(ctx context.Context) error {
		extctx.AfterCommit(ctx, func(context.Context) { ran = true })
		return wanted
	})

	require.ErrorIs(t, err, wanted)
	assert.False(t, ran, "a rolled-back transaction must not fire its after-commit callbacks")
}

func TestAfterCommit_SeesCommittedRow(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()

	ownerToken := Token(t, app)
	prjRes := Request(t, app, "POST", "/api/private/project",
		`{"name":"after-commit-project","color":"#778899"}`, ownerToken)
	require.Equal(t, 200, prjRes.StatusCode)

	var idProject int64
	require.NoError(t, app.Pool.QueryRow(ctx,
		`SELECT id_project FROM projects.project WHERE name = 'after-commit-project'`).Scan(&idProject))
	defer app.Pool.Exec(ctx, `DELETE FROM projects.project WHERE id_project = $1`, idProject) //nolint:errcheck

	var seen string
	err := extctx.RunInTx(ctx, app.Pool, func(txCtx context.Context) error {
		_, execErr := extctx.GetDb(txCtx, app.Pool).Exec(txCtx,
			`UPDATE projects.project SET name = 'after-commit-renamed' WHERE id_project = $1`, idProject)
		if execErr != nil {
			return execErr
		}
		extctx.AfterCommit(txCtx, func(hookCtx context.Context) {
			assert.Same(t, app.Pool, extctx.GetDb(hookCtx, app.Pool),
				"the hook context must not carry the committed transaction")
			extctx.GetDb(hookCtx, app.Pool).QueryRow(hookCtx, //nolint:errcheck
				`SELECT name FROM projects.project WHERE id_project = $1`, idProject).Scan(&seen)
		})
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, "after-commit-renamed", seen)
}
