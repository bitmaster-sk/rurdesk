package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) InsertUser(ctx context.Context, user *model.User) (*model.User, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx, `
		INSERT INTO users.user(email, password, name, color_avatar_bg, is_bot, is_admin)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id_user
	`, user.Email, user.Password, user.Name, user.ColorAvatarBg, user.IsBot, user.IsAdmin,
	).Scan(&user.IdUser)
	if err != nil {
		return nil, fmt.Errorf("inserting user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) LoadUser(ctx context.Context, idUser int64) (*model.User, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_user, name, email, password, color_avatar_bg, is_bot, is_admin
		FROM users.user
		WHERE id_user = $1
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.User])
	if err != nil {
		return nil, fmt.Errorf("collecting user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) LoadUserByEmail(ctx context.Context, email string) (*model.User, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_user, name, email, password, color_avatar_bg, is_bot, is_admin
		FROM users.user
		WHERE email = $1
	`, email)
	if err != nil {
		return nil, fmt.Errorf("querying user by email: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("collecting user by email: %w", err)
	}
	return user, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, idUser int64, name string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE users.user SET name = $1 WHERE id_user = $2`,
		name, idUser,
	)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// UpdateProfile updates name and email together
// (admin edit). A unique violation on email surfaces to the caller as a 409.
func (r *UserRepository) UpdateProfile(ctx context.Context, idUser int64, name, email string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE users.user SET name = $1, email = $2 WHERE id_user = $3`,
		name, email, idUser,
	)
	if err != nil {
		return fmt.Errorf("updating user profile: %w", err)
	}
	return nil
}

// UpdateAvatarColor sets the avatar background colour independently of the
// name/email fields, so a colour change never risks touching (or nulling) them.
func (r *UserRepository) UpdateAvatarColor(ctx context.Context, idUser int64, colorAvatarBg string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE users.user SET color_avatar_bg = $1 WHERE id_user = $2`,
		colorAvatarBg, idUser,
	)
	if err != nil {
		return fmt.Errorf("updating user avatar color: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, idUser int64, passwordHash string) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE users.user SET password = $1 WHERE id_user = $2`,
		passwordHash, idUser,
	)
	if err != nil {
		return fmt.Errorf("updating user password: %w", err)
	}
	return nil
}

func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users.user WHERE email = $1)`, email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking email exists: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) ListUsers(ctx context.Context) ([]*model.User, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT id_user, name, email, password, color_avatar_bg, is_bot, is_admin
		FROM users.user
		ORDER BY id_user
	`)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}
	users, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.User])
	if err != nil {
		return nil, fmt.Errorf("collecting users: %w", err)
	}
	return users, nil
}

func (r *UserRepository) CountUsers(ctx context.Context) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users.user`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

func (r *UserRepository) CountAdmins(ctx context.Context) (int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users.user WHERE is_admin = TRUE`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting admins: %w", err)
	}
	return count, nil
}

func (r *UserRepository) SetAdmin(ctx context.Context, idUser int64, isAdmin bool) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx,
		`UPDATE users.user SET is_admin = $1 WHERE id_user = $2`, isAdmin, idUser)
	if err != nil {
		return fmt.Errorf("setting user admin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) IsAdminUser(ctx context.Context, idUser int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var isAdmin bool
	err := db.QueryRow(ctx, `SELECT is_admin FROM users.user WHERE id_user = $1`, idUser).Scan(&isAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrUserNotFound
	}
	if err != nil {
		return false, fmt.Errorf("querying user admin: %w", err)
	}
	return isAdmin, nil
}

// HasAgentActivity reports whether the user is referenced by any agent table
// lacking ON DELETE CASCADE (agent.run.id_user_bot, agent.task.id_user_bot,
// agent.run_event.id_user). api_key, bot_gateway, and project memberships
// cascade and are intentionally excluded.
func (r *UserRepository) HasAgentActivity(ctx context.Context, idUser int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM agent.run        WHERE id_user_bot = $1)
		    OR EXISTS(SELECT 1 FROM agent.task       WHERE id_user_bot = $1)
		    OR EXISTS(SELECT 1 FROM agent.run_event  WHERE id_user     = $1)
	`, idUser).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking agent activity: %w", err)
	}
	return exists, nil
}

// HasAuthoredContent reports whether the user authored an issue or an issue
// relation. Those foreign keys are ON DELETE RESTRICT, so a delete would
// otherwise fail as a raw constraint violation with nothing to translate.
func (r *UserRepository) HasAuthoredContent(ctx context.Context, idUser int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM issues.issue          WHERE create_by  = $1 OR update_by = $1)
		    OR EXISTS(SELECT 1 FROM issues.issue_relation WHERE created_by = $1)
	`, idUser).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking authored content: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	tag, err := db.Exec(ctx, `DELETE FROM users.user WHERE id_user = $1`, idUser)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) IsBotUser(ctx context.Context, idUser int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var isBot bool
	err := db.QueryRow(ctx,
		`SELECT is_bot FROM users.user WHERE id_user = $1`, idUser,
	).Scan(&isBot)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("querying user is bot: %w", err)
	}
	return isBot, nil
}
