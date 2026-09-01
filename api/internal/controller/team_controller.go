package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type TeamController struct {
	teamRepo *repository.TeamRepository
	acl      *service.AclService
	notifSvc *service.NotificationService
}

func NewTeamController(
	tr *repository.TeamRepository,
	acl *service.AclService,
	notifSvc *service.NotificationService,
) *TeamController {
	return &TeamController{teamRepo: tr, acl: acl, notifSvc: notifSvc}
}

func (tc *TeamController) CreateTeam(c *gin.Context) {
	var dto model.CreateTeamReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()

	// No auto-join: teams are admin-managed, the creating admin is not a member.
	team, err := tc.teamRepo.InsertTeam(ctx, &model.Team{Name: dto.Name, Color: dto.Color})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, team)
}

func (tc *TeamController) UpdateTeam(c *gin.Context) {
	var dto model.EditTeamReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()

	team, err := tc.teamRepo.LoadTeam(ctx, dto.IdTeam)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	team.Name = dto.Name
	team.Color = dto.Color

	if err := tc.teamRepo.UpdateTeam(ctx, team); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, team)
}

func (tc *TeamController) DeleteTeam(c *gin.Context) {
	idTeam, err := strconv.ParseInt(c.Query("idTeam"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()

	if err := tc.teamRepo.DeleteTeam(ctx, idTeam); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

// GetAllTeams returns every team — any authenticated user may list teams
// (project-member dropdowns, admin screen).
func (tc *TeamController) GetAllTeams(c *gin.Context) {
	teams, err := tc.teamRepo.LoadAllTeams(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, teams)
}

// GetMyTeams returns the teams the caller is a member of (chat menu).
func (tc *TeamController) GetMyTeams(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	teams, err := tc.teamRepo.LoadTeams(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, teams)
}

func (tc *TeamController) GetTeamMembers(c *gin.Context) {
	idTeam, err := strconv.ParseInt(c.Param("idTeam"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()

	members, err := tc.teamRepo.LoadTeamsMembers(ctx, []int64{idTeam})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if members == nil {
		members = []*model.User{}
	}
	c.JSON(http.StatusOK, members)
}

// GetMyTeamMembers lists a team's members; caller must be a member (403 otherwise).
// Non-admin equivalent of GET /admin/team/:idTeam/member.
func (tc *TeamController) GetMyTeamMembers(c *gin.Context) {
	idTeam, err := strconv.ParseInt(c.Param("idTeam"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	caller, _ := extctx.GetUser(ctx)

	isMember, err := tc.teamRepo.IsTeamMember(ctx, idTeam, caller.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !isMember {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	members, err := tc.teamRepo.LoadTeamsMembers(ctx, []int64{idTeam})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if members == nil {
		members = []*model.User{}
	}
	c.JSON(http.StatusOK, members)
}

func (tc *TeamController) AddTeamMember(c *gin.Context) {
	var dto model.AddTeamMemberReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if err := tc.teamRepo.InsertUserToTeam(ctx, dto.IdUser, dto.IdTeam); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	tc.acl.InvalidateTeamMemberCache(ctx, dto.IdUser, dto.IdTeam)
	tc.acl.InvalidateUserTeamProjectCaches(ctx, dto.IdUser, dto.IdTeam)

	// Notify the added user.
	if team, lErr := tc.teamRepo.LoadTeam(ctx, dto.IdTeam); lErr == nil {
		notifReq := &model.CreateNotificationReq{
			IdUser:        dto.IdUser,
			Type:          constants.NotificationTypeTeamJoined,
			ActorName:     user.Name,
			ActorAvatarBg: user.ColorAvatarBg,
			RefType:       constants.NotificationRefTypeTeam,
			RefId:         strconv.FormatInt(team.IdTeam, 10),
			RefTitle:      team.Name,
		}
		if notifErr := tc.notifSvc.Notify(ctx, notifReq); notifErr != nil {
			log.Warn().Err(notifErr).
				Int64("idUser", notifReq.IdUser).
				Int64("idTeam", dto.IdTeam).
				Msg("AddTeamMember: failed to create notification")
		}
	}
	c.Status(http.StatusOK)
}

func (tc *TeamController) DeleteTeamMember(c *gin.Context) {
	idTeam, err := strconv.ParseInt(c.Query("idTeam"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idUser, err := strconv.ParseInt(c.Query("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()

	if err := tc.teamRepo.DeleteUserFromTeam(ctx, idUser, idTeam); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	tc.acl.InvalidateTeamMemberCache(ctx, idUser, idTeam)
	tc.acl.InvalidateUserTeamProjectCaches(ctx, idUser, idTeam)
	c.Status(http.StatusOK)
}
