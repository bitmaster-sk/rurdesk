package injector

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/mcp"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/router"
	"github.com/bitmaster-sk/rurdesk/api/internal/scheduler"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

var di = NewDependencyInjector()

func Clear(key string) {
	di.Clear(key)
}

// ClearAll drops every dependency, closing the ones that own OS resources first.
// Tests call it per suite; dropping the map alone leaves the pgx pool and redis
// client connected for the life of the process and exhausts max_connections.
func ClearAll() {
	if pool, err := peek[*pgxpool.Pool]("db"); err == nil {
		pool.Close()
	}
	if cache, err := peek[*redis.Client]("cache"); err == nil {
		_ = cache.Close()
	}
	di.ClearAll()
}

// peek returns an already-constructed dependency without creating one.
func peek[T any](key string) (T, error) {
	var zero T
	instance, ok := di.Peek(key)
	if !ok {
		return zero, fmt.Errorf("dependency %q not constructed", key)
	}
	typed, ok := instance.(T)
	if !ok {
		return zero, fmt.Errorf("dependency %q has unexpected type %T", key, instance)
	}
	return typed, nil
}

// Set injects a dependency by key, for testing only.
func Set(key string, instance any) {
	di.Set(key, instance)
}

func GetHttpServer() *gin.Engine {
	instance, _ := di.GetWithNew("http-server", func() (any, error) {
		server := gin.New()
		server.Use(gin.Recovery())
		return server, nil
	})
	return instance.(*gin.Engine)
}

func GetBaseLogger() zerolog.Logger {
	instance, _ := di.GetWithNew("base-logger", func() (any, error) {
		logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
		return logger, nil
	})
	return instance.(zerolog.Logger)
}

func GetCache() (*redis.Client, error) {
	instance, err := di.GetWithNew("cache", func() (any, error) {
		cache := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", viper.GetString("CACHE_HOST"), viper.GetInt("CACHE_PORT")),
			Password: viper.GetString("CACHE_PASSWORD"),
			DB:       viper.GetInt("CACHE_DB"),
		})
		return cache, nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*redis.Client), nil
}

func GetDb() (*pgxpool.Pool, error) {
	instance, err := di.GetWithNew("db", func() (any, error) {
		dsn := fmt.Sprintf("postgres://%s:%s@%s/%s",
			viper.GetString("DATABASE_USER"),
			viper.GetString("DATABASE_PASSWORD"),
			viper.GetString("DATABASE_HOST"),
			viper.GetString("DATABASE_NAME"),
		)
		return pgxpool.New(context.Background(), dsn)
	})
	if err != nil {
		return nil, err
	}
	return instance.(*pgxpool.Pool), nil
}

// mustDb and mustCache fail the boot loudly. The accessors below run once from
// main's wiring and there is no recovery from "no database"; discarding the
// error instead hands every repository a nil pool, which surfaces as a nil
// dereference on the first request, far from the cause.
func mustDb() *pgxpool.Pool {
	pool, err := GetDb()
	if err != nil {
		panic(fmt.Sprintf("injector: database pool unavailable: %v", err))
	}
	return pool
}

func mustCache() *redis.Client {
	client, err := GetCache()
	if err != nil {
		panic(fmt.Sprintf("injector: cache client unavailable: %v", err))
	}
	return client
}

func GetNotifier() *notify.Notifier {
	instance, _ := di.GetWithNew("notifier", func() (any, error) {
		return notify.NewNotifier(), nil
	})
	return instance.(*notify.Notifier)
}

func GetUserRepository() *repository.UserRepository {
	instance, _ := di.GetWithNew("user-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewUserRepository(pool), nil
	})
	return instance.(*repository.UserRepository)
}

func GetTeamRepository() *repository.TeamRepository {
	instance, _ := di.GetWithNew("team-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewTeamRepository(pool), nil
	})
	return instance.(*repository.TeamRepository)
}

func GetProjectRepository() *repository.ProjectRepository {
	instance, _ := di.GetWithNew("project-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewProjectRepository(pool), nil
	})
	return instance.(*repository.ProjectRepository)
}

func GetMessageRepository() *repository.MessageRepository {
	instance, _ := di.GetWithNew("message-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewMessageRepository(pool), nil
	})
	return instance.(*repository.MessageRepository)
}

func GetIssueRepository() *repository.IssueRepository {
	instance, _ := di.GetWithNew("issue-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewIssueRepository(pool), nil
	})
	return instance.(*repository.IssueRepository)
}

