package controller

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

type AdminController struct {
	userService *service.UserService
	userRepo    *repository.UserRepository
	projectRepo *repository.ProjectRepository
	apiKeySvc   *service.ApiKeyService
	acl         *service.AclService
	pool        *pgxpool.Pool
}

func NewAdminController(
	userService *service.UserService,
	userRepo *repository.UserRepository,
	projectRepo *repository.ProjectRepository,
	apiKeySvc *service.ApiKeyService,
	acl *service.AclService,
	pool *pgxpool.Pool,
) *AdminController {
	return &AdminController{
		userService: userService,
		userRepo:    userRepo,
		projectRepo: projectRepo,
		apiKeySvc:   apiKeySvc,
		acl:         acl,
		pool:        pool,
	}
}

func (ac *AdminController) ListUsers(c *gin.Context) {
	users, err := ac.userService.ListUsers(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if users == nil {
		users = []*model.User{}
	}
	c.JSON(http.StatusOK, users)
}

func (ac *AdminController) CreateUser(c *gin.Context) {
	var req model.AdminCreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if req.IsBot && req.IsAdmin {
		_ = c.Error(errBotAdmin)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	if req.IdProject != nil && !model.IsValidRole(req.Role) {
		_ = c.Error(errInvalidRole)
		c.Status(http.StatusBadRequest)
		return
	}

	if req.IsBot {
		ac.createBot(c, &req)
		return
	}

	ctx := c.Request.Context()

	if req.Email == "" || req.Password == "" {
		_ = c.Error(errMissingCredentials)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	exists, err := ac.userRepo.EmailExists(ctx, req.Email)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if exists {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusConflict)
		return
	}
	bHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	user := &model.User{
		Email:         req.Email,
		Name:          req.Name,
		Password:      string(bHash),
		ColorAvatarBg: avatarColorOrRandom(req.ColorAvatarBg),
		IsAdmin:       req.IsAdmin,
	}
	var created *model.User
	err = extctx.RunInTx(ctx, ac.pool, func(ctx context.Context) error {
		var txErr error
		if created, txErr = ac.userService.Register(ctx, user); txErr != nil {
			return txErr
		}
		return ac.assignToProject(ctx, created.IdUser, &req)
	})
	if err != nil {
		ac.writeCreateErr(c, err)
		return
	}
	c.JSON(http.StatusOK, model.AdminCreateUserRes{User: *created})
}

// assignToProject reuses ProjectMemberController.AddUser's insert path, with the
// bot-owner guard and cache invalidation.
func (ac *AdminController) assignToProject(ctx context.Context, idUser int64, req *model.AdminCreateUserReq) error {
	if req.IdProject == nil {
		return nil
	}
	if req.Role == model.RoleOwner && req.IsBot {
		return errBotOwner
	}
	if err := ac.projectRepo.InsertProjectUser(ctx, *req.IdProject, idUser, req.Role); err != nil {
		return err
	}
	ac.acl.InvalidateProjectUserCache(ctx, idUser, *req.IdProject)
	return nil
}

// writeCreateErr maps create-flow failures to HTTP status codes, including the
// case of losing the EmailExists pre-check race to a unique-constraint conflict.
func (ac *AdminController) writeCreateErr(c *gin.Context, err error) {
	switch {
	case err == errBotOwner:
		_ = c.Error(errBotOwner)
		c.Status(http.StatusUnprocessableEntity)
	case isConflict(err):
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusConflict)
	default:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	}
}

var botSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

func botEmailDomain() string {
	domain := strings.TrimSpace(viper.GetString("BOT_EMAIL_DOMAIN"))
	if domain == "" {
		return "bots.local"
	}
	return domain
}

// synthBotEmail returns a unique bot-<slug>@<domain>, suffixing -2, -3, … on collision.
func (ac *AdminController) synthBotEmail(ctx context.Context, name string) (string, error) {
	slug := strings.Trim(botSlugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = "bot"
	}
	domain := botEmailDomain()
	candidate := fmt.Sprintf("bot-%s@%s", slug, domain)
	for i := 2; ; i++ {
		exists, err := ac.userRepo.EmailExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("bot-%s-%d@%s", slug, i, domain)
	}
}

// generateRandomSecret returns a hex-encoded bot password seed that is discarded
// immediately, so the bot can never password-login.
func generateRandomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (ac *AdminController) createBot(c *gin.Context, req *model.AdminCreateUserReq) {
	ctx := c.Request.Context()

	email, err := ac.synthBotEmail(ctx, req.Name)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	secret, err := generateRandomSecret()
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	bHash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	user := &model.User{
		Email:         email,
		Name:          req.Name,
		Password:      string(bHash),
		ColorAvatarBg: avatarColorOrRandom(req.ColorAvatarBg),
		IsBot:         true,
	}
	var created *model.User
	var keyRes *model.CreateApiKeyRes
	err = extctx.RunInTx(ctx, ac.pool, func(ctx context.Context) error {
		var txErr error
		if created, txErr = ac.userService.Register(ctx, user); txErr != nil {
			return txErr
		}
		if txErr = ac.assignToProject(ctx, created.IdUser, req); txErr != nil {
			return txErr
		}
		keyRes, txErr = ac.apiKeySvc.Create(ctx, created.IdUser, &model.CreateApiKeyReq{Name: "default"})
		return txErr
	})
	if err != nil {
		ac.writeCreateErr(c, err)
		return
	}
	c.JSON(http.StatusOK, model.AdminCreateUserRes{User: *created, RawKey: keyRes.RawKey})
}

func (ac *AdminController) UpdateUser(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	var req model.AdminUpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()

	isBot, err := ac.userRepo.IsBotUser(ctx, idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if req.IsAdmin != nil && *req.IsAdmin && isBot {
		_ = c.Error(errBotAdmin)
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	// Guard + mutate in a tx to avoid a TOCTOU race on the admin count (mirrors guardLastOwner).
	// Only fields present in the request are applied — this endpoint serves both the
	// admin toggle (isAdmin only) and the edit form (name/email[/isAdmin]).
	adminChanged := false
	err = extctx.RunInTx(ctx, ac.pool, func(ctx context.Context) error {
		if pErr := ac.applyProfileUpdate(ctx, idUser, isBot, &req); pErr != nil {
			return pErr
		}
		if req.IsAdmin == nil {
			return nil
		}
		targetIsAdmin, gErr := ac.userRepo.IsAdminUser(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		// The edit form always sends isAdmin, even unchanged. Only a real change may
		// invalidate sessions, or a self name/colour edit would log you out.
		adminChanged = targetIsAdmin != *req.IsAdmin
		if targetIsAdmin && !*req.IsAdmin {
			if gErr := ac.guardLastAdmin(ctx); gErr != nil {
				return gErr
			}
		}
		return ac.userRepo.SetAdmin(ctx, idUser, *req.IsAdmin)
	})
	switch {
	case err == errLastAdmin:
		_ = c.Error(errLastAdmin)
		c.Status(http.StatusUnprocessableEntity)
	case errors.Is(err, repository.ErrUserNotFound):
		_ = c.Error(errNotFound)
		c.Status(http.StatusNotFound)
	case isConflict(err): // email taken (incl. losing the unique-index race)
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusConflict)
	case err != nil:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	default:
		// An isAdmin change must kill existing sessions and the cached project-role
		// admin bypass; a name/email/colour edit needs neither.
		if adminChanged {
			if iErr := ac.userService.InvalidateUserSessions(ctx, idUser); iErr != nil {
				_ = c.Error(iErr)
				c.Status(http.StatusInternalServerError)
				return
			}
			ac.acl.InvalidateAllUserProjectCaches(ctx, idUser)
		}
		c.Status(http.StatusOK)
	}
}

// applyProfileUpdate writes name/email when present. Bots have a synthetic,
// non-editable email so only their name is touched; a human email change also
func (ac *AdminController) applyProfileUpdate(ctx context.Context, idUser int64, isBot bool, req *model.AdminUpdateUserReq) error {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if !isBot && req.Email != nil {
			email := strings.TrimSpace(*req.Email)
			if err := ac.userRepo.UpdateProfile(ctx, idUser, name, email); err != nil {
				return err
			}
		} else if err := ac.userRepo.UpdateUser(ctx, idUser, name); err != nil {
			return err
		}
	}
	// Applied independently of name/email so a colour-only edit never nulls them.
	if req.ColorAvatarBg != nil {
		return ac.userRepo.UpdateAvatarColor(ctx, idUser, *req.ColorAvatarBg)
	}
	return nil
}

func (ac *AdminController) DeleteUser(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()

	// Captured before the delete cascades the key rows away, to purge the auth cache.
	keyHashes, err := ac.apiKeySvc.KeyHashesByUser(ctx, idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	err = extctx.RunInTx(ctx, ac.pool, func(ctx context.Context) error {
		// Checked inside the tx so the check→delete window is minimal; a concurrent
		// agent insert aborts the tx instead of leaving partial state.
		hasActivity, gErr := ac.userRepo.HasAgentActivity(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		if hasActivity {
			return errAgentActivity
		}
		hasContent, gErr := ac.userRepo.HasAuthoredContent(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		if hasContent {
			return errAuthoredContent
		}
		isAdmin, gErr := ac.userRepo.IsAdminUser(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		if isAdmin {
			if gErr := ac.guardLastAdmin(ctx); gErr != nil {
				return gErr
			}
		}
		return ac.userRepo.DeleteUser(ctx, idUser)
	})
	switch {
	case err == errAgentActivity:
		_ = c.Error(errAgentActivity)
		c.Status(http.StatusConflict)
	case err == errAuthoredContent:
		_ = c.Error(errAuthoredContent)
		c.Status(http.StatusConflict)
	case err == errLastAdmin:
		_ = c.Error(errLastAdmin)
		c.Status(http.StatusUnprocessableEntity)
	case errors.Is(err, repository.ErrUserNotFound):
		_ = c.Error(errNotFound)
		c.Status(http.StatusNotFound)
	case err != nil:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	default:
		// Kill remaining sessions and cached api-key auth snapshots.
		if iErr := ac.userService.InvalidateUserSessions(ctx, idUser); iErr != nil {
			_ = c.Error(iErr)
			c.Status(http.StatusInternalServerError)
			return
		}
		ac.apiKeySvc.PurgeCache(ctx, keyHashes)
		c.Status(http.StatusOK)
	}
}

func (ac *AdminController) guardLastAdmin(ctx context.Context) error {
	count, err := ac.userRepo.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errLastAdmin
	}
	return nil
}

// requireBot aborts with 422 unless the target user is a bot. API keys are bot-only.
func (ac *AdminController) requireBot(c *gin.Context, idUser int64) bool {
	isBot, err := ac.userRepo.IsBotUser(c.Request.Context(), idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return false
	}
	if !isBot {
		_ = c.Error(errNotABot)
		c.Status(http.StatusUnprocessableEntity)
		return false
	}
	return true
}

// GetBotKey returns the bot's single API key, or null when none exists
// (mirrors GetBotGateway).
func (ac *AdminController) GetBotKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireBot(c, idUser) {
		return
	}
	key, err := ac.apiKeySvc.GetByUser(c.Request.Context(), idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, key)
}

// CreateBotKey mints the bot's API key. A DB unique index caps a bot at one key;
// creating when one exists returns 409 — rotate via RegenerateBotKey instead.
func (ac *AdminController) CreateBotKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireBot(c, idUser) {
		return
	}
	var req model.CreateApiKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	res, err := ac.apiKeySvc.Create(c.Request.Context(), idUser, &req)
	if isConflict(err) {
		_ = c.Error(errApiKeyExists)
		c.Status(http.StatusConflict)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, res)
}

// RegenerateBotKey rotates the bot's key in place, returning the new one-time raw
// key; the old key stops authenticating immediately (mirrors RegenerateGatewayToken).
func (ac *AdminController) RegenerateBotKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireBot(c, idUser) {
		return
	}
	res, err := ac.apiKeySvc.Regenerate(c.Request.Context(), idUser)
	if errors.Is(err, repository.ErrApiKeyNotFound) {
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, res)
}

// RevokeBotKey deletes the bot's single API key.
func (ac *AdminController) RevokeBotKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireBot(c, idUser) {
		return
	}
	if err := ac.apiKeySvc.RevokeByUser(c.Request.Context(), idUser); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}
