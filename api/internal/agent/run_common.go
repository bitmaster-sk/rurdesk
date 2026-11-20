package agent

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// thinkingCompactor folds a finished task's thinking into the task.
type thinkingCompactor interface {
	Compact(ctx context.Context, idsTask ...int64)
}

// RunKindDeps are the dependencies of the run kind implementations.
type RunKindDeps struct {
	Pool              *pgxpool.Pool
	RunRepo           *repository.AgentRunRepository
	TaskRepo          *repository.AgentTaskRepository
	AgentGwRepo       *repository.AgentGatewayRepository
	GitIntRepo        *repository.GitIntegrationRepository
	IssueRepo         *repository.IssueRepository
	MessageRepo       *repository.MessageRepository
	ProjectRepo       *repository.ProjectRepository
	UserRepo          *repository.UserRepository
	ConversationRepo  *repository.AgentConversationRepository
	SkillService      *service.SkillService
	StagePlan         *service.AgentStagePlanService
	WikiAgent         *service.WikiAgentService
	Proposals         *service.WikiProposalService
	Releases          *service.ReleaseService
	AgentTaskConfig   *service.AgentTaskConfigService
	Conversations     *service.AgentConversationService
	AttachmentLinkSvc *service.AttachmentLinkService
	Thinking          thinkingCompactor
	Notifier          notify.INotifier
}

// runCommon is what the run kind implementations share: their dependencies and the steps more
// than one kind takes.
type runCommon struct {
	pool              *pgxpool.Pool
	runRepo           *repository.AgentRunRepository
	taskRepo          *repository.AgentTaskRepository
	agentGwRepo       *repository.AgentGatewayRepository
	gitIntRepo        *repository.GitIntegrationRepository
	issueRepo         *repository.IssueRepository
	messageRepo       *repository.MessageRepository
	projectRepo       *repository.ProjectRepository
	userRepo          *repository.UserRepository
	conversationRepo  *repository.AgentConversationRepository
	skillService      *service.SkillService
	stagePlan         *service.AgentStagePlanService
	wikiAgent         *service.WikiAgentService
	proposals         *service.WikiProposalService
	releases          *service.ReleaseService
	agentTaskConfig   *service.AgentTaskConfigService
	conversations     *service.AgentConversationService
	attachmentLinkSvc *service.AttachmentLinkService
	thinking          thinkingCompactor
	notifier          notify.INotifier
}

func newRunCommon(deps RunKindDeps) *runCommon {
	return &runCommon{
		pool:              deps.Pool,
		runRepo:           deps.RunRepo,
		taskRepo:          deps.TaskRepo,
		agentGwRepo:       deps.AgentGwRepo,
		gitIntRepo:        deps.GitIntRepo,
		issueRepo:         deps.IssueRepo,
		messageRepo:       deps.MessageRepo,
		projectRepo:       deps.ProjectRepo,
		userRepo:          deps.UserRepo,
		conversationRepo:  deps.ConversationRepo,
		skillService:      deps.SkillService,
		stagePlan:         deps.StagePlan,
		wikiAgent:         deps.WikiAgent,
		proposals:         deps.Proposals,
		releases:          deps.Releases,
		agentTaskConfig:   deps.AgentTaskConfig,
		conversations:     deps.Conversations,
		attachmentLinkSvc: deps.AttachmentLinkSvc,
		thinking:          deps.Thinking,
		notifier:          deps.Notifier,
	}
}
