package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/avatar"
	"github.com/bitmaster-sk/rurdesk/api/internal/email"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/password"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService *service.UserService
}

func NewUserController(us *service.UserService) *UserController {
	return &UserController{userService: us}
}

func (uc *UserController) Login(c *gin.Context) {
	var dto model.LoginReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	token, err := uc.userService.Login(c.Request.Context(), dto.Email, dto.Password, dto.HasExtendedSessionLifetime)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusUnauthorized)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (uc *UserController) Logout(c *gin.Context) {
	token := extctx.BearerToken(c.GetHeader("Authorization"))
	if err := uc.userService.Logout(c.Request.Context(), token); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (uc *UserController) Register(c *gin.Context) {
	var dto model.RegisterReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	// Public registration is a one-time bootstrap: the first ever user becomes the instance
	// admin, after which the endpoint is closed and all user creation is admin-gated.
	hash, err := password.Hash(dto.Password)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	user := &model.User{
		Email:         email.Normalize(dto.Email),
		Name:          dto.Name,
		Password:      hash,
		ColorAvatarBg: avatar.RandomColor(),
		IsAdmin:       true, // first ever user bootstraps as instance admin
	}
	_, created, err := uc.userService.RegisterFirst(c.Request.Context(), user)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !created {
		_ = c.Error(errs.ErrRegistrationClosed)
		c.Status(http.StatusForbidden)
		return
	}
	c.Status(http.StatusOK)
}

func (uc *UserController) GetByToken(c *gin.Context) {
	user, _ := extctx.GetUser(c.Request.Context())
	c.JSON(http.StatusOK, user)
}

func (uc *UserController) UpdateUser(c *gin.Context) {
	var dto model.UpdateUserReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	token := extctx.BearerToken(c.GetHeader("Authorization"))
	if token == "" {
		token, _ = c.Cookie("Authorization")
	}

	color := user.ColorAvatarBg
	if dto.ColorAvatarBg != nil {
		color = *dto.ColorAvatarBg
	}

	if err := uc.userService.Update(ctx, token, user, dto.Name, color); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	user.Name = dto.Name
	user.ColorAvatarBg = color
	c.JSON(http.StatusOK, user)
}

// ChangePassword lets a user replace their own password after verifying the
// current one. Agents are rejected by the service (API-key auth only).
func (uc *UserController) ChangePassword(c *gin.Context) {
	var req model.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	token := extctx.BearerToken(c.GetHeader("Authorization"))
	if token == "" {
		token, _ = c.Cookie("Authorization")
	}

	err := uc.userService.ChangePassword(ctx, user.IdUser, req.CurrentPassword, req.NewPassword, token)
	if errors.Is(err, service.ErrInvalidPassword) {
		_ = c.Error(err)
		c.Status(http.StatusForbidden)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

// ListUsers returns all users (no password hashes — model.User marshals
// Password as json:"-"). Available to any authenticated user.
func (uc *UserController) ListUsers(c *gin.Context) {
	users, err := uc.userService.ListUsers(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	public := make([]model.PublicUser, 0, len(users))
	for _, user := range users {
		public = append(public, user.Public())
	}
	c.JSON(http.StatusOK, public)
}

// avatarColorOrRandom returns the caller-supplied colour when present, else a
// fresh random one. Used by every admin create path.
func avatarColorOrRandom(supplied *string) string {
	if supplied != nil {
		return *supplied
	}
	return avatar.RandomColor()
}

func normalizeAvatarInitials(supplied *string) *string {
	if supplied == nil {
		return nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(*supplied))
	if normalized == "" {
		return nil
	}
	return &normalized
}
