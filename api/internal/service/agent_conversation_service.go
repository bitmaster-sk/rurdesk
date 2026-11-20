package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentConversationService owns the rules of a user's chat with an agent: which
// agent may be picked, which projects it may see, and that only the owner ever
// reaches the conversation (anyone else gets not found).
type AgentConversationService struct {
	pool        *pgxpool.Pool
	repo        *repository.AgentConversationRepository
	runRepo     *repository.AgentRunRepository
	taskRepo    *repository.AgentTaskRepository
	userRepo    *repository.UserRepository
	gatewayRepo *repository.AgentGatewayRepository
	projectRepo *repository.ProjectRepository
	acl         *AclService
}

func NewAgentConversationService(
	pool *pgxpool.Pool,
	repo *repository.AgentConversationRepository,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	userRepo *repository.UserRepository,
	gatewayRepo *repository.AgentGatewayRepository,
	projectRepo *repository.ProjectRepository,
	acl *AclService,
) *AgentConversationService {
	return &AgentConversationService{
		pool:        pool,
		repo:        repo,
		runRepo:     runRepo,
		taskRepo:    taskRepo,
		userRepo:    userRepo,
		gatewayRepo: gatewayRepo,
		projectRepo: projectRepo,
		acl:         acl,
	}
}

// CreateConversation starts a conversation. With a project the agent must be its member;
// without one it must share at least one project with the owner. The model must be one the
// agent's gateway offers.
func (s *AgentConversationService) CreateConversation(ctx context.Context, owner *model.User, req model.CreateAgentConversationReq) (*model.AgentConversation, error) {
	if err := s.checkAgent(ctx, req.IdUserAgent); err != nil {
		return nil, err
	}
	chatModel := nonEmpty(req.Model)
	if err := s.checkModelChoice(ctx, req.IdUserAgent, chatModel); err != nil {
		return nil, err
	}
	if req.IdProject != nil {
		if err := s.checkProject(ctx, owner.IdUser, req.IdUserAgent, *req.IdProject); err != nil {
			return nil, err
		}
	} else if err := s.checkSharesProject(ctx, owner.IdUser, req.IdUserAgent); err != nil {
		return nil, err
	}

	idConversation, err := s.repo.InsertConversation(ctx, owner.IdUser, req.IdUserAgent, req.IdProject, chatModel)
	if err != nil {
		return nil, err
	}
	return s.repo.LoadConversation(ctx, idConversation)
}

// AddProject sets the main project when there is none yet (allowed even after
// the first message; a context project is promoted) and otherwise adds a context project.
func (s *AgentConversationService) AddProject(ctx context.Context, owner *model.User, idConversation, idProject int64) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		if conversation.IdProject != nil && *conversation.IdProject == idProject {
			return nil
		}
		if err := s.checkProject(ctx, owner.IdUser, conversation.IdUserAgent, idProject); err != nil {
			return err
		}
		if conversation.IdProject == nil {
			return s.repo.SetMainProject(ctx, idConversation, &idProject)
		}
		return s.repo.AddContextProject(ctx, idConversation, idProject)
	})
}

// RemoveProject removes a context project, or clears the main project until the
// first message locks it; the context projects stay for the owner to promote one.
func (s *AgentConversationService) RemoveProject(ctx context.Context, owner *model.User, idConversation, idProject int64) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		if conversation.IdProject != nil && *conversation.IdProject == idProject {
			if conversation.IsLocked {
				return errs.ErrAgentConversationLocked
			}
			return s.repo.SetMainProject(ctx, idConversation, nil)
		}
		return s.repo.RemoveContextProject(ctx, idConversation, idProject)
	})
}

