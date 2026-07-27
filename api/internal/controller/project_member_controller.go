package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectMemberController struct {
	aclRepo     *repository.AclRepository
	projectRepo *repository.ProjectRepository
	userRepo    *repository.UserRepository
	acl         *service.AclService
	pool        *pgxpool.Pool
}

func NewProjectMemberController(
	aclRepo *repository.AclRepository,
	projectRepo *repository.ProjectRepository,
	userRepo *repository.UserRepository,
	acl *service.AclService,
	pool *pgxpool.Pool,
) *ProjectMemberController {
	return &ProjectMemberController{
		aclRepo:     aclRepo,
		projectRepo: projectRepo,
		userRepo:    userRepo,
		acl:         acl,
		pool:        pool,
	}
}

func (mc *ProjectMemberController) GetMembers(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanReadMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	members, err := mc.aclRepo.GetProjectMembers(ctx, idProject)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, members)
}

func (mc *ProjectMemberController) AddUser(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.AddProjectUserReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !model.IsValidRole(dto.Role) {
		_ = c.Error(errInvalidRole)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanCreateMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if dto.Role == model.RoleOwner {
		if err := mc.rejectIfBot(ctx, dto.IdUser); err == errBotOwner {
			_ = c.Error(errBotOwner)
			c.Status(http.StatusUnprocessableEntity)
			return
		} else if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	if err := mc.projectRepo.InsertProjectUser(ctx, idProject, dto.IdUser, dto.Role); err != nil {
		if isConflict(err) {
			_ = c.Error(err)
			c.Status(http.StatusConflict)
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	mc.acl.InvalidateProjectUserCache(ctx, dto.IdUser, idProject)
	c.Status(http.StatusOK)
}

func (mc *ProjectMemberController) UpdateUserRole(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.UpdateProjectUserRoleReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !model.IsValidRole(dto.Role) {
		_ = c.Error(errInvalidRole)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanUpdateMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if dto.Role == model.RoleOwner {
		if err := mc.rejectIfBot(ctx, idUser); err == errBotOwner {
			_ = c.Error(errBotOwner)
			c.Status(http.StatusUnprocessableEntity)
			return
		} else if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	// Wrapped in a transaction to prevent a TOCTOU race where two concurrent demote
	// requests both pass the owner count check.
	err = extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		currentRole, exists, txErr := mc.aclRepo.GetProjectUserRole(ctx, idUser, idProject)
		if txErr != nil {
			return txErr
		}
		if !exists {
			return errNotFound
		}
		if currentRole == model.RoleOwner && dto.Role != model.RoleOwner {
			if txErr := mc.guardLastOwner(ctx, idProject); txErr != nil {
				return txErr
			}
		}
		return mc.projectRepo.UpdateProjectUserRole(ctx, idProject, idUser, dto.Role)
	})
	if err == errNotFound {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if err == errLastOwner {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	mc.acl.InvalidateProjectUserCache(ctx, idUser, idProject)
	c.Status(http.StatusOK)
}

func (mc *ProjectMemberController) RemoveUser(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanDeleteMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// Wrapped in a transaction to prevent a TOCTOU race on the owner guard.
	err = extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		currentRole, _, txErr := mc.aclRepo.GetProjectUserRole(ctx, idUser, idProject)
		if txErr != nil {
			return txErr
		}
		if currentRole == model.RoleOwner {
			if txErr := mc.guardLastOwner(ctx, idProject); txErr != nil {
				return txErr
			}
		}
		return mc.projectRepo.DeleteProjectUser(ctx, idProject, idUser)
	})
	if err == errLastOwner {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	mc.acl.InvalidateProjectUserCache(ctx, idUser, idProject)
	c.Status(http.StatusOK)
}

func (mc *ProjectMemberController) AddTeam(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.AddProjectTeamReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !model.IsValidRole(dto.Role) {
		_ = c.Error(errInvalidRole)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanCreateMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if err := mc.projectRepo.InsertProjectTeam(ctx, idProject, dto.IdTeam, dto.Role); err != nil {
		if isConflict(err) {
			_ = c.Error(err)
			c.Status(http.StatusConflict)
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	mc.acl.InvalidateProjectTeamCache(ctx, dto.IdTeam, idProject)
	c.Status(http.StatusOK)
}

func (mc *ProjectMemberController) UpdateTeamRole(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idTeam, err := strconv.ParseInt(c.Param("idTeam"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.UpdateProjectTeamRoleReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !model.IsValidRole(dto.Role) {
		_ = c.Error(errInvalidRole)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanUpdateMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// Wrapped in a transaction to prevent a TOCTOU race on the owner guard.
	err = extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		currentRole, exists, txErr := mc.aclRepo.GetProjectTeamRoleById(ctx, idTeam, idProject)
		if txErr != nil {
			return txErr
		}
		if !exists {
			return errNotFound
		}
		if currentRole == model.RoleOwner && dto.Role != model.RoleOwner {
			if txErr := mc.guardLastOwner(ctx, idProject); txErr != nil {
				return txErr
			}
		}
		return mc.projectRepo.UpdateProjectTeamRole(ctx, idProject, idTeam, dto.Role)
	})
	if err == errNotFound {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}
	if err == errLastOwner {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	mc.acl.InvalidateProjectTeamCache(ctx, idTeam, idProject)
	c.Status(http.StatusOK)
}

func (mc *ProjectMemberController) RemoveTeam(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idTeam, err := strconv.ParseInt(c.Param("idTeam"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !mc.acl.CanDeleteMembers(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// Wrapped in a transaction to prevent a TOCTOU race on the owner guard.
	err = extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		currentRole, _, txErr := mc.aclRepo.GetProjectTeamRoleById(ctx, idTeam, idProject)
		if txErr != nil {
			return txErr
		}
		if currentRole == model.RoleOwner {
			if txErr := mc.guardLastOwner(ctx, idProject); txErr != nil {
				return txErr
			}
		}
		return mc.projectRepo.DeleteProjectTeam(ctx, idProject, idTeam)
	})
	if err == errLastOwner {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	mc.acl.InvalidateProjectTeamCache(ctx, idTeam, idProject)
	c.Status(http.StatusOK)
}

// rejectIfBot returns errBotOwner if the target user is a bot; call whenever
// role==owner is requested.
func (mc *ProjectMemberController) rejectIfBot(ctx context.Context, idUser int64) error {
	isBot, err := mc.userRepo.IsBotUser(ctx, idUser)
	if err != nil {
		return err
	}
	if isBot {
		return errBotOwner
	}
	return nil
}

// guardLastOwner errors if removing or downgrading would leave the project without
// an owner. Runs inside the caller's transaction; CountProjectOwnersLocked takes an
// advisory lock so concurrent demote/remove on the same project serialize and can't
// both pass the guard.
func (mc *ProjectMemberController) guardLastOwner(ctx context.Context, idProject int64) error {
	count, err := mc.aclRepo.CountProjectOwnersLocked(ctx, idProject)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errLastOwner
	}
	return nil
}
