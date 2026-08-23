package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/go-redis/redis/v8"
)

const aclCacheTTL = 24 * time.Hour // matches session lifetime

type AclService struct {
	aclRepo     *repository.AclRepository
	projectRepo *repository.ProjectRepository
	teamRepo    *repository.TeamRepository
	userRepo    *repository.UserRepository
	cache       *redis.Client
}

func NewAclService(
	aclRepo *repository.AclRepository,
	projectRepo *repository.ProjectRepository,
	teamRepo *repository.TeamRepository,
	userRepo *repository.UserRepository,
	cache *redis.Client,
) *AclService {
	return &AclService{
		aclRepo:     aclRepo,
		projectRepo: projectRepo,
		teamRepo:    teamRepo,
		userRepo:    userRepo,
		cache:       cache,
	}
}

// --- Project role resolution ---

func (acl *AclService) getProjectRole(ctx context.Context, idUser, idProject int64) (model.Role, bool) {
	cacheKey := fmt.Sprintf("acl:project:%d:%d", idProject, idUser)

	cached, err := acl.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		if cached == "" {
			return "", false
		}
		return model.Role(cached), true
	}

	role, hasAccess, resolveErr := acl.resolveProjectRole(ctx, idUser, idProject)
	if resolveErr != nil {
		// Do NOT cache on DB error — a transient failure must not deny access for 24h.
		return "", false
	}

	// Populate cache (empty string = no access)
	value := string(role)
	_ = acl.cache.Set(ctx, cacheKey, value, aclCacheTTL).Err()

	return role, hasAccess
}

func (acl *AclService) resolveProjectRole(ctx context.Context, idUser, idProject int64) (model.Role, bool, error) {
	// 0. Global instance admin is implicit owner of every project.
	isAdmin, err := acl.userRepo.IsAdminUser(ctx, idUser)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("resolveProjectRole: IsAdminUser")
		return "", false, err
	}
	if isAdmin {
		return model.RoleOwner, true, nil
	}

	// 1. Direct assignment always wins
	directRole, isDirect, err := acl.aclRepo.GetProjectUserRole(ctx, idUser, idProject)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("resolveProjectRole: GetProjectUserRole")
		return "", false, err
	}
	if isDirect {
		return directRole, true, nil
	}

	// 2. Highest team role
	teamRoles, err := acl.aclRepo.GetProjectTeamRoles(ctx, idUser, idProject)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("resolveProjectRole: GetProjectTeamRoles")
		return "", false, err
	}
	if len(teamRoles) == 0 {
		return "", false, nil
	}

	best := model.RoleViewer
	for _, entry := range teamRoles {
		if model.RoleRank(entry.Role) > model.RoleRank(best) {
			best = entry.Role
		}
	}
	return best, true, nil
}

// atLeast returns true if the user's role for the project meets the minimum required role.
func (acl *AclService) atLeast(ctx context.Context, idUser, idProject int64, minimum model.Role) bool {
	role, hasAccess := acl.getProjectRole(ctx, idUser, idProject)
	return hasAccess && model.RoleRank(role) >= model.RoleRank(minimum)
}

// --- Cache management ---

// InvalidateProjectUserCache removes the cached role for a specific user on a project.
func (acl *AclService) InvalidateProjectUserCache(ctx context.Context, idUser, idProject int64) {
	key := fmt.Sprintf("acl:project:%d:%d", idProject, idUser)
	_ = acl.cache.Del(ctx, key).Err()
}

// InvalidateProjectTeamCache removes cached roles for all members of a team on a project.
func (acl *AclService) InvalidateProjectTeamCache(ctx context.Context, idTeam, idProject int64) {
	memberIds, err := acl.aclRepo.GetTeamMemberIds(ctx, idTeam)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("InvalidateProjectTeamCache: GetTeamMemberIds")
		return
	}
	for _, idUser := range memberIds {
		acl.InvalidateProjectUserCache(ctx, idUser, idProject)
	}
}

// InvalidateAllUserProjectCaches removes every cached project role for a user.
// Called when the is_admin flag changes — the bypass affects all projects.
func (acl *AclService) InvalidateAllUserProjectCaches(ctx context.Context, idUser int64) {
	pattern := fmt.Sprintf("acl:project:*:%d", idUser)
	iter := acl.cache.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		_ = acl.cache.Del(ctx, iter.Val()).Err()
	}
}

