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
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/deactivation"
	"github.com/bitmaster-sk/rurdesk/api/internal/email"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/password"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

type AdminController struct {
	userService  *service.UserService
	userRepo     *repository.UserRepository
	projectRepo  *repository.ProjectRepository
	apiKeySvc    *service.ApiKeyService
	acl          *service.AclService
	notifier     *notify.Notifier
	deactivation *deactivation.Service
	changeGuard  func() authpolicy.UserChangeGuard
	pool         *pgxpool.Pool
}

func NewAdminController(
	userService *service.UserService,
	userRepo *repository.UserRepository,
	projectRepo *repository.ProjectRepository,
	apiKeySvc *service.ApiKeyService,
	acl *service.AclService,
	notifier *notify.Notifier,
	deactivationService *deactivation.Service,
	changeGuard func() authpolicy.UserChangeGuard,
	pool *pgxpool.Pool,
) *AdminController {
	return &AdminController{
		userService:  userService,
		userRepo:     userRepo,
		projectRepo:  projectRepo,
		apiKeySvc:    apiKeySvc,
		acl:          acl,
		notifier:     notifier,
		deactivation: deactivationService,
		changeGuard:  changeGuard,
		pool:         pool,
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
	if req.IsAgent && req.IsAdmin {
		_ = c.Error(errs.ErrAgentAdmin)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	if req.IdProject != nil && !model.IsValidRole(req.Role) {
		_ = c.Error(errs.ErrInvalidRole)
		c.Status(http.StatusBadRequest)
		return
	}

	if req.IsAgent {
		ac.createAgent(c, &req)
		return
	}

	ctx := c.Request.Context()

	req.Email = email.Normalize(req.Email)
	if req.Email == "" || req.Password == "" {
		_ = c.Error(errs.ErrMissingCredentials)
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
	hash, err := password.Hash(req.Password)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	user := &model.User{
		Email:          req.Email,
		Name:           req.Name,
		Password:       hash,
		ColorAvatarBg:  avatarColorOrRandom(req.ColorAvatarBg),
		IsAdmin:        req.IsAdmin,
		AvatarInitials: normalizeAvatarInitials(req.AvatarInitials),
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
// agent-owner guard and cache invalidation.
func (ac *AdminController) assignToProject(ctx context.Context, idUser int64, req *model.AdminCreateUserReq) error {
	if req.IdProject == nil {
		return nil
	}
	if req.Role == model.RoleOwner && req.IsAgent {
		return errs.ErrAgentOwner
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
	case err == errs.ErrAgentOwner:
		_ = c.Error(errs.ErrAgentOwner)
		c.Status(http.StatusUnprocessableEntity)
	case isConflict(err):
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusConflict)
	default:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	}
}

var agentSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

func agentEmailDomain() string {
	domain := strings.TrimSpace(viper.GetString("AGENT_EMAIL_DOMAIN"))
	if domain == "" {
		return "agents.local"
	}
	return domain
}

// synthAgentEmail returns a unique agent-<slug>@<domain>, suffixing -2, -3, … on collision.
func (ac *AdminController) synthAgentEmail(ctx context.Context, name string) (string, error) {
	slug := strings.Trim(agentSlugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = "agent"
	}
	domain := agentEmailDomain()
	candidate := fmt.Sprintf("agent-%s@%s", slug, domain)
	for i := 2; ; i++ {
		exists, err := ac.userRepo.EmailExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return email.Normalize(candidate), nil
		}
		candidate = fmt.Sprintf("agent-%s-%d@%s", slug, i, domain)
	}
}

// generateRandomSecret returns a hex-encoded agent password seed that is discarded
// immediately, so the agent can never password-login.
func generateRandomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (ac *AdminController) createAgent(c *gin.Context, req *model.AdminCreateUserReq) {
	ctx := c.Request.Context()

	email, err := ac.synthAgentEmail(ctx, req.Name)
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
	hash, err := password.Hash(secret)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	user := &model.User{
		Email:          email,
		Name:           req.Name,
		Password:       hash,
		ColorAvatarBg:  avatarColorOrRandom(req.ColorAvatarBg),
		IsAgent:        true,
		AvatarInitials: normalizeAvatarInitials(req.AvatarInitials),
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
		keyRes, txErr = ac.apiKeySvc.CreateAgentApiKey(ctx, created.IdUser, &model.CreateAgentApiKeyReq{Name: "default"})
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
	if req.AvatarInitials.IsDefined && req.AvatarInitials.Value != nil &&
		utf8.RuneCountInString(*req.AvatarInitials.Value) > 2 {
		_ = c.Error(errs.ErrValidation.WithMessage("avatarInitials must be at most 2 characters"))
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()

	isAgent, err := ac.userRepo.IsAgentUser(ctx, idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if req.IsAdmin != nil && *req.IsAdmin && isAgent {
		_ = c.Error(errs.ErrAgentAdmin)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	// The last-admin guard runs only when the target is itself an active admin:
	// CountAdmins counts active admins only, so guarding a disabled (or non-admin)
	// target would wrongly block demoting/deleting/disabling it while a single
	// active admin remains.
	var targetActiveAdmin bool
	if req.IsAdmin != nil || req.IsActive != nil {
		isAdmin, err := ac.userRepo.IsAdminUser(ctx, idUser)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				_ = c.Error(errs.ErrNotFound)
				c.Status(http.StatusNotFound)
				return
			}
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		active, err := ac.userRepo.IsActiveUser(ctx, idUser)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		targetActiveAdmin = isAdmin && active
	}

	// Guards that need the acting admin are resolved before the tx so a self
	// disable / last-admin demotion fails fast without touching the row.
	if req.IsActive != nil && !*req.IsActive {
		acting, ok := extctx.GetUser(ctx)
		if !ok || acting.IdUser == idUser {
			_ = c.Error(errs.ErrSelfDisable)
			c.Status(http.StatusUnprocessableEntity)
			return
		}
	}

	acting, _ := extctx.GetUser(ctx)
	// Guard + mutate in a tx to avoid a TOCTOU race on the admin count (mirrors guardLastOwner).
	// Only fields present in the request are applied — this endpoint serves the
	// admin toggle (isAdmin only), the active toggle (isActive only) and the edit
	// form (name/email[/isAdmin]).
	adminChanged := false
	var activeResult *deactivation.Result
	err = extctx.RunInTx(ctx, ac.pool, func(ctx context.Context) error {
		if pErr := ac.applyProfileUpdate(ctx, idUser, isAgent, &req); pErr != nil {
			return pErr
		}
		changed, gErr := ac.applyAdminUpdate(ctx, idUser, targetActiveAdmin, &req)
		if gErr != nil {
			return gErr
		}
		adminChanged = changed
		if req.IsActive != nil {
			res, aErr := ac.deactivation.ApplyInTx(ctx, idUser, *req.IsActive, deactivation.Options{IdActor: acting.IdUser})
			if aErr != nil {
				return aErr
			}
			activeResult = res
		}
		return ac.applyBreakGlassUpdate(ctx, idUser, isAgent, &req)
	})
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case isConflict(err) || errors.Is(err, repository.ErrEmailTaken): // email taken (incl. losing the unique-index race)
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusConflict)
	case err != nil:
		ResponseErr(c, err)
	default:
		// An isAdmin change must kill existing sessions and the cached project-role
		// admin bypass; an isActive change does the same plus purges the cached
		// api-key sessions (see deactivation.Service.AfterCommit). A
		// name/email/colour edit needs neither.
		if adminChanged {
			if iErr := ac.userService.InvalidateUserSessions(ctx, idUser); iErr != nil {
				ResponseErr(c, iErr)
				return
			}
			ac.acl.InvalidateAllUserProjectCaches(ctx, idUser)
		}
		if aErr := ac.deactivation.AfterCommit(ctx, activeResult); aErr != nil {
			ResponseErr(c, aErr)
			return
		}
		c.Status(http.StatusOK)
	}
}

// applyAdminUpdate applies the isAdmin toggle when present and reports whether
// the flag actually changed (only a real change may invalidate sessions — the
// edit form always sends isAdmin, even unchanged). The last-admin guard runs
// only when the target is an active admin being demoted.
func (ac *AdminController) applyAdminUpdate(ctx context.Context, idUser int64, targetActiveAdmin bool, req *model.AdminUpdateUserReq) (bool, error) {
	if req.IsAdmin == nil {
		return false, nil
	}
	targetIsAdmin, err := ac.userRepo.IsAdminUser(ctx, idUser)
	if err != nil {
		return false, err
	}
	changed := targetIsAdmin != *req.IsAdmin
	if targetIsAdmin && !*req.IsAdmin {
		if err := ac.changeGuard().Check(ctx, authpolicy.UserChange{IdUser: idUser, Kind: authpolicy.ChangeRemoveAdmin}); err != nil {
			return false, err
		}
		if targetActiveAdmin {
			if err := ac.deactivation.GuardLastAdmin(ctx); err != nil {
				return false, err
			}
		}
		if err := ac.userRepo.SetBreakGlass(ctx, idUser, false); err != nil {
			return false, err
		}
	}
	return changed, ac.userRepo.SetAdmin(ctx, idUser, *req.IsAdmin)
}

// applyBreakGlassUpdate runs after the admin and active changes of the same request,
// so the eligibility check sees the final state of the account.
func (ac *AdminController) applyBreakGlassUpdate(ctx context.Context, idUser int64, isAgent bool, req *model.AdminUpdateUserReq) error {
	if req.IsBreakGlass == nil {
		return nil
	}
	if !*req.IsBreakGlass {
		if err := ac.changeGuard().Check(ctx, authpolicy.UserChange{IdUser: idUser, Kind: authpolicy.ChangeRemoveBreakGlass}); err != nil {
			return err
		}
		return ac.userRepo.SetBreakGlass(ctx, idUser, false)
	}
	user, err := ac.userRepo.LoadUser(ctx, idUser)
	if err != nil {
		return err
	}
	hasIdentity, err := ac.userRepo.HasIdentity(ctx, idUser)
	if err != nil {
		return err
	}
	if isAgent || !user.IsAdmin || !user.IsActive || user.IsPasswordless || hasIdentity {
		return errs.ErrBreakGlassNotAllowed
	}
	return ac.userRepo.SetBreakGlass(ctx, idUser, true)
}

// applyProfileUpdate writes name/email when present. Agents have a synthetic,
// non-editable email so only their name is touched; a human email change also
func (ac *AdminController) applyProfileUpdate(ctx context.Context, idUser int64, isAgent bool, req *model.AdminUpdateUserReq) error {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if !isAgent && req.Email != nil {
			normalizedEmail := email.Normalize(*req.Email)
			isTaken, err := ac.userRepo.EmailTakenByOther(ctx, normalizedEmail, idUser)
			if err != nil {
				return err
			}
			if isTaken {
				return repository.ErrEmailTaken
			}
			if err := ac.userRepo.UpdateProfile(ctx, idUser, name, normalizedEmail); err != nil {
				return err
			}
		} else if err := ac.userRepo.UpdateUser(ctx, idUser, name); err != nil {
			return err
		}
	}
	// Applied independently of name/email so a colour-only edit never nulls them.
	if req.ColorAvatarBg != nil {
		if err := ac.userRepo.UpdateAvatarColor(ctx, idUser, *req.ColorAvatarBg); err != nil {
			return err
		}
	}
	if req.AvatarInitials.IsDefined {
		if err := ac.userRepo.UpdateAvatarInitials(ctx, idUser, normalizeAvatarInitials(req.AvatarInitials.Value)); err != nil {
			return err
		}
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
			return errs.ErrAgentActivity
		}
		hasContent, gErr := ac.userRepo.HasAuthoredContent(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		if hasContent {
			return errs.ErrAuthoredContent
		}
		isAdmin, gErr := ac.userRepo.IsAdminUser(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		active, gErr := ac.userRepo.IsActiveUser(ctx, idUser)
		if gErr != nil {
			return gErr
		}
		if gErr := ac.changeGuard().Check(ctx, authpolicy.UserChange{IdUser: idUser, Kind: authpolicy.ChangeDelete}); gErr != nil {
			return gErr
		}
		if isAdmin && active {
			if gErr := ac.deactivation.GuardLastAdmin(ctx); gErr != nil {
				return gErr
			}
		}
		// The owner guard only applies to an active target: a disabled user no
		// longer counts as an owner (GetProjectOwnerCount joins on is_active),
		// so deleting an already-disabled owner can never orphan the project.
		if active {
			if gErr := ac.deactivation.GuardLastProjectOwner(ctx, idUser); gErr != nil {
				return gErr
			}
		}
		return ac.userRepo.DeleteUser(ctx, idUser)
	})
	switch {
	case err == errs.ErrAgentActivity:
		_ = c.Error(errs.ErrAgentActivity)
		c.Status(http.StatusConflict)
	case err == errs.ErrAuthoredContent:
		_ = c.Error(errs.ErrAuthoredContent)
		c.Status(http.StatusConflict)
	case errors.Is(err, repository.ErrUserNotFound):
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case err != nil:
		ResponseErr(c, err)
	default:
		// Kill remaining sessions and cached api-key auth snapshots.
		if iErr := ac.userService.InvalidateUserSessions(ctx, idUser); iErr != nil {
			_ = c.Error(iErr)
			c.Status(http.StatusInternalServerError)
			return
		}
		ac.apiKeySvc.PurgeCache(ctx, keyHashes)
		ac.notifier.Disconnect(idUser)
		c.Status(http.StatusOK)
	}
}

func (ac *AdminController) SetPassword(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	var req model.AdminSetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	ctx := c.Request.Context()
	user, err := ac.userRepo.LoadUser(ctx, idUser)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if user.IsAgent {
		ResponseErr(c, errs.ErrAgentPassword)
		return
	}
	keepToken := ""
	if acting, _ := extctx.GetUser(ctx); acting.IdUser == idUser {
		keepToken = extctx.BearerToken(c.GetHeader("Authorization"))
		if keepToken == "" {
			keepToken, _ = c.Cookie("Authorization")
		}
	}
	if err := ac.userService.SetPassword(ctx, idUser, req.Password, keepToken); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// requireAgent aborts with 422 unless the target user is an agent. API keys minted
// here are agent-only; humans mint their own through the user routes.
func (ac *AdminController) requireAgent(c *gin.Context, idUser int64) bool {
	isAgent, err := ac.userRepo.IsAgentUser(c.Request.Context(), idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return false
	}
	if !isAgent {
		_ = c.Error(errs.ErrNotAnAgent)
		c.Status(http.StatusUnprocessableEntity)
		return false
	}
	return true
}

// GetAgentApiKey returns the agent's single API key, or null when none exists
// (mirrors GetAgentGateway).
func (ac *AdminController) GetAgentApiKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireAgent(c, idUser) {
		return
	}
	key, err := ac.apiKeySvc.GetAgentApiKey(c.Request.Context(), idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, key)
}

// CreateAgentApiKey mints the agent's API key. A DB unique index caps an agent at
// one key; creating when one exists returns 409 — rotate via RegenerateAgentApiKey.
func (ac *AdminController) CreateAgentApiKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireAgent(c, idUser) {
		return
	}
	var req model.CreateAgentApiKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	res, err := ac.apiKeySvc.CreateAgentApiKey(c.Request.Context(), idUser, &req)
	if isConflict(err) {
		_ = c.Error(errs.ErrApiKeyExists)
		c.Status(http.StatusConflict)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, res)
}

// RegenerateAgentApiKey rotates the agent's key in place, returning the new one-time raw
// key; the old key stops authenticating immediately (mirrors RegenerateGatewayToken).
func (ac *AdminController) RegenerateAgentApiKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireAgent(c, idUser) {
		return
	}
	res, err := ac.apiKeySvc.RegenerateAgentApiKey(c.Request.Context(), idUser)
	if errors.Is(err, repository.ErrApiKeyNotFound) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, res)
}

// RevokeAgentApiKey deletes the agent's single API key.
func (ac *AdminController) RevokeAgentApiKey(c *gin.Context) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if !ac.requireAgent(c, idUser) {
		return
	}
	if err := ac.apiKeySvc.RevokeAgentApiKey(c.Request.Context(), idUser); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}
