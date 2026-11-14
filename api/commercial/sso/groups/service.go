package groups

import (
	"context"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IService interface {
	ListMappings(ctx context.Context) ([]MappingRes, error)
	InsertMapping(ctx context.Context, req InsertMappingReq) error
	DeleteMapping(ctx context.Context, idMapping int64) error
	Sync(ctx context.Context, user *model.User, id identity.Identity, adminGroup string) (*SyncResult, error)
	AfterSync(ctx context.Context, result *SyncResult) error
}

type SyncResult struct {
	idUser         int64
	changedTeams   []int64
	isAdminChanged bool
}

type Service struct {
	repo     *mappingRepository
	users    *repository.UserRepository
	teams    *repository.TeamRepository
	acl      *service.AclService
	sessions *service.UserService
}

var _ IService = (*Service)(nil)

func NewService(pool *pgxpool.Pool, users *repository.UserRepository, teams *repository.TeamRepository, acl *service.AclService, sessions *service.UserService) *Service {
	return &Service{repo: &mappingRepository{pool: pool}, users: users, teams: teams, acl: acl, sessions: sessions}
}

func (s *Service) ListMappings(ctx context.Context) ([]MappingRes, error) {
	return s.repo.listMappings(ctx)
}

func (s *Service) InsertMapping(ctx context.Context, req InsertMappingReq) error {
	return s.repo.insertMapping(ctx, strings.TrimSpace(req.GroupValue), req.IdTeam)
}

func (s *Service) DeleteMapping(ctx context.Context, idMapping int64) error {
	return s.repo.deleteMapping(ctx, idMapping)
}

func (s *Service) Sync(ctx context.Context, user *model.User, id identity.Identity, adminGroup string) (*SyncResult, error) {
	userGroups, isKnown, err := s.groupsOf(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isKnown {
		extctx.GetLogger(ctx).Warn().Int64("idUser", user.IdUser).Bool("isOverage", id.HasGroupsOverage).Msg("sso: groups claim is missing, group sync skipped")
		return &SyncResult{idUser: user.IdUser}, nil
	}
	if !id.HasGroupsClaim {
		extctx.GetLogger(ctx).Info().Int64("idUser", user.IdUser).Msg("sso: groups claim is missing after earlier groups, syncing as no groups")
	}
	input, err := s.input(ctx, user, userGroups, adminGroup)
	if err != nil {
		return nil, err
	}
	plan := PlanSync(input)
	if plan.IsAdminRemovalSkipped {
		extctx.GetLogger(ctx).Warn().Int64("idUser", user.IdUser).Msg("sso: last admin with a password keeps the admin flag")
	}
	return s.apply(ctx, user, plan)
}

// groupsOf reads a missing claim as "no groups" only for an identity whose IdP sent groups before,
// because IdPs leave the claim out for people in no group but a misconfigured IdP never sends it.
func (s *Service) groupsOf(ctx context.Context, id identity.Identity) ([]string, bool, error) {
	if id.HasGroupsClaim {
		return id.Groups, true, s.repo.markGroupsSent(ctx, id.Issuer, id.Subject)
	}
	if id.HasGroupsOverage {
		return nil, false, nil
	}
	hasSent, err := s.repo.hasSentGroups(ctx, id.Issuer, id.Subject)
	if err != nil || !hasSent {
		return nil, false, err
	}
	return []string{}, true, nil
}

func (s *Service) input(ctx context.Context, user *model.User, userGroups []string, adminGroup string) (SyncInput, error) {
	mappings, err := s.repo.loadMappings(ctx)
	if err != nil {
		return SyncInput{}, err
	}
	memberTeams, err := s.repo.loadMappedMemberships(ctx, user.IdUser)
	if err != nil {
		return SyncInput{}, err
	}
	input := SyncInput{
		Groups:         userGroups,
		HasGroupsClaim: true,
		Mappings:       mappings,
		MemberTeams:    memberTeams,
		AdminGroup:     adminGroup,
		IsAdmin:        user.IsAdmin,
	}
	if adminGroup != "" && user.IsAdmin && user.IsActive && !user.IsPasswordless {
		count, err := s.repo.countAdminsWithPasswordLocked(ctx)
		if err != nil {
			return SyncInput{}, err
		}
		input.IsLastAdminWithPassword = count <= 1
	}
	return input, nil
}

func (s *Service) apply(ctx context.Context, user *model.User, plan SyncPlan) (*SyncResult, error) {
	result := &SyncResult{idUser: user.IdUser}
	for _, idTeam := range plan.AddTeams {
		if err := s.teams.InsertUserToTeam(ctx, user.IdUser, idTeam); err != nil {
			return nil, err
		}
		result.changedTeams = append(result.changedTeams, idTeam)
	}
	for _, idTeam := range plan.RemoveTeams {
		if err := s.teams.DeleteUserFromTeam(ctx, user.IdUser, idTeam); err != nil {
			return nil, err
		}
		result.changedTeams = append(result.changedTeams, idTeam)
	}
	if plan.SetAdmin == nil {
		return result, nil
	}
	if !*plan.SetAdmin {
		if err := s.users.SetBreakGlass(ctx, user.IdUser, false); err != nil {
			return nil, err
		}
	}
	if err := s.users.SetAdmin(ctx, user.IdUser, *plan.SetAdmin); err != nil {
		return nil, err
	}
	user.IsAdmin = *plan.SetAdmin
	result.isAdminChanged = true
	return result, nil
}

// AfterSync must run after the transaction of Sync committed, otherwise a cache
// rebuilt in between would read the old memberships.
func (s *Service) AfterSync(ctx context.Context, result *SyncResult) error {
	if result == nil {
		return nil
	}
	for _, idTeam := range result.changedTeams {
		s.acl.InvalidateTeamMemberCache(ctx, result.idUser, idTeam)
		s.acl.InvalidateUserTeamProjectCaches(ctx, result.idUser, idTeam)
	}
	if !result.isAdminChanged {
		return nil
	}
	s.acl.InvalidateAllUserProjectCaches(ctx, result.idUser)
	return s.sessions.InvalidateUserSessions(ctx, result.idUser)
}
