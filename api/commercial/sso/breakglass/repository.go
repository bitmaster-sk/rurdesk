package breakglass

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lockKey int64 = 5 << 48

const activeCondition = `u.is_break_glass AND u.is_active AND u.is_admin AND NOT u.is_agent AND u.password IS NOT NULL
	AND NOT EXISTS (SELECT 1 FROM users.user_identity i WHERE i.id_user = u.id_user)`

type Account struct {
	IdUser      int64      `json:"idUser" db:"id_user"`
	Name        string     `json:"name" db:"name"`
	Email       string     `json:"email" db:"email"`
	LastLoginAt *time.Time `json:"lastLoginAt" db:"last_login_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Lock(ctx context.Context) error {
	if _, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
		return fmt.Errorf("locking break-glass accounts: %w", err)
	}
	return nil
}

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `SELECT count(*) FROM users.user u WHERE `+activeCondition).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting break-glass accounts: %w", err)
	}
	return count, nil
}

func (r *Repository) IsActive(ctx context.Context, idUser int64) (bool, error) {
	var isActive bool
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users.user u WHERE u.id_user = $1 AND `+activeCondition+`)
	`, idUser).Scan(&isActive)
	if err != nil {
		return false, fmt.Errorf("checking break-glass account: %w", err)
	}
	return isActive, nil
}

func (r *Repository) ListActive(ctx context.Context) ([]Account, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `
		SELECT u.id_user, u.name, u.email, u.last_login_at
		FROM users.user u
		WHERE `+activeCondition+`
		ORDER BY u.name, u.id_user
	`)
	if err != nil {
		return nil, fmt.Errorf("querying break-glass accounts: %w", err)
	}
	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByName[Account])
	if err != nil {
		return nil, fmt.Errorf("collecting break-glass accounts: %w", err)
	}
	return accounts, nil
}

func (r *Repository) LoadIdsToSignOut(ctx context.Context) ([]int64, error) {
	rows, err := extctx.GetDb(ctx, r.pool).Query(ctx, `SELECT id_user FROM users.user WHERE NOT is_agent AND NOT is_break_glass`)
	if err != nil {
		return nil, fmt.Errorf("querying users to sign out: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("collecting users to sign out: %w", err)
	}
	return ids, nil
}
