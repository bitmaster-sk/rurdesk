package test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/require"
)

func Test_A_BootstrapRegistrationIsSerialized(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()

	require.NoError(t, resetUsers(ctx, app), "this test must run first, before any test data depends on users")

	var count int64
	require.NoError(t, app.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users.user`).Scan(&count))
	require.Zero(t, count)

	const attempts = 8
	statuses := make([]int, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"name":"race%d","email":"race%d@test.sk","password":"kreslo"}`, i, i)
			statuses[i] = Request(t, app, "POST", "/api/public/register", body, "").StatusCode
		}(i)
	}
	wg.Wait()

	bootstrapped, closed := 0, 0
	for _, status := range statuses {
		switch status {
		case http.StatusOK:
			bootstrapped++
		case http.StatusForbidden:
			closed++
		default:
			t.Fatalf("concurrent registration answered %d; only 200 and 403 are valid", status)
		}
	}
	require.Equal(t, 1, bootstrapped, "exactly one concurrent registration may bootstrap the instance")
	require.Equal(t, attempts-1, closed)

	require.NoError(t, app.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users.user`).Scan(&count))
	require.EqualValues(t, 1, count, "the instance must end up with exactly one user")

	require.NoError(t, resetUsers(ctx, app))
}

func resetUsers(ctx context.Context, app *issue.Application) error {
	if _, err := app.Pool.Exec(ctx, `DELETE FROM users.user`); err != nil {
		return err
	}
	_, err := app.Pool.Exec(ctx,
		`SELECT setval(pg_get_serial_sequence('users.user', 'id_user'), 1, false)`)
	return err
}