func GetSeverityRepository() *repository.SeverityRepository {
	instance, _ := di.GetWithNew("severity-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewSeverityRepository(pool), nil
	})
	return instance.(*repository.SeverityRepository)
}

func GetIssueTypeRepository() *repository.IssueTypeRepository {
	instance, _ := di.GetWithNew("issue-type-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewIssueTypeRepository(pool), nil
	})
	return instance.(*repository.IssueTypeRepository)
}

func GetStateRepository() *repository.StateRepository {
	instance, _ := di.GetWithNew("state-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewStateRepository(pool), nil
	})
	return instance.(*repository.StateRepository)
}

func GetSprintRepository() *repository.SprintRepository {
	instance, _ := di.GetWithNew("sprint-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewSprintRepository(pool), nil
	})
	return instance.(*repository.SprintRepository)
}

func GetSavedViewRepository() *repository.SavedViewRepository {
	instance, _ := di.GetWithNew("saved-view-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewSavedViewRepository(pool), nil
	})
	return instance.(*repository.SavedViewRepository)
}

func GetSprintService() *service.SprintService {
	instance, _ := di.GetWithNew("sprint-service", func() (any, error) {
		pool := mustDb()
		return service.NewSprintService(pool, GetSprintRepository(), GetStateRepository()), nil
	})
	return instance.(*service.SprintService)
}

func GetStateService() *service.StateService {
	instance, _ := di.GetWithNew("state-service", func() (any, error) {
		pool := mustDb()
		return service.NewStateService(pool, GetStateRepository()), nil
	})
	return instance.(*service.StateService)
}

func GetSeverityService() *service.SeverityService {
	instance, _ := di.GetWithNew("severity-service", func() (any, error) {
		pool := mustDb()
		return service.NewSeverityService(pool, GetSeverityRepository()), nil
	})
	return instance.(*service.SeverityService)
}

func GetIssueTypeService() *service.IssueTypeService {
	instance, _ := di.GetWithNew("issue-type-service", func() (any, error) {
		pool := mustDb()
		return service.NewIssueTypeService(pool, GetIssueTypeRepository()), nil
	})
	return instance.(*service.IssueTypeService)
}

func GetSprintController() (*controller.SprintController, error) {
	instance, err := di.GetWithNew("sprint-controller", func() (any, error) {
		settings, err := GetAppSettingsService()
		if err != nil {
			return nil, err
		}
		return controller.NewSprintController(GetSprintRepository(), GetStateRepository(), GetSprintService(), GetAclService(), settings), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*controller.SprintController), nil
}

func GetSavedViewController() *controller.SavedViewController {
	instance, _ := di.GetWithNew("saved-view-controller", func() (any, error) {
		return controller.NewSavedViewController(GetSavedViewRepository(), GetAclService()), nil
	})
	return instance.(*controller.SavedViewController)
}

func GetAclRepository() *repository.AclRepository {
	instance, _ := di.GetWithNew("acl-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewAclRepository(pool), nil
	})
	return instance.(*repository.AclRepository)
}

func GetApiKeyRepository() *repository.ApiKeyRepository {
	instance, _ := di.GetWithNew("api-key-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewApiKeyRepository(pool), nil
	})
	return instance.(*repository.ApiKeyRepository)
}

func GetIssueService() *service.IssueService {
	instance, _ := di.GetWithNew("issue-service", func() (any, error) {
		pool := mustDb()
		return service.NewIssueService(pool), nil
	})
	return instance.(*service.IssueService)
}

func GetMCPServer() *mcp.MCPServer {
	instance, _ := di.GetWithNew("mcp-server", func() (any, error) {
		return mcp.NewMCPServer(mcp.LoadConfig(), GetHttpServer()), nil
	})
	return instance.(*mcp.MCPServer)
}

func GetApiKeyService() *service.ApiKeyService {
	instance, _ := di.GetWithNew("api-key-service", func() (any, error) {
		cache := mustCache()
		return service.NewApiKeyService(GetApiKeyRepository(), GetAdvisoryLockRepository(), cache, mustDb()), nil
	})
	return instance.(*service.ApiKeyService)
}

func GetTrackerRepository() *repository.TrackerRepository {
	instance, _ := di.GetWithNew("tracker-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewTrackerRepository(pool), nil
	})
	return instance.(*repository.TrackerRepository)
}

func GetPinRepository() *repository.PinRepository {
	instance, _ := di.GetWithNew("pin-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewPinRepository(pool), nil
	})
	return instance.(*repository.PinRepository)
}

