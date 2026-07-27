package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamRepository struct {
	pool *pgxpool.Pool
}

func NewTeamRepository(pool *pgxpool.Pool) *TeamRepository {
	return &TeamRepository{pool: pool}
}

func (r *TeamRepository) InsertTeam(ctx context.Context, team *model.Team) (*model.Team, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx,
		`INSERT INTO users.team(name, color) VALUES ($1, $2) RETURNING id_team`,
		team.Name, team.Color,
	).Scan(&team.IdTeam)
	if err != nil {
		return nil, fmt.Errorf("inserting team: %w", err)
	}
	return team, nil
}

func (r *TeamRepository) UpdateTeam(ctx context.Context, team *model.Team) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE users.team SET name = $1, color = $2 WHERE id_team = $3`,
		team.Name, team.Color, team.IdTeam)
	if err != nil {
		return fmt.Errorf("updating team: %w", err)
	}
	return nil
}

func (r *TeamRepository) DeleteTeam(ctx context.Context, idTeam int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM users.team WHERE id_team = $1`, idTeam)
	if err != nil {
		return fmt.Errorf("deleting team: %w", err)
	}
	return nil
}

func (r *TeamRepository) LoadTeams(ctx context.Context, idUser int64) ([]*model.Team, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT tem.id_team, tem.name, tem.color
		FROM users.team tem
		INNER JOIN users.user_team ute ON tem.id_team = ute.id_team
		WHERE ute.id_user = $1
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying teams: %w", err)
	}
	teams, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Team])
	if err != nil {
		return nil, fmt.Errorf("collecting teams: %w", err)
	}
	return teams, nil
}

// LoadAllTeams returns every team regardless of membership — any authenticated
// user may list teams (project-member dropdowns, admin screen).
func (r *TeamRepository) LoadAllTeams(ctx context.Context) ([]*model.Team, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT tem.id_team, tem.name, tem.color
		FROM users.team tem
		ORDER BY tem.name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying all teams: %w", err)
	}
	teams, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Team])
	if err != nil {
		return nil, fmt.Errorf("collecting all teams: %w", err)
	}
	return teams, nil
}

func (r *TeamRepository) LoadTeam(ctx context.Context, idTeam int64) (*model.Team, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT tem.id_team, tem.name, tem.color
		FROM users.team tem
		WHERE tem.id_team = $1
	`, idTeam)
	if err != nil {
		return nil, fmt.Errorf("querying team: %w", err)
	}
	team, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Team])
	if err != nil {
		return nil, fmt.Errorf("collecting team: %w", err)
	}
	return team, nil
}

func (r *TeamRepository) LoadTeamsMembers(ctx context.Context, idsTeam []int64) ([]*model.User, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT DISTINCT usr.id_user, usr.name, usr.email, usr.color_avatar_bg, usr.is_bot
		FROM
			users.team tem
			INNER JOIN users.user_team ute ON tem.id_team = ute.id_team
			INNER JOIN users.user usr ON ute.id_user = usr.id_user
		WHERE
			tem.id_team = ANY($1)
		ORDER BY usr.name ASC
	`, idsTeam)
	if err != nil {
		return nil, fmt.Errorf("querying team members: %w", err)
	}
	members, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[model.User])
	if err != nil {
		return nil, fmt.Errorf("collecting team members: %w", err)
	}
	return members, nil
}

func (r *TeamRepository) IsTeamMember(ctx context.Context, idTeam, idUser int64) (bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users.user_team WHERE id_team = $1 AND id_user = $2)`,
		idTeam, idUser,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking team membership: %w", err)
	}
	return exists, nil
}

func (r *TeamRepository) InsertUserToTeam(ctx context.Context, idUser, idTeam int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`INSERT INTO users.user_team(id_user, id_team) VALUES ($1, $2)`,
		idUser, idTeam)
	if err != nil {
		return fmt.Errorf("inserting user to team: %w", err)
	}
	return nil
}

func (r *TeamRepository) DeleteUserFromTeam(ctx context.Context, idUser, idTeam int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`DELETE FROM users.user_team WHERE id_user = $1 AND id_team = $2`,
		idUser, idTeam)
	if err != nil {
		return fmt.Errorf("deleting user from team: %w", err)
	}
	return nil
}
