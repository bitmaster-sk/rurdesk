package repository

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

func (r *ProjectRepository) InsertProject(ctx context.Context, p *model.Project) (*model.Project, error) {
	db := extctx.GetDb(ctx, r.pool)
	err := db.QueryRow(ctx,
		`INSERT INTO projects.project(name, color) VALUES ($1, $2) RETURNING id_project`,
		p.Name, p.Color,
	).Scan(&p.IdProject)
	if err != nil {
		return nil, fmt.Errorf("inserting project: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) UpdateProject(ctx context.Context, p *model.Project) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `
		UPDATE projects.project
		SET name = $1, color = $2, id_state_default = $3, id_severity_default = $4, id_issue_type_default = $5
		WHERE id_project = $6
	`, p.Name, p.Color, p.IdStateDefault, p.IdSeverityDefault, p.IdIssueTypeDefault, p.IdProject)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) DeleteProject(ctx context.Context, idProject int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM projects.project WHERE id_project = $1`, idProject)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) LoadProjects(ctx context.Context, idUser int64) ([]*model.Project, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT prj.id_project, prj.name, prj.color, prj.id_state_default, prj.id_severity_default, prj.id_issue_type_default
		FROM 
			projects.project prj
		WHERE prj.id_project IN (
			SELECT pru.id_project 
			FROM 
				projects.project_user pru
			WHERE pru.id_user = $1
		) OR prj.id_project IN (
			SELECT prt.id_project 
			FROM 
				projects.project_team prt
				INNER JOIN users.user_team ust ON ust.id_team = prt.id_team
			WHERE ust.id_user = $1
		)
		ORDER BY prj.name
	`, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying projects: %w", err)
	}
	projects, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Project])
	if err != nil {
		return nil, fmt.Errorf("collecting projects: %w", err)
	}
	return projects, nil
}

// LoadAllProjects returns every project regardless of membership, for global
// instance admins who are implicit owners of all projects.
func (r *ProjectRepository) LoadAllProjects(ctx context.Context) ([]*model.Project, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT prj.id_project, prj.name, prj.color, prj.id_state_default, prj.id_severity_default, prj.id_issue_type_default
		FROM projects.project prj
		ORDER BY prj.name
	`)
	if err != nil {
		return nil, fmt.Errorf("querying all projects: %w", err)
	}
	projects, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Project])
	if err != nil {
		return nil, fmt.Errorf("collecting all projects: %w", err)
	}
	return projects, nil
}

func (r *ProjectRepository) LoadProjectsIds(ctx context.Context, idUser int64) ([]int64, error) {
	projects, err := r.LoadProjects(ctx, idUser)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(projects))
	for i, p := range projects {
		ids[i] = p.IdProject
	}
	return ids, nil
}

func (r *ProjectRepository) LoadProjectsMembers(ctx context.Context, idsProject []int64) ([]*model.User, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT DISTINCT u.id_user, u.name, u.email, u.color_avatar_bg, u.is_bot
		FROM (
			SELECT usr.id_user, usr.name, usr.email, usr.color_avatar_bg, usr.is_bot
			FROM
				projects.project prj
				INNER JOIN projects.project_user pru ON prj.id_project = pru.id_project
				INNER JOIN users.user usr ON usr.id_user = pru.id_user
			WHERE
				prj.id_project = ANY($1)
			UNION
			SELECT usr.id_user, usr.name, usr.email, usr.color_avatar_bg, usr.is_bot
			FROM
				projects.project prj
				INNER JOIN projects.project_team prt ON prj.id_project = prt.id_project
				INNER JOIN users.user_team ust ON ust.id_team = prt.id_team
				INNER JOIN users.user usr ON ust.id_user = usr.id_user
			WHERE
				prj.id_project = ANY($1)
		) u
	`, idsProject)
	if err != nil {
		return nil, fmt.Errorf("querying project members: %w", err)
	}
	// Lax variant because password is not selected
	members, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[model.User])
	if err != nil {
		return nil, fmt.Errorf("collecting project members: %w", err)
	}
	return members, nil
}

func (r *ProjectRepository) LoadProject(ctx context.Context, idProject int64) (*model.Project, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT prj.id_project, prj.name, prj.color, prj.id_state_default, prj.id_severity_default, prj.id_issue_type_default
		FROM projects.project prj
		WHERE prj.id_project = $1
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying project: %w", err)
	}
	project, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Project])
	if err != nil {
		return nil, fmt.Errorf("collecting project: %w", err)
	}
	return project, nil
}

func (r *ProjectRepository) LoadProjectByIssue(ctx context.Context, idIssue int64) (*model.Project, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT prj.id_project, prj.name, prj.color, prj.id_state_default, prj.id_severity_default, prj.id_issue_type_default
		FROM projects.project prj
		INNER JOIN issues.issue iss ON iss.id_project = prj.id_project
		WHERE iss.id_issue = $1
	`, idIssue)
	if err != nil {
		return nil, fmt.Errorf("querying project by issue: %w", err)
	}
	project, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Project])
	if err != nil {
		return nil, fmt.Errorf("collecting project by issue: %w", err)
	}
	return project, nil
}

func (r *ProjectRepository) InsertProjectUser(ctx context.Context, idProject, idUser int64, role model.Role) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`INSERT INTO projects.project_user(id_project, id_user, role) VALUES ($1, $2, $3)`,
		idProject, idUser, role)
	if err != nil {
		return fmt.Errorf("inserting project user: %w", err)
	}
	return nil
}

func (r *ProjectRepository) InsertProjectTeam(ctx context.Context, idProject, idTeam int64, role model.Role) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`INSERT INTO projects.project_team(id_project, id_team, role) VALUES ($1, $2, $3)`,
		idProject, idTeam, role)
	if err != nil {
		return fmt.Errorf("inserting project team: %w", err)
	}
	return nil
}

func (r *ProjectRepository) UpdateProjectUserRole(ctx context.Context, idProject, idUser int64, role model.Role) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE projects.project_user SET role = $3 WHERE id_project = $1 AND id_user = $2`,
		idProject, idUser, role)
	if err != nil {
		return fmt.Errorf("updating project user role: %w", err)
	}
	return nil
}

func (r *ProjectRepository) UpdateProjectTeamRole(ctx context.Context, idProject, idTeam int64, role model.Role) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE projects.project_team SET role = $3 WHERE id_project = $1 AND id_team = $2`,
		idProject, idTeam, role)
	if err != nil {
		return fmt.Errorf("updating project team role: %w", err)
	}
	return nil
}

func (r *ProjectRepository) DeleteProjectUser(ctx context.Context, idProject, idUser int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`DELETE FROM projects.project_user WHERE id_project = $1 AND id_user = $2`,
		idProject, idUser)
	if err != nil {
		return fmt.Errorf("deleting project user: %w", err)
	}
	return nil
}

func (r *ProjectRepository) DeleteProjectTeam(ctx context.Context, idProject, idTeam int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx,
		`DELETE FROM projects.project_team WHERE id_project = $1 AND id_team = $2`,
		idProject, idTeam)
	if err != nil {
		return fmt.Errorf("deleting project team: %w", err)
	}
	return nil
}