func GetAdvisoryLockRepository() *repository.AdvisoryLockRepository {
	instance, _ := di.GetWithNew("advisory-lock-repository", func() (any, error) {
		return repository.NewAdvisoryLockRepository(mustDb()), nil
	})
	return instance.(*repository.AdvisoryLockRepository)
}

func GetUserService() *service.UserService {
	instance, _ := di.GetWithNew("user-service", func() (any, error) {
		cache := mustCache()
		return service.NewUserService(GetUserRepository(), GetAdvisoryLockRepository(), cache, mustDb()), nil
	})
	return instance.(*service.UserService)
}

func GetAclService() *service.AclService {
	instance, _ := di.GetWithNew("acl-service", func() (any, error) {
		cache := mustCache()
		return service.NewAclService(
			GetAclRepository(),
			GetProjectRepository(),
			GetTeamRepository(),
			GetUserRepository(),
			cache,
		), nil
	})
	return instance.(*service.AclService)
}

func GetUserController() *controller.UserController {
	instance, _ := di.GetWithNew("user-controller", func() (any, error) {
		return controller.NewUserController(GetUserService()), nil
	})
	return instance.(*controller.UserController)
}

func GetAdminController() *controller.AdminController {
	instance, _ := di.GetWithNew("admin-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewAdminController(
			GetUserService(),
			GetUserRepository(),
			GetProjectRepository(),
			GetApiKeyService(),
			GetAclService(),
			pool,
		), nil
	})
	return instance.(*controller.AdminController)
}

func GetTeamController() *controller.TeamController {
	instance, _ := di.GetWithNew("team-controller", func() (any, error) {
		return controller.NewTeamController(
			GetTeamRepository(),
			GetAclService(),
			GetNotificationService(),
		), nil
	})
	return instance.(*controller.TeamController)
}

func GetProjectController() *controller.ProjectController {
	instance, _ := di.GetWithNew("project-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewProjectController(
			GetProjectRepository(),
			GetTeamRepository(),
			GetSeverityRepository(),
			GetStateRepository(),
			GetIssueTypeRepository(),
			GetProjectSkillService(),
			GetAclService(),
			pool,
		), nil
	})
	return instance.(*controller.ProjectController)
}

func GetProjectMemberController() *controller.ProjectMemberController {
	instance, _ := di.GetWithNew("project-member-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewProjectMemberController(
			GetAclRepository(),
			GetProjectRepository(),
			GetUserRepository(),
			GetAclService(),
			pool,
		), nil
	})
	return instance.(*controller.ProjectMemberController)
}

func GetAgentRunRepository() *repository.AgentRunRepository {
	instance, _ := di.GetWithNew("agent-run-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewAgentRunRepository(pool).WithPhaseStateTransitioner(GetPhaseStateTransitioner()), nil
	})
	return instance.(*repository.AgentRunRepository)
}

func GetAgentTaskRepository() *repository.AgentTaskRepository {
	instance, _ := di.GetWithNew("agent-task-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewAgentTaskRepository(pool), nil
	})
	return instance.(*repository.AgentTaskRepository)
}

func GetAgentTaskService() *agent.TaskService {
	instance, _ := di.GetWithNew("agent-task-service", func() (any, error) {
		return agent.NewTaskService(GetAgentTaskRepository()), nil
	})
	return instance.(*agent.TaskService)
}

func GetAgentThinkingService() (*agent.ThinkingService, error) {
	instance, err := di.GetWithNew("agent-thinking-service", func() (any, error) {
		settings, err := GetAppSettingsService()
		if err != nil {
			return nil, err
		}
		return agent.NewThinkingService(
			GetAgentThinkingRepository(),
			GetAgentTaskRepository(),
			GetAgentRunRepository(),
			GetAgentThinkingNotifier(),
			settings,
		), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*agent.ThinkingService), nil
}

func GetAgentThinkingNotifier() *agent.ThinkingNotifier {
	instance, _ := di.GetWithNew("agent-thinking-notifier", func() (any, error) {
		return agent.NewThinkingNotifier(GetNotifier(), GetProjectRepository()), nil
	})
	return instance.(*agent.ThinkingNotifier)
}

func GetAgentThinkingRepository() *repository.AgentThinkingRepository {
	instance, _ := di.GetWithNew("agent-thinking-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewAgentThinkingRepository(pool), nil
	})
	return instance.(*repository.AgentThinkingRepository)
}

