package deactivation

import (
	"context"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	// Zero means the system acted (SCIM, identity move): runs are cancelled without a user actor.
	IdActor            int64
	SkipLastOwnerGuard bool
}

type Result struct {
	Changed          bool
	idUser           int64
	isDisabled       bool
	keyHashes        []string
	cancelledRuns    []*model.AgentRun
	cancelledTaskIds []int64
}

type Service struct {
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
	changeGuard   func() authpolicy.UserChangeGuard
}

func New(
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
	changeGuard func() authpolicy.UserChangeGuard,
) *Service {
	return &Service{
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
		changeGuard:   changeGuard,
	}
}

func (s *Service) SetActive(ctx context.Context, idUser int64, isActive bool, opts Options) (bool, error) {
	var res *Result
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		var txErr error
		res, txErr = s.ApplyInTx(ctx, idUser, isActive, opts)
		return txErr
	})
	if err != nil {
		return false, err
	}
	return res.Changed, s.AfterCommit(ctx, res)
}

func (s *Service) ApplyInTx(ctx context.Context, idUser int64, isActive bool, opts Options) (*Result, error) {
	user, err := s.userRepo.LoadUser(ctx, idUser)
	if err != nil {
		return nil, err
	}
	res := &Result{idUser: idUser, Changed: user.IsActive != isActive}
	if !res.Changed {
		return res, nil
	}
	if !isActive {
		if err := s.changeGuard().Check(ctx, authpolicy.UserChange{IdUser: idUser, Kind: authpolicy.ChangeDeactivate}); err != nil {
			return nil, err
		}
		if user.IsAdmin && user.IsActive {
			if err := s.GuardLastAdmin(ctx); err != nil {
				return nil, err
			}
		}
		if !opts.SkipLastOwnerGuard {
			if err := s.GuardLastProjectOwner(ctx, idUser); err != nil {
				return nil, err
			}
		}
	}
	// Key hashes must be read before the flag flips so the cache purge can run once
	// the tx commits. Purged in both directions: disabling must kill cached key
	// sessions at once, and re-enabling must clear the negative cache entry a
	// disabled key's lookup writes.
	hashes, err := s.apiKeySvc.KeyHashesByUser(ctx, idUser)
	if err != nil {
		return nil, err
	}
	res.keyHashes = hashes
	if err := s.userRepo.SetActive(ctx, idUser, isActive); err != nil {
		return nil, err
	}
	if isActive {
		return res, nil
	}
	res.isDisabled = true
	if user.IsBreakGlass {
		if err := s.userRepo.SetBreakGlass(ctx, idUser, false); err != nil {
			return nil, err
		}
	}
	return res, s.cancelRuns(ctx, idUser, opts.IdActor, res)
}

// cancelRuns stops every non-terminal run of a disabled agent: its gateway callbacks
// authenticate with the api key, which stops working on disable, so the active stage
// could never report completion.
func (s *Service) cancelRuns(ctx context.Context, idUser, idActor int64, res *Result) error {
	runs, err := s.agentRunRepo.LoadActiveForUpdate(ctx, idUser)
	if err != nil {
		return err
	}
	actorType := constants.ActorTypeSystem
	var actorID *int64
	if idActor != 0 {
		actorType = constants.ActorTypeUser
		actorID = &idActor
	}
	for _, locked := range runs {
		// The rows are locked FOR UPDATE by LoadActiveForUpdate, so a concurrent
		// phase transition can't slip between the load and the cancel.
		updated, err := s.agentRunRepo.TransitionPhase(ctx, locked.IdRun, locked.Phase, constants.PhaseCancelled, actorType, actorID, "agent account disabled")
		if err != nil {
			return err
		}
		idsTask, err := s.agentTaskRepo.CancelNonTerminalForRun(ctx, locked.IdRun)
		if err != nil {
			return err
		}
		res.cancelledTaskIds = append(res.cancelledTaskIds, idsTask...)
		res.cancelledRuns = append(res.cancelledRuns, updated)
	}
	return nil
}

// AfterCommit must run only after the transaction of ApplyInTx committed, so the rows it
// reads are visible.
func (s *Service) AfterCommit(ctx context.Context, res *Result) error {
	if res == nil || !res.Changed {
		return nil
	}
	if err := s.userService.InvalidateUserSessions(ctx, res.idUser); err != nil {
		return err
	}
	s.acl.InvalidateAllUserProjectCaches(ctx, res.idUser)
	s.apiKeySvc.PurgeCache(ctx, res.keyHashes)
	if !res.isDisabled {
		return nil
	}
	s.notifier.Disconnect(res.idUser)
	// Fold the cancelled tasks' live thinking events, like AgentRunController.Cancel
	// does; best-effort, never fails the request.
	if len(res.cancelledTaskIds) > 0 {
		s.thinking.Compact(ctx, res.cancelledTaskIds...)
	}
	for _, run := range res.cancelledRuns {
		runForEvent := run
		// A gateway that is down must not fail the disable.
		go func() {
			_ = s.dispatcher.DispatchCancelled(context.Background(), runForEvent)
		}()
		agent.BroadcastRunUpdate(ctx, s.notifier, s.projectRepo, s.issueRepo, s.agentRunRepo, s.agentTaskRepo, run)
	}
	return nil
}

func (s *Service) GuardLastAdmin(ctx context.Context) error {
	count, err := s.userRepo.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errs.ErrLastAdmin
	}
	return nil
}

// GuardLastProjectOwner rejects a disable that would leave a project without an
// active owner: every project where idUser holds owner access (directly or via an
// owner-ranked team) must keep at least one other active owner. Runs inside the
// caller's transaction; CountProjectOwnersLocked takes the advisory lock, and the
// count already excludes disabled users, so the target itself is the one expected
// to disappear — count <= 1 means the disable would orphan the project.
func (s *Service) GuardLastProjectOwner(ctx context.Context, idUser int64) error {
	idsProject, err := s.projectRepo.LoadProjectIdsWhereUserOwner(ctx, idUser)
	if err != nil {
		return err
	}
	for _, idProject := range idsProject {
		count, err := s.aclRepo.CountProjectOwnersLocked(ctx, idProject)
		if err != nil {
			return err
		}
		if count <= 1 {
			name, err := s.projectRepo.LoadProjectName(ctx, idProject)
			if err != nil {
				return err
			}
			return errs.ErrLastOwner.WithMessage(fmt.Sprintf("cannot disable the last active owner of project %q", name))
		}
	}
	return nil
}