// LoadAgents lists the agents the owner can chat with: active agents with a
// gateway that are members of the project, or of any project the owner can see, each with the
// models its gateway offers.
func (s *AgentConversationService) LoadAgents(ctx context.Context, owner *model.User, idProject *int64) ([]*model.AgentConversationAgent, error) {
	var idsProject []int64
	if idProject != nil {
		if !s.acl.CanReadProject(ctx, owner.IdUser, *idProject) {
			return nil, errs.ErrAgentConversationNoAccess
		}
		idsProject = []int64{*idProject}
	} else {
		visible, err := s.acl.LoadVisibleProjectIds(ctx, owner.IdUser)
		if err != nil {
			return nil, fmt.Errorf("loading projects of user %d: %w", owner.IdUser, err)
		}
		idsProject = visible
	}
	if len(idsProject) == 0 {
		return []*model.AgentConversationAgent{}, nil
	}

	members, err := s.projectRepo.LoadProjectsMembers(ctx, idsProject)
	if err != nil {
		return nil, err
	}
	candidates := make([]*model.User, 0, len(members))
	idsCandidate := make([]int64, 0, len(members))
	for _, member := range members {
		if member.IsAgent && member.IsActive {
			candidates = append(candidates, member)
			idsCandidate = append(idsCandidate, member.IdUser)
		}
	}
	if len(candidates) == 0 {
		return []*model.AgentConversationAgent{}, nil
	}
	gateways, err := s.gatewayRepo.LoadByAgentUsers(ctx, idsCandidate)
	if err != nil {
		return nil, err
	}

	agents := make([]*model.AgentConversationAgent, 0, len(candidates))
	for _, candidate := range candidates {
		gateway := gateways[candidate.IdUser]
		if gateway == nil {
			continue
		}
		agents = append(agents, &model.AgentConversationAgent{User: *candidate, AgentModelChoices: gateway.ModelChoices()})
	}
	slices.SortFunc(agents, func(left, right *model.AgentConversationAgent) int {
		return cmp.Or(cmp.Compare(left.Name, right.Name), cmp.Compare(left.IdUser, right.IdUser))
	})
	return agents, nil
}

// checkModelChoice refuses a model the agent's gateway does not offer; nil is the default.
func (s *AgentConversationService) checkModelChoice(ctx context.Context, idUserAgent int64, chatModel *string) error {
	gateway, err := s.gatewayRepo.LoadByAgentUser(ctx, idUserAgent)
	if err != nil {
		return fmt.Errorf("loading gateway of agent %d: %w", idUserAgent, err)
	}
	if gateway == nil {
		return errs.ErrAgentNoGateway
	}
	if chatModel != nil && !gateway.IsSupportedModel(*chatModel) {
		return errs.ErrAgentChatModelNotOffered.WithMessage("the agent does not offer the model " + *chatModel)
	}
	return nil
}

// nonEmpty treats an empty value as not set.
func nonEmpty(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func (s *AgentConversationService) LoadConversation(ctx context.Context, owner *model.User, idConversation int64) (*model.AgentConversation, error) {
	return s.loadOwned(ctx, owner, idConversation, false)
}

// LoadConversations returns the owner's conversations; a non-nil idProject keeps those whose
// main project it is.
func (s *AgentConversationService) LoadConversations(ctx context.Context, owner *model.User, idProject *int64) ([]*model.AgentConversationListItem, error) {
	return s.repo.LoadConversationsByUser(ctx, owner.IdUser, idProject)
}

// DeleteConversation removes the conversation, its run and the run's turns in one transaction. It returns the
// deleted chat run (nil before the first message) so the caller can drop its worktree.
func (s *AgentConversationService) DeleteConversation(ctx context.Context, owner *model.User, idConversation int64) (*model.AgentRun, error) {
	var run *model.AgentRun
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		// Locking the run waits out a turn the scheduler is activating.
		if run, err = s.lockRun(ctx, conversation); err != nil {
			return err
		}
		return s.repo.DeleteConversation(ctx, idConversation)
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}

// LoadMessages returns the conversation's messages, oldest first.
func (s *AgentConversationService) LoadMessages(ctx context.Context, owner *model.User, idConversation int64) ([]*model.AgentConversationMessage, error) {
	if _, err := s.loadOwned(ctx, owner, idConversation, false); err != nil {
		return nil, err
	}
	return s.repo.LoadMessages(ctx, idConversation)
}

func (s *AgentConversationService) SetRead(ctx context.Context, owner *model.User, idConversation int64) error {
	if _, err := s.loadOwned(ctx, owner, idConversation, false); err != nil {
		return err
	}
	return s.repo.SetRead(ctx, idConversation)
}

// SetPinned pins or unpins the owner's conversation.
func (s *AgentConversationService) SetPinned(ctx context.Context, owner *model.User, idConversation int64, isPinned bool) error {
	if _, err := s.loadOwned(ctx, owner, idConversation, false); err != nil {
		return err
	}
	return s.repo.SetPinned(ctx, idConversation, isPinned)
}

// UpdateSettings replaces the model the agent answers with. It is refused while a turn runs (the agent
// is answering); a queued turn picks the new model up when it starts. The same model as stored changes nothing.
func (s *AgentConversationService) UpdateSettings(ctx context.Context, owner *model.User, idConversation int64, req model.UpdateAgentConversationSettingsReq) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		chatModel := nonEmpty(req.Model)
		if equalOptional(conversation.Model, chatModel) {
			return nil
		}
		if err := s.checkModelChoice(ctx, conversation.IdUserAgent, chatModel); err != nil {
			return err
		}
		// Locking the run waits out a turn the scheduler is activating.
		run, err := s.lockRun(ctx, conversation)
		if err != nil {
			return err
		}
		if run != nil && run.Phase == constants.PhaseInProgress {
			return errs.ErrAgentConversationBusy
		}
		return s.repo.SetSettings(ctx, idConversation, chatModel)
	})
}