func GetBotGatewayRepository() *repository.BotGatewayRepository {
	instance, _ := di.GetWithNew("bot-gateway-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewBotGatewayRepository(pool), nil
	})
	return instance.(*repository.BotGatewayRepository)
}

func GetWebhookDedupRepository() *repository.WebhookDedupRepository {
	instance, _ := di.GetWithNew("webhook-dedup-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewWebhookDedupRepository(pool), nil
	})
	return instance.(*repository.WebhookDedupRepository)
}

func GetGatewayClient() *agent.GatewayClient {
	instance, _ := di.GetWithNew("gateway-client", func() (any, error) {
		return agent.NewGatewayClient(), nil
	})
	return instance.(*agent.GatewayClient)
}

func GetDispatcher() *agent.Dispatcher {
	instance, _ := di.GetWithNew("dispatcher", func() (any, error) {
		return agent.NewDispatcher(
			GetAgentRunRepository(),
			GetAgentTaskRepository(),
			GetBotGatewayRepository(),
			GetIssueRepository(),
			GetMessageRepository(),
			GetProjectRepository(),
			GetUserRepository(),
			GetSkillService(),
			GetStagePlanService(),
			GetGatewayClient(),
			GetNotifier(),
		), nil
	})
	return instance.(*agent.Dispatcher)
}

func GetSweep() *agent.Sweep {
	instance, _ := di.GetWithNew("sweep", func() (any, error) {
		return agent.NewSweep(
			GetAgentTaskRepository(),
			GetAgentRunRepository(),
			GetProjectRepository(),
			GetWebhookDedupRepository(),
			GetNotifier(),
		), nil
	})
	return instance.(*agent.Sweep)
}

func GetScheduler() *agent.Scheduler {
	instance, _ := di.GetWithNew("scheduler", func() (any, error) {
		return agent.NewScheduler(
			GetAgentRunRepository(),
			GetAgentTaskRepository(),
			GetProjectRepository(),
			GetDispatcher(),
			GetNotifier(),
		), nil
	})
	return instance.(*agent.Scheduler)
}

func GetJobScheduler() *scheduler.Scheduler {
	instance, _ := di.GetWithNew("job-scheduler", func() (any, error) {
		return scheduler.New(
			scheduler.Task{
				Name:       "sprint-snapshot",
				Interval:   time.Hour,
				RunOnStart: true,
				Run: func(ctx context.Context) error {
					_, err := GetSprintRepository().UpsertSnapshotsForOpenSprints(ctx)
					return err
				},
			},
			scheduler.Task{
				// Hourly, not weekly: a cancelled or swept stage has no thinking
				// to show until this runs.
				Name:       "agent-thinking-compaction",
				Interval:   time.Hour,
				RunOnStart: true,
				Run: func(ctx context.Context) error {
					thinking, err := GetAgentThinkingService()
					if err != nil {
						return err
					}
					_, err = thinking.CompactOrphaned(ctx)
					return err
				},
			},
			scheduler.Task{
				Name:     "agent-thinking-tail-sweep",
				Interval: time.Hour,
				Run: func(ctx context.Context) error {
					thinking, err := GetAgentThinkingService()
					if err != nil {
						return err
					}
					thinking.SweepTails()
					return nil
				},
			},
		), nil
	})
	return instance.(*scheduler.Scheduler)
}

func GetAgentRunController() (*controller.AgentRunController, error) {
	instance, err := di.GetWithNew("agent-run-controller", func() (any, error) {
		pool := mustDb()
		thinking, err := GetAgentThinkingService()
		if err != nil {
			return nil, err
		}
		return controller.NewAgentRunController(
			GetAgentRunRepository(),
			GetAgentTaskRepository(),
			GetAgentTaskService(),
			thinking,
			GetBotGatewayRepository(),
			GetIssueRepository(),
			GetProjectRepository(),
			GetMessageRepository(),
			GetGitIntegrationRepository(),
			GetAclService(),
			GetStagePlanService(),
			GetDispatcher(),
			GetNotifier(),
			pool,
		), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*controller.AgentRunController), nil
}

func GetBotGatewayController() *controller.BotGatewayController {
	instance, _ := di.GetWithNew("bot-gateway-controller", func() (any, error) {
		return controller.NewBotGatewayController(
			GetBotGatewayRepository(),
			GetUserRepository(),
		), nil
	})
	return instance.(*controller.BotGatewayController)
}

func GetMessageController() *controller.MessageController {
	instance, _ := di.GetWithNew("message-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewMessageController(
			GetMessageRepository(),
			GetTeamRepository(),
			GetProjectRepository(),
			GetUserRepository(),
			GetIssueRepository(),
			GetNotifier(),
			GetAclService(),
			GetNotificationService(),
			GetIssueParticipantRepository(),
			pool,
		).WithAgentRun(GetAgentRunRepository(), GetAgentTaskRepository(), GetBotGatewayRepository(), GetDispatcher(), GetNotifier()), nil
	})
	return instance.(*controller.MessageController)
}

