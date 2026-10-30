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

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
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
	userService   *service.UserService
	userRepo      *repository.UserRepository
	projectRepo   *repository.ProjectRepository
	aclRepo       *repository.AclRepository
	apiKeySvc     *service.ApiKeyService
	acl           *service.AclService
	agentRunRepo  *repository.AgentRunRepository
	agentTaskRepo *repository.AgentTaskRepository
	issueRepo     *repository.IssueRepository
	dispatcher    *agent.Dispatcher
	notifier      *notify.Notifier
	thinking      *agent.ThinkingService
	pool          *pgxpool.Pool
}

func NewAdminController(
	userService *service.UserService,
	userRepo *repository.UserRepository,
	projectRepo *repository.ProjectRepository,
	aclRepo *repository.AclRepository,
	apiKeySvc *service.ApiKeyService,
	acl *service.AclService,
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	issueRepo *repository.IssueRepository,
	dispatcher *agent.Dispatcher,
	notifier *notify.Notifier,
	thinking *agent.ThinkingService,
	pool *pgxpool.Pool,
) *AdminController {
	return &AdminController{
		userService:   userService,
		userRepo:      userRepo,
		projectRepo:   projectRepo,
		aclRepo:       aclRepo,
		apiKeySvc:     apiKeySvc,
		acl:           acl,
		agentRunRepo:  agentRunRepo,
		agentTaskRepo: agentTaskRepo,
		issueRepo:     issueRepo,
		dispatcher:    dispatcher,
		notifier:      notifier,
		thinking:      thinking,
		pool:          pool,
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
			return candidate, nil
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

	// Captured before the mutation for the api-key cache purge when the active
	// flag changes (the key rows themselves are not touched by a disable). Purged
	// in BOTH directions: disabling must kill cached sessions at once, and
	// re-enabling must clear the negative cache entry a disabled key's lookup
	// writes, or the key would stay 401 for up to the negative-cache TTL.
	var keyHashes []string
	purgeKeyCache := false
	// Runs of a disabled agent, captured inside the tx so the non-terminal set is
	// consistent with the flag flip; dispatched/broadcast after commit (below).
	var cancelledRuns []*model.AgentRun
	// Task ids cancelled with those runs, compacted after commit like
	// AgentRunController.Cancel does.
	var cancelledTaskIds []int64
	// The acting admin, recorded as the actor of the run-cancel events.
	acting, _ := extctx.GetUser(ctx)
	var actorID *int64
	if acting.IdUser != 0 {
		actingID := acting.IdUser
		actorID = &actingID
	}
	// Guard + mutate in a tx to avoid a TOCTOU race on the admin count (mirrors guardLastOwner).
	// Only fields present in the request are applied — this endpoint serves the
	// admin toggle (isAdmin only), the active toggle (isActive only) and the edit
	// form (name/email[/isAdmin]).
	adminChanged := false
	activeChanged := false
	disableApplied := false
	err = extctx.RunInTx(ctx, ac.pool, func(ctx context.Context) error {
		if pErr := ac.applyProfileUpdate(ctx, idUser, isAgent, &req); pErr != nil {
			return pErr
		}
		changed, gErr := ac.applyAdminUpdate(ctx, idUser, targetActiveAdmin, &req)
		if gErr != nil {
			return gErr
		}
		adminChanged = changed
		acChanged, aErr := ac.applyActiveUpdate(ctx, idUser, targetActiveAdmin, &req)
		if aErr != nil {
			return aErr
		}
		activeChanged = acChanged
		if activeChanged {
			// Key hashes must be read before the flag flips so the cache purge
			// can run once the tx commits.
			hashes, hErr := ac.apiKeySvc.KeyHashesByUser(ctx, idUser)
			if hErr != nil {
				return hErr
			}
			keyHashes = hashes
			purgeKeyCache = true
		}
		if activeChanged && !*req.IsActive {
			disableApplied = true
			// A disabled agent must not leave runs hanging: the gateway callbacks
			// authenticate with its api key, which stops working on disable, so
			// the active stage could never report completion. Cancel every
			// non-terminal run now (same cancel path as AgentRunController.Cancel),
			// then dispatch + broadcast after the tx commits.
			runs, rErr := ac.agentRunRepo.LoadActiveForUpdate(ctx, idUser)
			if rErr != nil {
				return rErr
			}
			for _, locked := range runs {
				// The rows are locked FOR UPDATE by LoadActiveForUpdate, so a
				// concurrent phase transition can't slip between the load and
				// the cancel. The acting admin is the actor so run_event shows
				// who cancelled the run.
				updated, tErr := ac.agentRunRepo.TransitionPhase(ctx, locked.IdRun, locked.Phase, constants.PhaseCancelled, constants.ActorTypeUser, actorID, "agent account disabled")
				if tErr != nil {
					return tErr
				}
				idsTask, cErr := ac.agentTaskRepo.CancelNonTerminalForRun(ctx, locked.IdRun)
				if cErr != nil {
					return cErr
				}
				cancelledTaskIds = append(cancelledTaskIds, idsTask...)
				cancelledRuns = append(cancelledRuns, updated)
			}
		}
		return nil
	})
	switch {
	case err == errs.ErrLastAdmin:
		_ = c.Error(errs.ErrLastAdmin)
		c.Status(http.StatusUnprocessableEntity)
	case err == errs.ErrSelfDisable:
		_ = c.Error(errs.ErrSelfDisable)
		c.Status(http.StatusUnprocessableEntity)
	case isLastOwnerErr(err):
		// ErrLastOwner.WithMessage returns a copy, so match by code via errs.As.
		_ = c.Error(err)
		c.Status(http.StatusUnprocessableEntity)
	case errors.Is(err, repository.ErrUserNotFound):
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case isConflict(err): // email taken (incl. losing the unique-index race)
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusConflict)
	case err != nil:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	default:
		// An isAdmin change must kill existing sessions and the cached project-role
		// admin bypass; an isActive change must do the same plus purge the cached
		// api-key sessions (a disabled agent's key must stop working at once, and a
		// re-enabled one must not trip the negative cache entry). A name/email/colour
		// edit needs neither.
		if adminChanged || activeChanged {
			if iErr := ac.userService.InvalidateUserSessions(ctx, idUser); iErr != nil {
				_ = c.Error(iErr)
				c.Status(http.StatusInternalServerError)
				return
			}
			ac.acl.InvalidateAllUserProjectCaches(ctx, idUser)
			if purgeKeyCache {
				ac.apiKeySvc.PurgeCache(ctx, keyHashes)
			}
		}
		if disableApplied {
			// Fold the cancelled tasks' live thinking events, like
			// AgentRunController.Cancel does. Must run after the tx committed so
			// the rows it reads are visible; best-effort, never fails the request.
			if len(cancelledTaskIds) > 0 {
				ac.thinking.Compact(ctx, cancelledTaskIds...)
			}
			// Tell the gateways of the cancelled runs and refresh open clients.
			// Dispatch is async like in AgentRunController.Cancel — a gateway
			// that is down must not fail the disable.
			for _, run := range cancelledRuns {
				runForEvent := run
				go func() {
					_ = ac.dispatcher.DispatchCancelled(context.Background(), runForEvent)
				}()
				agent.BroadcastRunUpdate(ctx, ac.notifier, ac.projectRepo, ac.issueRepo, ac.agentRunRepo, ac.agentTaskRepo, run)
			}
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
	if targetActiveAdmin && targetIsAdmin && !*req.IsAdmin {
		if err := ac.guardLastAdmin(ctx); err != nil {
			return false, err
		}
	}
	return changed, ac.userRepo.SetAdmin(ctx, idUser, *req.IsAdmin)
}

// applyActiveUpdate applies the isActive toggle when present and reports whether
// the flag actually changed. Disabling the last active admin is rejected (the
// guard only applies to an active admin target); disabling the last active owner
// of any project is rejected with ErrLastOwner naming the project; self-disable
// is rejected by the caller before the tx starts.
func (ac *AdminController) applyActiveUpdate(ctx context.Context, idUser int64, targetActiveAdmin bool, req *model.AdminUpdateUserReq) (bool, error) {
	if req.IsActive == nil {
		return false, nil
	}
	user, err := ac.userRepo.LoadUser(ctx, idUser)
	if err != nil {
		return false, err
	}
	changed := user.IsActive != *req.IsActive
	if !changed || *req.IsActive {
		return changed, ac.userRepo.SetActive(ctx, idUser, *req.IsActive)
	}
	if targetActiveAdmin {
		if err := ac.guardLastAdmin(ctx); err != nil {
			return false, err
		}
	}
	if err := ac.guardLastProjectOwner(ctx, idUser); err != nil {
		return false, err
	}
	return changed, ac.userRepo.SetActive(ctx, idUser, *req.IsActive)
}

// guardLastProjectOwner rejects a disable that would leave a project without an
// active owner: every project where idUser holds owner access (directly or via an
// owner-ranked team) must keep at least one other active owner. Runs inside the
// caller's transaction; CountProjectOwnersLocked takes the advisory lock, and the
// count already excludes disabled users, so the target itself is the one expected
// to disappear — count <= 1 means the disable would orphan the project.
func (ac *AdminController) guardLastProjectOwner(ctx context.Context, idUser int64) error {
	idsProject, err := ac.projectRepo.LoadProjectIdsWhereUserOwner(ctx, idUser)
	if err != nil {
		return err
	}
	for _, idProject := range idsProject {
		count, err := ac.aclRepo.CountProjectOwnersLocked(ctx, idProject)
		if err != nil {
			return err
		}
		if count <= 1 {
			name, err := ac.projectRepo.LoadProjectName(ctx, idProject)
			if err != nil {
				return err
			}
			return errs.ErrLastOwner.WithMessage(fmt.Sprintf("cannot disable the last active owner of project %q", name))
		}
	}
	return nil
}

// applyProfileUpdate writes name/email when present. Agents have a synthetic,
// non-editable email so only their name is touched; a human email change also
func (ac *AdminController) applyProfileUpdate(ctx context.Context, idUser int64, isAgent bool, req *model.AdminUpdateUserReq) error {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if !isAgent && req.Email != nil {
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
		if isAdmin && active {
			if gErr := ac.guardLastAdmin(ctx); gErr != nil {
				return gErr
			}
		}
		// The owner guard only applies to an active target: a disabled user no
		// longer counts as an owner (GetProjectOwnerCount joins on is_active),
		// so deleting an already-disabled owner can never orphan the project.
		if active {
			if gErr := ac.guardLastProjectOwner(ctx, idUser); gErr != nil {
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
	case err == errs.ErrLastAdmin:
		_ = c.Error(errs.ErrLastAdmin)
		c.Status(http.StatusUnprocessableEntity)
	case isLastOwnerErr(err):
		_ = c.Error(err)
		c.Status(http.StatusUnprocessableEntity)
	case errors.Is(err, repository.ErrUserNotFound):
		_ = c.Error(errs.ErrNotFound)
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

// isLastOwnerErr matches ErrLastOwner by code: the disable guard returns the
// sentinel with WithMessage (a copy), so pointer/sentinel equality fails.
func isLastOwnerErr(err error) bool {
	var appErr *errs.Error
	return errs.As(err, &appErr) && appErr.Code == errs.ErrLastOwner.Code
}

func (ac *AdminController) guardLastAdmin(ctx context.Context) error {
	count, err := ac.userRepo.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errs.ErrLastAdmin
	}
	return nil
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