func equalOptional(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

// CountUnread is the number of the owner's conversations with an unread agent message.
func (s *AgentConversationService) CountUnread(ctx context.Context, owner *model.User) (int, error) {
	return s.repo.CountUnread(ctx, owner.IdUser)
}

// LoadDetail is the conversation with what its header shows: the attached projects,
// the agent, what the agent is doing, the models it offers and the token stats of every turn.
func (s *AgentConversationService) LoadDetail(ctx context.Context, owner *model.User, idConversation int64) (*model.AgentConversationDetail, error) {
	conversation, err := s.loadOwned(ctx, owner, idConversation, false)
	if err != nil {
		return nil, err
	}
	projects, err := s.attachedProjects(ctx, conversation)
	if err != nil {
		return nil, err
	}
	agentUser, err := s.userRepo.LoadUser(ctx, conversation.IdUserAgent)
	if err != nil {
		return nil, fmt.Errorf("loading agent %d: %w", conversation.IdUserAgent, err)
	}
	status, err := s.agentStatus(ctx, owner, conversation)
	if err != nil {
		return nil, err
	}
	gateway, err := s.gatewayRepo.LoadByAgentUser(ctx, conversation.IdUserAgent)
	if err != nil {
		return nil, fmt.Errorf("loading gateway of agent %d: %w", conversation.IdUserAgent, err)
	}
	stats := &model.RunStatsRes{AttemptsPerStage: map[string]int{}, TokensByModel: map[string]int{}}
	if conversation.IdRun != nil {
		if stats, err = s.taskRepo.StatsForRun(ctx, *conversation.IdRun); err != nil {
			return nil, err
		}
	}
	return &model.AgentConversationDetail{
		Conversation: conversation,
		Projects:     projects,
		Agent:        agentUser,
		AgentStatus:  status,
		AgentModels:  gateway.ModelChoices(),
		Stats:        stats,
	}, nil
}

// attachedProjects lists the main project first, then the context projects.
func (s *AgentConversationService) attachedProjects(ctx context.Context, conversation *model.AgentConversation) ([]model.AgentConversationProject, error) {
	idsProject := conversation.IdsProjectContext
	if conversation.IdProject != nil {
		idsProject = append([]int64{*conversation.IdProject}, idsProject...)
	}
	projects := make([]model.AgentConversationProject, 0, len(idsProject))
	for _, idProject := range idsProject {
		name, err := s.projectRepo.LoadProjectName(ctx, idProject)
		if err != nil {
			return nil, fmt.Errorf("loading name of project %d: %w", idProject, err)
		}
		projects = append(projects, model.AgentConversationProject{
			IdProject: idProject,
			Name:      name,
			IsMain:    conversation.IdProject != nil && *conversation.IdProject == idProject,
		})
	}
	return projects, nil
}

// agentStatus tells whether the agent is free, runs a stage, or has this conversation's turn
// queued. The stage is named only when it belongs to an issue or release run of a project the
// owner can read; anything else shows as plain busy, and this conversation's own running turn as
// busy with stage "chat".
func (s *AgentConversationService) agentStatus(ctx context.Context, owner *model.User, conversation *model.AgentConversation) (model.AgentStatus, error) {
	status := model.AgentStatus{State: constants.AgentStatusIdle}
	active, err := s.taskRepo.LoadAgentActiveTask(ctx, conversation.IdUserAgent)
	if err != nil {
		return status, err
	}
	if active != nil {
		status.State = constants.AgentStatusBusy
		isOwnTurn := conversation.IdRun != nil && active.IdRun == *conversation.IdRun
		switch {
		case isOwnTurn:
			status.Stage = &active.Stage
			status.IdTask = &active.IdTask
		case !constants.IsPrivateRunKind(active.Kind) && active.IdProject != 0 &&
			s.acl.CanReadProject(ctx, owner.IdUser, active.IdProject):
			status.IdProject = &active.IdProject
			status.IdIssuePublic = active.IdIssuePublic
			status.Stage = &active.Stage
		}
	}
	if conversation.IdRun == nil {
		return status, nil
	}
	position, err := s.repo.LoadChatQueuePosition(ctx, conversation.IdUserAgent, *conversation.IdRun)
	if err != nil {
		return status, err
	}
	if position > 0 {
		status.State = constants.AgentStatusQueued
		status.QueuePosition = &position
	}
	return status, nil
}

// CreateMessage stores the owner's message and queues an agent turn for it. The first message
// titles the conversation, locks its main project and creates the chat run; while a turn is
// queued or running the message is only stored.
func (s *AgentConversationService) CreateMessage(ctx context.Context, owner *model.User, idConversation int64, text string) (*model.AgentConversationMessage, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errs.ErrValidation.WithMessage("the message is empty")
	}
	var message *model.AgentConversationMessage
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		if err := s.checkCanSend(ctx, owner, conversation); err != nil {
			return err
		}
		run, err := s.lockRun(ctx, conversation)
		if err != nil {
			return err
		}
		message, err = s.repo.InsertMessage(ctx, idConversation,
			constants.ConversationAuthorUser, constants.ConversationMessageText, text, &owner.IdUser, nil)
		if err != nil {
			return err
		}
		if !conversation.IsLocked {
			if err := s.repo.LockConversation(ctx, idConversation, conversationTitle(text)); err != nil {
				return err
			}
		}
		return s.queueTurn(ctx, owner, conversation, run)
	})
	if err != nil {
		return nil, err
	}
	return message, nil
}