func GetIssueParticipantRepository() *repository.IssueParticipantRepository {
	instance, _ := di.GetWithNew("issue-participant-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewIssueParticipantRepository(pool), nil
	})
	return instance.(*repository.IssueParticipantRepository)
}

func GetIssueController() *controller.IssueController {
	instance, _ := di.GetWithNew("issue-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewIssueController(
			GetIssueRepository(),
			GetProjectRepository(),
			GetUserRepository(),
			GetStateRepository(),
			GetSeverityRepository(),
			GetIssueTypeRepository(),
			GetIssueParticipantRepository(),
			GetAclService(),
			GetNotificationService(),
			pool,
		).WithGitIntRepo(GetGitIntegrationRepository()).
			WithAgentRun(GetAgentRunRepository(), GetAgentTaskRepository(), GetBotGatewayRepository(),
				GetProjectSkillService(), GetStagePlanService(), GetDispatcher(), GetNotifier()), nil
	})
	return instance.(*controller.IssueController)
}

func GetSeverityController() *controller.SeverityController {
	instance, _ := di.GetWithNew("severity-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewSeverityController(GetSeverityRepository(), GetSeverityService(), GetAclService(), GetProjectRepository(), pool), nil
	})
	return instance.(*controller.SeverityController)
}

func GetIssueTypeController() *controller.IssueTypeController {
	instance, _ := di.GetWithNew("issue-type-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewIssueTypeController(GetIssueTypeRepository(), GetIssueTypeService(), GetAclService(), pool), nil
	})
	return instance.(*controller.IssueTypeController)
}

func GetStateController() *controller.StateController {
	instance, _ := di.GetWithNew("state-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewStateController(GetStateRepository(), GetStateService(), GetAclService(), GetProjectRepository(), pool), nil
	})
	return instance.(*controller.StateController)
}

func GetWebsocketController() *controller.WebsocketController {
	instance, _ := di.GetWithNew("websocket-controller", func() (any, error) {
		return controller.NewWebsocketController(GetNotifier()), nil
	})
	return instance.(*controller.WebsocketController)
}

func GetTrackerController() *controller.TrackerController {
	instance, _ := di.GetWithNew("tracker-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewTrackerController(
			GetAclService(),
			GetTrackerRepository(),
			GetProjectRepository(),
			GetIssueRepository(),
			pool,
		), nil
	})
	return instance.(*controller.TrackerController)
}

func GetGanttOrderController() *controller.GanttOrderController {
	instance, _ := di.GetWithNew("gantt-order-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewGanttOrderController(GetIssueRepository(), GetAclService(), pool), nil
	})
	return instance.(*controller.GanttOrderController)
}

func GetPinController() *controller.PinController {
	instance, _ := di.GetWithNew("pin-controller", func() (any, error) {
		return controller.NewPinController(GetPinRepository(), GetAclService()), nil
	})
	return instance.(*controller.PinController)
}

func GetIssueRelationRepository() *repository.IssueRelationRepository {
	instance, _ := di.GetWithNew("issue-relation-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewIssueRelationRepository(pool), nil
	})
	return instance.(*repository.IssueRelationRepository)
}

func GetAIProvider() ai.Provider {
	instance, _ := di.GetWithNew("ai-provider", func() (any, error) {
		apiKey := viper.GetString("AI_API_KEY")
		model := viper.GetString("AI_MODEL")
		host := viper.GetString("AI_HOST")
		// AI_TIMEOUT_SECONDS bounds one provider request; <= 0 uses the provider default.
		aiTimeout := time.Duration(viper.GetInt("AI_TIMEOUT_SECONDS")) * time.Second
		// No hardcoded model default: comes solely from AI_MODEL (or AI_QUALITY_MODEL
		// for the checker); unset returns errs.ErrAiNotConfigured.
		switch viper.GetString("AI_PROVIDER") {
		case "gemini":
			return ai.NewGeminiProvider(apiKey, model), nil
		case "openai":
			return ai.NewOpenAIProvider(host, apiKey, model, aiTimeout), nil
		case "ollama":
			return ai.NewOllamaProvider(host, model), nil
		default: // "anthropic" or unset
			return ai.NewAnthropicProvider(host, apiKey, model), nil
		}
	})
	return instance.(ai.Provider)
}