// InvalidateTeamMemberCache removes the team membership cache key for a user.
func (acl *AclService) InvalidateTeamMemberCache(ctx context.Context, idUser, idTeam int64) {
	key := fmt.Sprintf("acl:team:%d:%d", idTeam, idUser)
	_ = acl.cache.Del(ctx, key).Err()
}

// InvalidateUserTeamProjectCaches removes project-role cache entries for a user
// across all projects where the team is assigned. Called when a user leaves a team.
func (acl *AclService) InvalidateUserTeamProjectCaches(ctx context.Context, idUser, idTeam int64) {
	projectIds, err := acl.aclRepo.GetTeamProjectIds(ctx, idTeam)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("InvalidateUserTeamProjectCaches: GetTeamProjectIds")
		return
	}
	for _, idProject := range projectIds {
		acl.InvalidateProjectUserCache(ctx, idUser, idProject)
	}
}

// --- Project visibility ---

// LoadVisibleProjects returns the projects a user may see: their own, or all of them for
// an instance admin, who resolveProjectRole already treats as an owner everywhere. Lives
// here so both answers come from one place — the membership query alone returned nothing
// for a non-member admin the ACL was letting in.
func (acl *AclService) LoadVisibleProjects(ctx context.Context, idUser int64) ([]*model.Project, error) {
	isAdmin, err := acl.userRepo.IsAdminUser(ctx, idUser)
	if err != nil {
		return nil, fmt.Errorf("resolving admin flag: %w", err)
	}
	if isAdmin {
		return acl.projectRepo.LoadAllProjects(ctx)
	}
	return acl.projectRepo.LoadProjects(ctx, idUser)
}

func (acl *AclService) LoadVisibleProjectIds(ctx context.Context, idUser int64) ([]int64, error) {
	projects, err := acl.LoadVisibleProjects(ctx, idUser)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(projects))
	for i, project := range projects {
		ids[i] = project.IdProject
	}
	return ids, nil
}

// --- Project-scoped Can* methods (viewer+) ---

func (acl *AclService) CanReadProject(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleViewer)
}

// --- Project-scoped Can* methods (member+) ---

func (acl *AclService) CanCreateIssue(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleMember)
}

func (acl *AclService) CanUpdateIssue(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleMember)
}

func (acl *AclService) CanDeleteIssue(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleMember)
}

// --- Project-scoped Can* methods (owner only) ---

func (acl *AclService) CanUpdateProject(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanDeleteProject(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanCreateSeverity(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanUpdateSeverity(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanDeleteSeverity(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanCreateIssueType(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanUpdateIssueType(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanDeleteIssueType(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanCreateState(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanUpdateState(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanDeleteState(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

// CanManageSprint gates sprint create/close — a project-config write, same as states.
func (acl *AclService) CanManageSprint(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanReadMembers(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanCreateMembers(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanUpdateMembers(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanDeleteMembers(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanReadGitIntegration(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleMember)
}

func (acl *AclService) CanManageGitIntegration(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

func (acl *AclService) CanManageAgentPhaseStateMap(ctx context.Context, idUser, idProject int64) bool {
	return acl.atLeast(ctx, idUser, idProject, model.RoleOwner)
}

// --- Team-scoped methods ---

func (acl *AclService) CanReadTeam(ctx context.Context, idUser, idTeam int64) bool {
	cacheKey := fmt.Sprintf("acl:team:%d:%d", idTeam, idUser)

	cached, err := acl.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		return cached == "1"
	}

	users, err := acl.teamRepo.LoadTeamsMembers(ctx, []int64{idTeam})
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("CanReadTeam")
		return false
	}
	isMember := isInUserArray(users, idUser)
	// Cache positive and negative results — avoids repeated DB hits for
	// non-members (e.g. message_controller permission checks).
	value := "0"
	if isMember {
		value = "1"
	}
	_ = acl.cache.Set(ctx, cacheKey, value, aclCacheTTL).Err()
	return isMember
}

func isInUserArray(users []*model.User, idUser int64) bool {
	for _, u := range users {
		if u.IdUser == idUser {
			return true
		}
	}
	return false
}

// GetProjectRole returns the resolved role for a user on a project.
// Returns ("", false) if the user has no access.
func (acl *AclService) GetProjectRole(ctx context.Context, idUser, idProject int64) (model.Role, bool) {
	return acl.getProjectRole(ctx, idUser, idProject)
}