// CreateTaskDraft asks for a turn in which the agent drafts one task about the given agent answer
// instead of replying. Only an agent text answer of this conversation qualifies, anything else is
// not found. An idle conversation runs the draft next; while a reply turn is queued or running,
// the draft runs after it.
func (s *AgentConversationService) CreateTaskDraft(ctx context.Context, owner *model.User, idConversation, idMessage int64) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		if err := requireMainProject(conversation); err != nil {
			return err
		}
		if idMessage == 0 {
			return errs.ErrValidation.WithMessage("idMessage is required")
		}
		answer, err := s.repo.LoadMessage(ctx, idConversation, idMessage)
		if err != nil {
			return err
		}
		if answer == nil || answer.Author != constants.ConversationAuthorAgent || answer.Kind != constants.ConversationMessageText {
			return errs.ErrNotFound
		}
		if conversation.IdRun == nil {
			return errs.ErrValidation.WithMessage("send a message before asking for a task draft")
		}
		if err := s.checkCanSend(ctx, owner, conversation); err != nil {
			return err
		}
		run, err := s.lockRun(ctx, conversation)
		if err != nil {
			return err
		}
		request := constants.TaskDraftNextTurn
		if run.Phase == constants.PhaseQueued || run.Phase == constants.PhaseInProgress {
			request = constants.TaskDraftAfterTurn
		}
		if conversation.TaskDraftRequest != nil {
			return errs.ErrAgentConversationDraftPending
		}
		if err := s.repo.SetTaskDraftRequest(ctx, idConversation, &request, &idMessage); err != nil {
			return err
		}
		return s.queueTurn(ctx, owner, conversation, run)
	})
}