func GetProjectBuilderController() *controller.ProjectBuilderController {
	instance, _ := di.GetWithNew("project-builder-controller", func() (any, error) {
		pool := mustDb()
		cache := mustCache()
		return controller.NewProjectBuilderController(
			pool,
			GetAIProvider(),
			cache,
			GetIssueRepository(),
			GetIssueRelationRepository(),
			GetAclService(),
			GetNotifier(),
		), nil
	})
	return instance.(*controller.ProjectBuilderController)
}

func GetIssueParticipantController() *controller.IssueParticipantController {
	instance, _ := di.GetWithNew("issue-participant-controller", func() (any, error) {
		return controller.NewIssueParticipantController(
			GetIssueParticipantRepository(),
			GetIssueRepository(),
			GetProjectRepository(),
			GetAclService(),
			GetNotifier(),
		), nil
	})
	return instance.(*controller.IssueParticipantController)
}

func GetIssueRelationController() *controller.IssueRelationController {
	instance, _ := di.GetWithNew("issue-relation-controller", func() (any, error) {
		pool := mustDb()
		return controller.NewIssueRelationController(
			GetIssueRelationRepository(),
			GetIssueRepository(),
			GetProjectRepository(),
			GetAclService(),
			GetNotifier(),
			pool,
		), nil
	})
	return instance.(*controller.IssueRelationController)
}

func GetSplitService() *service.SplitService {
	instance, _ := di.GetWithNew("split-service", func() (any, error) {
		pool := mustDb()
		return service.NewSplitService(pool, GetAIProvider(), GetIssueRepository(), GetIssueRelationRepository(), GetStateRepository()), nil
	})
	return instance.(*service.SplitService)
}

func GetSplitController() *controller.SplitController {
	instance, _ := di.GetWithNew("split-controller", func() (any, error) {
		cache := mustCache()
		return controller.NewSplitController(GetSplitService(), GetAclService(), cache, GetIssueRepository()), nil
	})
	return instance.(*controller.SplitController)
}

func GetNotificationRepository() *repository.NotificationRepository {
	instance, _ := di.GetWithNew("notification-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewNotificationRepository(pool), nil
	})
	return instance.(*repository.NotificationRepository)
}

func GetNotificationService() *service.NotificationService {
	instance, _ := di.GetWithNew("notification-service", func() (any, error) {
		return service.NewNotificationService(GetNotificationRepository(), GetNotifier()), nil
	})
	return instance.(*service.NotificationService)
}

func GetNotificationController() *controller.NotificationController {
	instance, _ := di.GetWithNew("notification-controller", func() (any, error) {
		return controller.NewNotificationController(GetNotificationRepository()), nil
	})
	return instance.(*controller.NotificationController)
}

func GetQualityRepository() *repository.QualityRepository {
	instance, _ := di.GetWithNew("quality-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewQualityRepository(pool), nil
	})
	return instance.(*repository.QualityRepository)
}

func GetQualityService() *service.QualityService {
	instance, _ := di.GetWithNew("quality-service", func() (any, error) {
		return service.NewQualityService(GetAIProvider(), GetQualityRepository(), GetIssueRepository()), nil
	})
	return instance.(*service.QualityService)
}

func GetQualityController() *controller.QualityController {
	instance, _ := di.GetWithNew("quality-controller", func() (any, error) {
		cache := mustCache()
		return controller.NewQualityController(GetQualityService(), GetAclService(), GetIssueRepository(), cache), nil
	})
	return instance.(*controller.QualityController)
}

func GetMyIssuesController() *controller.MyIssuesController {
	instance, _ := di.GetWithNew("my-issues-controller", func() (any, error) {
		return controller.NewMyIssuesController(GetIssueRepository()), nil
	})
	return instance.(*controller.MyIssuesController)
}

func GetGitIntegrationRepository() *repository.GitIntegrationRepository {
	instance, _ := di.GetWithNew("git-integration-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewGitIntegrationRepository(pool), nil
	})
	return instance.(*repository.GitIntegrationRepository)
}

func GetDiffCache() *githost.DiffCache {
	instance, _ := di.GetWithNew("diff-cache", func() (any, error) {
		return githost.NewDiffCache(500, 200), nil
	})
	return instance.(*githost.DiffCache)
}

