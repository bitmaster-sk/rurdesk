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

type AclRepository struct {
	pool *pgxpool.Pool
}

func NewAclRepository(pool *pgxpool.Pool) *AclRepository {
	return &AclRepository{pool: pool}
}

// GetProjectUserRole returns the direct role for a user on a project.
// Returns ("", false, nil) when no direct assignment exists.
func (r *AclRepository) GetProjectUserRole(ctx context.Context, idUser, idProject int64) (model.Role, bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var role model.Role
	err := db.QueryRow(ctx,
		`SELECT role FROM projects.project_user WHERE id_user = $1 AND id_project = $2`,
		idUser, idProject,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("querying project user role: %w", err)
	}
	return role, true, nil
}

// GetProjectTeamRoles returns (idTeam, role) for each team the user belongs
// to that is assigned to the project.
func (r *AclRepository) GetProjectTeamRoles(ctx context.Context, idUser, idProject int64) ([]struct {
	IdTeam int64
	Role   model.Role
}, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx, `
		SELECT pt.id_team, pt.role
		FROM projects.project_team pt
		INNER JOIN users.user_team ut ON ut.id_team = pt.id_team
		WHERE pt.id_project = $1 AND ut.id_user = $2
	`, idProject, idUser)
	if err != nil {
		return nil, fmt.Errorf("querying project team roles: %w", err)
	}
	defer rows.Close()

	var results []struct {
		IdTeam int64
		Role   model.Role
	}
	for rows.Next() {
		var entry struct {
			IdTeam int64
			Role   model.Role
		}
		if err := rows.Scan(&entry.IdTeam, &entry.Role); err != nil {
			return nil, fmt.Errorf("scanning project team role: %w", err)
		}
		results = append(results, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project team roles: %w", err)
	}
	return results, nil
}

// GetProjectOwnerCount returns the count of distinct users with owner-level
// access (direct or via an owner-ranked team), so an empty team never counts
// as an owner.
func (r *AclRepository) GetProjectOwnerCount(ctx context.Context, idProject int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	var count int
	err := db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT id_user) FROM (
			SELECT id_user FROM projects.project_user
			WHERE id_project = $1 AND role = 'owner'
			UNION ALL
			SELECT ut.id_user FROM projects.project_team pt
			INNER JOIN users.user_team ut ON ut.id_team = pt.id_team
			WHERE pt.id_project = $1 AND pt.role = 'owner'
		) owners
	`, idProject).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting project owners: %w", err)
	}
	return count, nil
}

// ownerGuardLockNamespace tags the advisory-lock keyspace for the last-owner
// guard: high 16 bits of the key, low 48 bits hold the project id (bigserial
// won't reach 2^48). Reserved for this guard — any other advisory lock must
// pick a different tag to avoid collision.
const ownerGuardLockNamespace int64 = 1

// ownerGuardLockKey builds the namespaced advisory-lock key for a project.
func ownerGuardLockKey(idProject int64) int64 {
	return (ownerGuardLockNamespace << 48) | (idProject & 0xFFFFFFFFFFFF)
}

// CountProjectOwnersLocked takes a transaction-scoped advisory lock on the
// project's owner-guard keyspace, then returns the owner count. Serializes
// concurrent owner demote/remove so two transactions can't both pass the
// last-owner guard under READ COMMITTED and leave the project ownerless.
// MUST run inside a transaction — otherwise the lock releases immediately
// and serializes nothing.
func (r *AclRepository) CountProjectOwnersLocked(ctx context.Context, idProject int64) (int, error) {
	db := extctx.GetDb(ctx, r.pool)
	if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, ownerGuardLockKey(idProject)); err != nil {
		return 0, fmt.Errorf("locking owner guard for project %d: %w", idProject, err)
	}
	return r.GetProjectOwnerCount(ctx, idProject)
}

// GetProjectTeamRoleById returns the role assigned to a specific team on a project.
// Returns ("", false, nil) when the team is not assigned to the project.
func (r *AclRepository) GetProjectTeamRoleById(ctx context.Context, idTeam, idProject int64) (model.Role, bool, error) {
	db := extctx.GetDb(ctx, r.pool)
	var role model.Role
	err := db.QueryRow(ctx,
		`SELECT role FROM projects.project_team WHERE id_team = $1 AND id_project = $2`,
		idTeam, idProject,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("querying project team role: %w", err)
	}
	return role, true, nil
}

// GetTeamMemberIds returns the user IDs of all members of a team.
// Used to invalidate individual project-role cache keys when a team role changes.
func (r *AclRepository) GetTeamMemberIds(ctx context.Context, idTeam int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT id_user FROM users.user_team WHERE id_team = $1`, idTeam)
	if err != nil {
		return nil, fmt.Errorf("querying team member ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var idUser int64
		if err := rows.Scan(&idUser); err != nil {
			return nil, fmt.Errorf("scanning team member id: %w", err)
		}
		ids = append(ids, idUser)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team member ids: %w", err)
	}
	return ids, nil
}