// Retry queues a new reply turn after a turn that ended in error, without a message of its own.
// It applies only while the run waits for input and the newest message is that error; anything
// else is a conflict.
func (s *AgentConversationService) Retry(ctx context.Context, owner *model.User, idConversation int64) error {
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		conversation, err := s.loadOwned(ctx, owner, idConversation, true)
		if err != nil {
			return err
		}
		if err := requireMainProject(conversation); err != nil {
			return err
		}
		if conversation.IdRun == nil {
			return errs.ErrAgentConversationNotRetryable
		}
		if err := s.checkCanSend(ctx, owner, conversation); err != nil {
			return err
		}
		run, err := s.lockRun(ctx, conversation)
		if err != nil {
			return err
		}
		if run.Phase != constants.PhaseAwaitingInput {
			return errs.ErrAgentConversationNotRetryable
		}
		latest, err := s.repo.LoadLatestMessage(ctx, idConversation)
		if err != nil {
			return err
		}
		if latest == nil || latest.Author != constants.ConversationAuthorSystem ||
			latest.Kind != constants.ConversationMessageError {
			return errs.ErrAgentConversationNotRetryable
		}
		return s.queueTurn(ctx, owner, conversation, run)
	})
}

// lockRun locks the conversation's chat run, or returns nil before the first message. The
// scheduler activates a turn under the same lock, so a message is never missed by both the
// turn's history and the next turn.
func (s *AgentConversationService) lockRun(ctx context.Context, conversation *model.AgentConversation) (*model.AgentRun, error) {
	if conversation.IdRun == nil {
		return nil, nil
	}
	run, err := s.runRepo.LoadByIdForUpdate(ctx, *conversation.IdRun)
	if err != nil {
		return nil, fmt.Errorf("loading chat run of conversation %d: %w", conversation.IdConversation, err)
	}
	return run, nil
}

// queueTurn creates the chat run on the first message (run is nil), otherwise wakes the idle
// locked run; a queued or running turn is left alone. Call it inside a transaction holding the
// conversation row.
func (s *AgentConversationService) queueTurn(ctx context.Context, owner *model.User, conversation *model.AgentConversation, run *model.AgentRun) error {
	if run == nil {
		created, err := s.runRepo.InsertForConversation(ctx, conversation.IdUserAgent, conversation.IdProject)
		if err != nil {
			return err
		}
		return s.repo.SetRun(ctx, conversation.IdConversation, created.IdRun)
	}
	if run.IdProject == 0 && conversation.IdProject != nil {
		if err := s.runRepo.SetChatProject(ctx, run.IdRun, *conversation.IdProject); err != nil {
			return err
		}
	}
	if run.Phase == constants.PhaseQueued || run.Phase == constants.PhaseInProgress {
		return nil
	}
	idOwner := owner.IdUser
	_, err := s.runRepo.TransitionPhase(ctx, run.IdRun, run.Phase, constants.PhaseQueued, constants.ActorTypeUser, &idOwner, "chat message")
	return err
}

// requireMainProject refuses a conversation that has context projects but lost its main one; a
// conversation with neither is a plain chat.
func requireMainProject(conversation *model.AgentConversation) error {
	if conversation.IdProject == nil && len(conversation.IdsProjectContext) > 0 {
		return errs.ErrAgentConversationMainRequired
	}
	return nil
}

// checkCanSend re-checks, on every send, what Create checked once: the agent can
// still answer, the owner still reads the main project and the agent is still in
// it (or, without a project, the two still share one).
func (s *AgentConversationService) checkCanSend(ctx context.Context, owner *model.User, conversation *model.AgentConversation) error {
	if err := s.checkAgent(ctx, conversation.IdUserAgent); err != nil {
		return err
	}
	if err := requireMainProject(conversation); err != nil {
		return err
	}
	if conversation.IdProject == nil {
		return s.checkSharesProject(ctx, owner.IdUser, conversation.IdUserAgent)
	}
	if !s.acl.CanReadProject(ctx, owner.IdUser, *conversation.IdProject) {
		return errs.ErrAgentConversationNoAccess
	}
	if !s.acl.CanReadProject(ctx, conversation.IdUserAgent, *conversation.IdProject) {
		return errs.ErrAgentNotInProject.WithMessage("the agent is not a member of this project")
	}
	return nil
}