func GetGitIntegrationController() *controller.GitIntegrationController {
	instance, _ := di.GetWithNew("git-integration-controller", func() (any, error) {
		return controller.NewGitIntegrationController(
			GetGitIntegrationRepository(),
			GetAclService(),
			GetDiffCache(),
		), nil
	})
	return instance.(*controller.GitIntegrationController)
}

func GetMergePoller() *agent.MergePoller {
	instance, _ := di.GetWithNew("merge-poller", func() (any, error) {
		return agent.NewMergePoller(
			GetAgentRunRepository(),
			GetAgentTaskRepository(),
			GetProjectRepository(),
			GetGitIntegrationRepository(),
			GetIssueRepository(),
			GetStateRepository(),
			GetPhaseStateTransitioner(),
			GetNotifier(),
		), nil
	})
	return instance.(*agent.MergePoller)
}

func GetWorkflowEventMapRepository() *repository.WorkflowEventMapRepository {
	instance, _ := di.GetWithNew("workflow-event-map-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewWorkflowEventMapRepository(pool), nil
	})
	return instance.(*repository.WorkflowEventMapRepository)
}

func GetSkillRepository() *repository.SkillRepository {
	instance, _ := di.GetWithNew("skill-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewSkillRepository(pool), nil
	})
	return instance.(*repository.SkillRepository)
}

func GetProjectSkillRepository() *repository.ProjectSkillRepository {
	instance, _ := di.GetWithNew("project-skill-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewProjectSkillRepository(pool), nil
	})
	return instance.(*repository.ProjectSkillRepository)
}

func GetSkillService() *service.SkillService {
	instance, _ := di.GetWithNew("skill-service", func() (any, error) {
		return service.NewSkillService(GetSkillRepository(), GetProjectSkillRepository()), nil
	})
	return instance.(*service.SkillService)
}

func GetProjectSkillService() *service.ProjectSkillService {
	instance, _ := di.GetWithNew("project-skill-service", func() (any, error) {
		return service.NewProjectSkillService(GetProjectSkillRepository(), GetSkillRepository()), nil
	})
	return instance.(*service.ProjectSkillService)
}

func GetStagePlanService() *service.StagePlanService {
	instance, _ := di.GetWithNew("stage-plan-service", func() (any, error) {
		pool := mustDb()
		return service.NewStagePlanService(pool, GetAgentRunRepository()), nil
	})
	return instance.(*service.StagePlanService)
}

func GetProjectSkillController() *controller.ProjectSkillController {
	instance, _ := di.GetWithNew("project-skill-controller", func() (any, error) {
		return controller.NewProjectSkillController(
			GetProjectSkillRepository(),
			GetProjectSkillService(),
			GetAclService(),
		), nil
	})
	return instance.(*controller.ProjectSkillController)
}

func GetAgentOverviewRepository() *repository.AgentOverviewRepository {
	instance, _ := di.GetWithNew("agent-overview-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewAgentOverviewRepository(pool), nil
	})
	return instance.(*repository.AgentOverviewRepository)
}

