package login

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIdentityNotFound = errors.New("sso identity not found")

type IdentityRepository struct {
	pool *pgxpool.Pool
}

func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool}
}

func (r *IdentityRepository) LoadUserId(ctx context.Context, issuer, subject string) (int64, error) {
	var idUser int64
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT id_user FROM users.user_identity WHERE issuer = $1 AND subject = $2
	`, issuer, subject).Scan(&idUser)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrIdentityNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("loading sso identity: %w", err)
	}
	return idUser, nil
}

func (r *IdentityRepository) HasIssuer(ctx context.Context, idUser int64, issuer string) (bool, error) {
	var hasIssuer bool
	err := extctx.GetDb(ctx, r.pool).QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users.user_identity WHERE id_user = $1 AND issuer = $2)
	`, idUser, issuer).Scan(&hasIssuer)
	if err != nil {
		return false, fmt.Errorf("checking sso identity issuer: %w", err)
	}
	return hasIssuer, nil
}

func (r *IdentityRepository) Insert(ctx context.Context, idUser int64, issuer, subject string) error {
	_, err := extctx.GetDb(ctx, r.pool).Exec(ctx, `
		INSERT INTO users.user_identity (id_user, issuer, subject) VALUES ($1, $2, $3)
	`, idUser, issuer, subject)
	if err != nil {
		return fmt.Errorf("inserting sso identity: %w", err)
	}
	return nil
}