// conversationTitle is the first ChatTitleMaxRunes characters of the first message,
// with its whitespace collapsed to single spaces.
func conversationTitle(text string) string {
	title := []rune(strings.Join(strings.Fields(text), " "))
	if len(title) > constants.ChatTitleMaxRunes {
		title = title[:constants.ChatTitleMaxRunes]
	}
	return string(title)
}

// LoadReadableProjects is the projects a turn may read: the main and context projects that both the
// owner and the agent can read right now. A chat without a main project reads nothing.
func (s *AgentConversationService) LoadReadableProjects(ctx context.Context, conversation *model.AgentConversation) ([]int64, error) {
	if conversation.IdProject == nil {
		return []int64{}, nil
	}
	candidates := append([]int64{*conversation.IdProject}, conversation.IdsProjectContext...)

	readable := make([]int64, 0, len(candidates))
	for _, idProject := range candidates {
		if s.acl.CanReadProject(ctx, conversation.IdUser, idProject) &&
			s.acl.CanReadProject(ctx, conversation.IdUserAgent, idProject) {
			readable = append(readable, idProject)
		}
	}
	return readable, nil
}

func (s *AgentConversationService) loadOwned(ctx context.Context, owner *model.User, idConversation int64, isForUpdate bool) (*model.AgentConversation, error) {
	load := s.repo.LoadConversation
	if isForUpdate {
		load = s.repo.LoadConversationForUpdate
	}
	conversation, err := load(ctx, idConversation)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.IdUser != owner.IdUser {
		return nil, errs.ErrNotFound
	}
	return conversation, nil
}

// checkAgent rejects a user who cannot answer: not an agent, disabled, or with no
// gateway to run on.
func (s *AgentConversationService) checkAgent(ctx context.Context, idUserAgent int64) error {
	agentUser, err := s.userRepo.LoadUser(ctx, idUserAgent)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNotAnAgent
	}
	if err != nil {
		return fmt.Errorf("loading agent %d: %w", idUserAgent, err)
	}
	if !agentUser.IsAgent || !agentUser.IsActive {
		return errs.ErrNotAnAgent
	}
	gateway, err := s.gatewayRepo.LoadByAgentUser(ctx, idUserAgent)
	if err != nil {
		return fmt.Errorf("loading gateway of agent %d: %w", idUserAgent, err)
	}
	if gateway == nil {
		return errs.ErrAgentNoGateway
	}
	return nil
}

// checkProject requires both the owner and the agent to read the project.
func (s *AgentConversationService) checkProject(ctx context.Context, idOwner, idUserAgent, idProject int64) error {
	if !s.acl.CanReadProject(ctx, idOwner, idProject) {
		return errs.ErrAgentNotInProject.WithMessage("you are not a member of this project")
	}
	if !s.acl.CanReadProject(ctx, idUserAgent, idProject) {
		return errs.ErrAgentNotInProject.WithMessage("the agent is not a member of this project")
	}
	return nil
}

func (s *AgentConversationService) checkSharesProject(ctx context.Context, idOwner, idUserAgent int64) error {
	idsOwnerProject, err := s.acl.LoadVisibleProjectIds(ctx, idOwner)
	if err != nil {
		return fmt.Errorf("loading projects of user %d: %w", idOwner, err)
	}
	idsAgentProject, err := s.acl.LoadVisibleProjectIds(ctx, idUserAgent)
	if err != nil {
		return fmt.Errorf("loading projects of agent %d: %w", idUserAgent, err)
	}
	for _, idProject := range idsAgentProject {
		if slices.Contains(idsOwnerProject, idProject) {
			return nil
		}
	}
	return errs.ErrAgentNotInProject.WithMessage("the agent shares no project with you")
}