func GetAgentThinkingController() (*controller.AgentThinkingController, error) {
	instance, err := di.GetWithNew("agent-thinking-controller", func() (any, error) {
		thinking, err := GetAgentThinkingService()
		if err != nil {
			return nil, err
		}
		return controller.NewAgentThinkingController(
			thinking,
			GetAgentTaskService(),
			GetAgentRunRepository(),
			GetAclService(),
		), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*controller.AgentThinkingController), nil
}

func GetAgentOverviewController() *controller.AgentOverviewController {
	instance, _ := di.GetWithNew("agent-overview-controller", func() (any, error) {
		return controller.NewAgentOverviewController(
			GetAgentOverviewRepository(),
			GetAclService(),
		), nil
	})
	return instance.(*controller.AgentOverviewController)
}

func GetSkillController() *controller.SkillController {
	instance, _ := di.GetWithNew("skill-controller", func() (any, error) {
		return controller.NewSkillController(GetSkillService()), nil
	})
	return instance.(*controller.SkillController)
}

func GetPhaseStateTransitioner() *agent.PhaseStateTransitioner {
	instance, _ := di.GetWithNew("workflow-event-mirror", func() (any, error) {
		return agent.NewPhaseStateTransitioner(
			GetWorkflowEventMapRepository(),
			GetIssueRepository(),
			GetStateRepository(),
			GetProjectRepository(),
			GetNotifier(),
		), nil
	})
	return instance.(*agent.PhaseStateTransitioner)
}

func GetWorkflowEventMapController() *controller.WorkflowEventMapController {
	instance, _ := di.GetWithNew("workflow-event-map-controller", func() (any, error) {
		return controller.NewWorkflowEventMapController(
			GetWorkflowEventMapRepository(),
			GetStateRepository(),
			GetAclService(),
		), nil
	})
	return instance.(*controller.WorkflowEventMapController)
}

func GetHealthController() *controller.HealthController {
	instance, _ := di.GetWithNew("health-controller", func() (any, error) {
		pool := mustDb()
		cache := mustCache()
		checks := []controller.HealthCheck{
			{Name: "db", Ping: func(ctx context.Context) error { return pool.Ping(ctx) }},
			{Name: "cache", Ping: func(ctx context.Context) error { return cache.Ping(ctx).Err() }},
		}
		return controller.NewHealthController(checks), nil
	})
	return instance.(*controller.HealthController)
}

func GetApiKeyController() (*controller.ApiKeyController, error) {
	instance, err := di.GetWithNew("api-key-controller", func() (any, error) {
		settings, err := GetAppSettingsService()
		if err != nil {
			return nil, err
		}
		return controller.NewApiKeyController(GetApiKeyService(), settings), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*controller.ApiKeyController), nil
}

func GetVersionController() *controller.VersionController {
	instance, _ := di.GetWithNew("version-controller", func() (any, error) {
		return controller.NewVersionController(), nil
	})
	return instance.(*controller.VersionController)
}

func GetRouter() (*router.Router, error) {
	instance, err := di.GetWithNew("router", func() (any, error) {
		if _, err := GetDb(); err != nil {
			return nil, err
		}

		cache, err := GetCache()
		if err != nil {
			return nil, err
		}

		appSettingsController, err := GetAppSettingsController()
		if err != nil {
			return nil, err
		}

		sprintController, err := GetSprintController()
		if err != nil {
			return nil, err
		}

		apiKeyController, err := GetApiKeyController()
		if err != nil {
			return nil, err
		}

		agentRunController, err := GetAgentRunController()
		if err != nil {
			return nil, err
		}

		agentThinkingController, err := GetAgentThinkingController()
		if err != nil {
			return nil, err
		}

		return router.New(
			GetHttpServer(),
			GetBaseLogger(),
			cache,
			GetApiKeyService(),
			GetUserController(),
			GetTeamController(),
			GetProjectController(),
			GetProjectMemberController(),
			GetMessageController(),
			GetWebsocketController(),
			GetIssueController(),
			GetSeverityController(),
			GetIssueTypeController(),
			GetStateController(),
			sprintController,
			GetSavedViewController(),
			GetTrackerController(),
			GetPinController(),
			GetIssueRelationController(),
			GetGanttOrderController(),
			GetIssueParticipantController(),
			GetProjectBuilderController(),
			GetSplitController(),
			GetQualityController(),
			GetNotificationController(),
			GetAdminController(),
			GetMyIssuesController(),
			GetGitIntegrationController(),
			agentRunController,
			agentThinkingController,
			GetBotGatewayController(),
			GetWorkflowEventMapController(),
			GetSkillController(),
			GetProjectSkillController(),
			GetAgentOverviewController(),
			appSettingsController,
			apiKeyController,
			GetHealthController(),
			GetVersionController(),
			GetMCPServer().Handler(),
		), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*router.Router), nil
}

func GetAppSettingsRepository() *repository.AppSettingsRepository {
	instance, _ := di.GetWithNew("app-settings-repository", func() (any, error) {
		pool := mustDb()
		return repository.NewAppSettingsRepository(pool), nil
	})
	return instance.(*repository.AppSettingsRepository)
}

func GetAppSettingsService() (*service.AppSettingsService, error) {
	instance, err := di.GetWithNew("app-settings-service", func() (any, error) {
		svc := service.NewAppSettingsService(GetAppSettingsRepository())
		if err := svc.Load(context.Background()); err != nil {
			return nil, fmt.Errorf("loading app settings at boot: %w", err)
		}
		return svc, nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*service.AppSettingsService), nil
}

func GetAppSettingsController() (*controller.AppSettingsController, error) {
	instance, err := di.GetWithNew("app-settings-controller", func() (any, error) {
		svc, err := GetAppSettingsService()
		if err != nil {
			return nil, err
		}
		return controller.NewAppSettingsController(svc, mustDb()), nil
	})
	if err != nil {
		return nil, err
	}
	return instance.(*controller.AppSettingsController), nil
}