// GetTeamProjectIds returns project IDs where the given team is assigned.
// Used to invalidate project-role cache keys when a user is removed from a team.
func (r *AclRepository) GetTeamProjectIds(ctx context.Context, idTeam int64) ([]int64, error) {
	db := extctx.GetDb(ctx, r.pool)
	rows, err := db.Query(ctx,
		`SELECT id_project FROM projects.project_team WHERE id_team = $1`, idTeam)
	if err != nil {
		return nil, fmt.Errorf("querying team project ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var idProject int64
		if err := rows.Scan(&idProject); err != nil {
			return nil, fmt.Errorf("scanning team project id: %w", err)
		}
		ids = append(ids, idProject)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team project ids: %w", err)
	}
	return ids, nil
}

// GetProjectMembers loads a project's full member list: direct users,
// team-assigned users, and assigned teams.
func (r *AclRepository) GetProjectMembers(ctx context.Context, idProject int64) (*model.ProjectMembersRes, error) {
	db := extctx.GetDb(ctx, r.pool)

	// 1. Direct users
	directRows, err := db.Query(ctx, `
		SELECT u.id_user, u.name, u.email, u.color_avatar_bg, pu.role
		FROM projects.project_user pu
		INNER JOIN users.user u ON u.id_user = pu.id_user
		WHERE pu.id_project = $1
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying direct project members: %w", err)
	}
	defer directRows.Close()

	type directEntry struct {
		user model.User
		role model.Role
	}
	directMap := make(map[int64]directEntry)
	for directRows.Next() {
		var u model.User
		var role model.Role
		if err := directRows.Scan(&u.IdUser, &u.Name, &u.Email, &u.ColorAvatarBg, &role); err != nil {
			return nil, fmt.Errorf("scanning direct project member: %w", err)
		}
		directMap[u.IdUser] = directEntry{user: u, role: role}
	}
	if err := directRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating direct project members: %w", err)
	}

	// 2. Team-user assignments
	teamUserRows, err := db.Query(ctx, `
		SELECT u.id_user, u.name, u.email, u.color_avatar_bg, pt.id_team, pt.role
		FROM projects.project_team pt
		INNER JOIN users.user_team ut ON ut.id_team = pt.id_team
		INNER JOIN users.user u ON u.id_user = ut.id_user
		WHERE pt.id_project = $1
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying team-user assignments: %w", err)
	}
	defer teamUserRows.Close()

	// accumulate: idUser -> map[idTeam]role
	teamAccum := make(map[int64]struct {
		user  model.User
		teams map[int64]model.Role
	})
	for teamUserRows.Next() {
		var u model.User
		var idTeam int64
		var role model.Role
		if err := teamUserRows.Scan(&u.IdUser, &u.Name, &u.Email, &u.ColorAvatarBg, &idTeam, &role); err != nil {
			return nil, fmt.Errorf("scanning team-user assignment: %w", err)
		}
		entry, exists := teamAccum[u.IdUser]
		if !exists {
			entry = struct {
				user  model.User
				teams map[int64]model.Role
			}{user: u, teams: make(map[int64]model.Role)}
		}
		entry.teams[idTeam] = role
		teamAccum[u.IdUser] = entry
	}
	if err := teamUserRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team-user assignments: %w", err)
	}

	// 3. Build AroUser slice
	allUserIds := make(map[int64]struct{})
	for idUser := range directMap {
		allUserIds[idUser] = struct{}{}
	}
	for idUser := range teamAccum {
		allUserIds[idUser] = struct{}{}
	}

	aroUsers := make([]model.AroUser, 0, len(allUserIds))
	for idUser := range allUserIds {
		direct, isDirect := directMap[idUser]
		teamInfo, inTeam := teamAccum[idUser]

		// A nil slice marshals to JSON null; the client models idsTeams as a
		// plain array, so a user in no team must still serialize as [].
		aro := model.AroUser{IdsTeams: []int64{}}
		if isDirect {
			aro.User = direct.user
			aro.IsDirect = true
			aro.Role = direct.role // direct always wins
		} else {
			aro.User = teamInfo.user
			aro.IsDirect = false
			aro.Role = highestRole(teamInfo.teams)
		}

		if inTeam {
			for idTeam := range teamInfo.teams {
				aro.IdsTeams = append(aro.IdsTeams, idTeam)
			}
		}

		aroUsers = append(aroUsers, aro)
	}

	// 4. Teams
	teamRows, err := db.Query(ctx, `
		SELECT t.id_team, t.name, t.color, pt.role,
			   COUNT(ut.id_user) as member_count
		FROM projects.project_team pt
		INNER JOIN users.team t ON t.id_team = pt.id_team
		LEFT JOIN users.user_team ut ON ut.id_team = pt.id_team
		WHERE pt.id_project = $1
		GROUP BY t.id_team, t.name, t.color, pt.role
	`, idProject)
	if err != nil {
		return nil, fmt.Errorf("querying project teams: %w", err)
	}
	defer teamRows.Close()

	aroTeams := make([]model.AroTeam, 0)
	for teamRows.Next() {
		var aro model.AroTeam
		if err := teamRows.Scan(&aro.IdTeam, &aro.Name, &aro.Color, &aro.Role, &aro.MemberCount); err != nil {
			return nil, fmt.Errorf("scanning project team: %w", err)
		}
		aroTeams = append(aroTeams, aro)
	}
	if err := teamRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project teams: %w", err)
	}

	return &model.ProjectMembersRes{Users: aroUsers, Teams: aroTeams}, nil
}

func highestRole(roles map[int64]model.Role) model.Role {
	best := model.RoleViewer
	for _, role := range roles {
		if model.RoleRank(role) > model.RoleRank(best) {
			best = role
		}
	}
	return best
}
