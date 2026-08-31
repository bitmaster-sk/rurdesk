package router

import (
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Router struct {
	engine *gin.Engine
}

func New(
	engine *gin.Engine,
	baseLogger zerolog.Logger,
	cache *redis.Client,
	apiKeyAuth middleware.ApiKeyAuthenticator,
	userCtrl *controller.UserController,
	teamCtrl *controller.TeamController,
	projectCtrl *controller.ProjectController,
	memberCtrl *controller.ProjectMemberController,
	msgCtrl *controller.MessageController,
	wsCtrl *controller.WebsocketController,
	issueCtrl *controller.IssueController,
	sevCtrl *controller.SeverityController,
	itCtrl *controller.IssueTypeController,
	stateCtrl *controller.StateController,
	sprintCtrl *controller.SprintController,
	savedViewCtrl *controller.SavedViewController,
	trackerCtrl *controller.TrackerController,
	pinCtrl *controller.PinController,
	irc *controller.IssueRelationController,
	ganttOrderCtrl *controller.GanttOrderController,
	participantCtrl *controller.IssueParticipantController,
	pbCtrl *controller.ProjectBuilderController,
	splitCtrl *controller.SplitController,
	qualityCtrl *controller.QualityController,
	notifCtrl *controller.NotificationController,
	adminCtrl *controller.AdminController,
	myIssuesCtrl *controller.MyIssuesController,
	gitIntCtrl *controller.GitIntegrationController,
	agentRunCtrl *controller.AgentRunController,
	botGwCtrl *controller.BotGatewayController,
	workflowEventMapCtrl *controller.WorkflowEventMapController,
	skillCtrl *controller.SkillController,
	projectSkillCtrl *controller.ProjectSkillController,
	agentOverviewCtrl *controller.AgentOverviewController,
	appSettingsCtrl *controller.AppSettingsController,
	apiKeyCtrl *controller.ApiKeyController,
	healthCtrl *controller.HealthController,
	versionCtrl *controller.VersionController,
	mcpHandler http.Handler,
) *Router {
	// Health probes and MCP are registered before the global middleware: health
	// polling must not spam the request log, and MCP's long-lived SSE streams
	// must bypass the logger (flushes only on close) and the CORS OPTIONS
	// short-circuit.
	engine.GET("/healthz", healthCtrl.Live)
	engine.GET("/healthz/ready", healthCtrl.Ready)
	registerMCPHandler(engine, mcpHandler)

	engine.Use(middleware.Logger(baseLogger))
	engine.Use(middleware.Cors())
	// Renders a uniform {code, message, translateKey} body for any handler that
	// signalled failure via c.Error + c.Status without writing its own body.
	engine.Use(middleware.ErrorRenderer())

	api := engine.Group("/api")

	pub := api.Group("/public")
	pub.POST("/register", userCtrl.Register)
	pub.POST("/login", userCtrl.Login)

	auth := middleware.Auth(cache, apiKeyAuth)
	pri := api.Group("/private", auth)

	pri.GET("/user", userCtrl.GetByToken)
	pri.GET("/users", userCtrl.ListUsers)
	pri.PATCH("/user", userCtrl.UpdateUser)
	pri.PUT("/user/password", userCtrl.ChangePassword)
	pri.DELETE("/logout", userCtrl.Logout)

	pri.GET("/user/api-key", apiKeyCtrl.List)
	pri.POST("/user/api-key", apiKeyCtrl.Create)
	pri.POST("/user/api-key/:idApiKey/token", apiKeyCtrl.Regenerate)
	pri.DELETE("/user/api-key/:idApiKey", apiKeyCtrl.Revoke)

	// App settings — readable by any authenticated user (clients need page sizes)
	pri.GET("/settings", appSettingsCtrl.Get)

	pri.GET("/skills", skillCtrl.List)

	// Admin (global instance admin only)
	admin := pri.Group("/admin", middleware.AdminOnly())
	admin.PATCH("/settings", appSettingsCtrl.Update)
	admin.GET("/version", versionCtrl.Get)
	admin.GET("/user", adminCtrl.ListUsers)
	admin.POST("/user", adminCtrl.CreateUser)
	admin.PATCH("/user/:idUser", adminCtrl.UpdateUser)
	admin.DELETE("/user/:idUser", adminCtrl.DeleteUser)
	admin.GET("/user/:idUser/api-key", adminCtrl.GetAgentApiKey)
	admin.POST("/user/:idUser/api-key", adminCtrl.CreateAgentApiKey)
	admin.POST("/user/:idUser/api-key/token", adminCtrl.RegenerateAgentApiKey)
	admin.DELETE("/user/:idUser/api-key", adminCtrl.RevokeAgentApiKey)

	admin.GET("/skills/:idSkill", skillCtrl.Get)
	admin.POST("/skills", skillCtrl.Create)
	admin.PATCH("/skills/:idSkill", skillCtrl.Update)
	admin.DELETE("/skills/:idSkill", skillCtrl.Delete)
	admin.POST("/skills/:idSkill/restore", skillCtrl.Restore)

	// Bot gateway (1:1 with bot user)
	admin.GET("/user/:idUser/gateway", botGwCtrl.GetBotGateway)
	admin.POST("/user/:idUser/gateway", botGwCtrl.CreateBotGateway)
	admin.PATCH("/user/:idUser/gateway", botGwCtrl.UpdateBotGateway)
	admin.POST("/user/:idUser/gateway/token", botGwCtrl.RegenerateGatewayToken)
	admin.DELETE("/user/:idUser/gateway", botGwCtrl.DeleteBotGateway)

	// Team — management is instance-admin only
	admin.POST("/team", teamCtrl.CreateTeam)
	admin.PATCH("/team", teamCtrl.UpdateTeam)
	admin.DELETE("/team", teamCtrl.DeleteTeam)
	admin.GET("/team/:idTeam/member", teamCtrl.GetTeamMembers)
	admin.POST("/team/member", teamCtrl.AddTeamMember)
	admin.DELETE("/team/member", teamCtrl.DeleteTeamMember)

	// Team — reads for any authenticated user
	pri.GET("/team", teamCtrl.GetAllTeams)                      // all teams (project-member dropdowns, admin screen)
	pri.GET("/team/my", teamCtrl.GetMyTeams)                    // caller's teams (chat menu)
	pri.GET("/team/:idTeam/members", teamCtrl.GetMyTeamMembers) // members of a team (caller must be a member)

	pri.GET("/message", msgCtrl.GetMessages)
	pri.GET("/message/unread", msgCtrl.GetUnreadMessages)
	pri.POST("/message", msgCtrl.CreateMessage)
	pri.POST("/message/read", msgCtrl.SetReadMessages)
	pri.PATCH("/message/:id", msgCtrl.UpdateMessage)

	pri.GET("/project", projectCtrl.GetProjects)
	pri.GET("/project/:idProject", projectCtrl.GetProject)
	pri.GET("/project/:idProject/members", projectCtrl.GetProjectMembers)
	pri.GET("/project/:idProject/user-role", projectCtrl.GetUserRole)
	pri.POST("/project", projectCtrl.CreateProject)
	pri.PATCH("/project", projectCtrl.UpdateProject)
	pri.DELETE("/project/:idProject", projectCtrl.DeleteProject)

	// Project member management
	pri.GET("/project/:idProject/member", memberCtrl.GetMembers)
	pri.POST("/project/:idProject/member/user", memberCtrl.AddUser)
	pri.PATCH("/project/:idProject/member/user/:idUser", memberCtrl.UpdateUserRole)
	pri.DELETE("/project/:idProject/member/user/:idUser", memberCtrl.RemoveUser)
	pri.POST("/project/:idProject/member/team", memberCtrl.AddTeam)
	pri.PATCH("/project/:idProject/member/team/:idTeam", memberCtrl.UpdateTeamRole)
	pri.DELETE("/project/:idProject/member/team/:idTeam", memberCtrl.RemoveTeam)

	pri.GET("/project/:idProject/issue", issueCtrl.GetIssues)
	pri.GET("/project/:idProject/issue/:idIssuePublic", issueCtrl.GetIssue)
	pri.POST("/project/:idProject/issue", issueCtrl.CreateIssue)
	pri.PATCH("/project/:idProject/issue/batch", issueCtrl.BulkEditIssues)
	pri.PATCH("/project/:idProject/issue/:idIssuePublic", issueCtrl.EditIssue)
	pri.DELETE("/project/:idProject/issue/:idIssuePublic", issueCtrl.DeleteIssue)
	pri.PUT("/project/:idProject/gantt-order", ganttOrderCtrl.Reorder)

	pri.GET("/project/:idProject/sprint", sprintCtrl.List)
	pri.POST("/project/:idProject/sprint", sprintCtrl.Create)
	pri.PATCH("/project/:idProject/issue/:idIssuePublic/sprint", sprintCtrl.AssignIssue)
	pri.PATCH("/sprint/:idSprint", sprintCtrl.Edit)
	pri.DELETE("/sprint/:idSprint", sprintCtrl.Delete)
	pri.POST("/sprint/:idSprint/close", sprintCtrl.Close)
	pri.GET("/sprint/:idSprint/stats", sprintCtrl.SprintStats)
	pri.GET("/sprint/:idSprint/burndown", sprintCtrl.Burndown)
	pri.GET("/project/:idProject/backlog/stats", sprintCtrl.BacklogStats)
	pri.GET("/project/:idProject/sprint/velocity", sprintCtrl.Velocity)

	pri.GET("/project/:idProject/saved-view", savedViewCtrl.List)
	pri.POST("/project/:idProject/saved-view", savedViewCtrl.Create)
	pri.PATCH("/saved-view/:idSavedView", savedViewCtrl.Edit)
	pri.DELETE("/saved-view/:idSavedView", savedViewCtrl.Delete)

	// Project builder
	pri.POST("/project/:idProject/project-builder/generate", pbCtrl.Generate)
	pri.POST("/project/:idProject/project-builder/accept", pbCtrl.Accept)

	// Issue relations
	pri.GET("/project/:idProject/relation", irc.GetRelationsBulk)
	pri.GET("/project/:idProject/issue/:idIssuePublic/relation", irc.GetRelations)
	pri.POST("/project/:idProject/issue/:idIssuePublic/relation", irc.CreateRelation)
	pri.PATCH("/project/:idProject/issue/:idIssuePublic/relation/:idRelation", irc.UpdateRelation)
	pri.DELETE("/project/:idProject/issue/:idIssuePublic/relation/:idRelation", irc.DeleteRelation)

	// Issue participants
	pri.GET("/project/:idProject/issue/:idIssuePublic/participant", participantCtrl.GetParticipants)
	pri.POST("/project/:idProject/issue/:idIssuePublic/participant", participantCtrl.AddParticipant)
	pri.PATCH("/project/:idProject/issue/:idIssuePublic/participant/notifications", participantCtrl.SetMyNotifications)

	// Split
	pri.POST("/project/:idProject/issue/:idIssuePublic/split", splitCtrl.Preview)
	pri.POST("/project/:idProject/issue/:idIssuePublic/split/accept", splitCtrl.Accept)

	// Quality
	pri.POST("/project/:idProject/quality", qualityCtrl.Preview)
	pri.POST("/project/:idProject/issue/:idIssuePublic/quality", qualityCtrl.Check)
	pri.GET("/project/:idProject/issue/:idIssuePublic/quality", qualityCtrl.GetQuality)

	pri.GET("/state", stateCtrl.GetStates)
	pri.POST("/state", stateCtrl.CreateState)
	pri.PATCH("/state/:idState", stateCtrl.EditState)
	pri.DELETE("/state/:idState/project/:idProject", stateCtrl.DeleteState)
	pri.GET("/state/:idState/project/:idProject/usage", stateCtrl.GetStateUsage)

	pri.GET("/severity", sevCtrl.GetSeverities)
	pri.POST("/severity", sevCtrl.CreateSeverity)
	pri.PATCH("/severity/:idSeverity", sevCtrl.EditSeverity)
	pri.DELETE("/severity/:idSeverity/project/:idProject", sevCtrl.DeleteSeverity)
	pri.GET("/severity/:idSeverity/project/:idProject/usage", sevCtrl.GetSeverityUsage)

	pri.GET("/issue-type", itCtrl.GetIssueTypes)
	pri.POST("/issue-type", itCtrl.CreateIssueType)
	pri.PATCH("/issue-type/:idIssueType", itCtrl.EditIssueType)
	pri.DELETE("/issue-type/:idIssueType/project/:idProject", itCtrl.DeleteIssueType)
	pri.GET("/issue-type/:idIssueType/project/:idProject/usage", itCtrl.GetIssueTypeUsage)

	pri.GET("/tracker", trackerCtrl.GetTracker)
	pri.POST("/tracker", trackerCtrl.CreateTracker)
	pri.PATCH("/tracker/:idTracker/submit", trackerCtrl.SubmitTracker)
	pri.DELETE("/tracker/:idTracker", trackerCtrl.DeleteTracker)

	pri.GET("/track", trackerCtrl.GetTracks)
	pri.POST("/track", trackerCtrl.CreateTrack)
	pri.PATCH("/track/:idTrack", trackerCtrl.EditTrack)
	pri.DELETE("/track/:idTrack", trackerCtrl.DeleteTrack)

	pri.GET("/pin", pinCtrl.GetPins)
	pri.POST("/pin", pinCtrl.CreatePin)
	pri.DELETE("/pin/:idPin", pinCtrl.DeletePin)
	pri.GET("/pin/destination-type", pinCtrl.GetPinDestinationTypes)

	// My Issues
	pri.GET("/my-issues", myIssuesCtrl.GetMyIssues)

	// Git integration CRUD
	pri.GET("/project/:idProject/git-integration", gitIntCtrl.List)
	pri.POST("/project/:idProject/git-integration", gitIntCtrl.Create)
	pri.GET("/project/:idProject/git-integration/:idGitIntegration", gitIntCtrl.Get)
	pri.PUT("/project/:idProject/git-integration/:idGitIntegration", gitIntCtrl.Update)
	pri.DELETE("/project/:idProject/git-integration/:idGitIntegration", gitIntCtrl.Delete)

	// MR diff & status (read-only, cached)
	pri.GET("/project/:idProject/git-integration/:idGitIntegration/mr/:mrId/diff", gitIntCtrl.GetDiff)
	pri.GET("/project/:idProject/git-integration/:idGitIntegration/mr/:mrId/status", gitIntCtrl.GetStatus)

	// WebSocket
	pri.GET("/ws", wsCtrl.Connect)

	// Notifications
	pri.GET("/notification", notifCtrl.List)
	pri.POST("/notification/read", notifCtrl.MarkAllRead)
	pri.PUT("/notification/:id/read", notifCtrl.MarkRead)
	pri.DELETE("/notification/:id", notifCtrl.Delete)

	// Agent runs (user-facing)
	pri.GET("/agent/run/:idRun", agentRunCtrl.GetRun)
	pri.POST("/agent/run/:idRun/approve", agentRunCtrl.Approve)
	pri.POST("/agent/run/:idRun/cancel", agentRunCtrl.Cancel)
	pri.POST("/agent/run/:idRun/continue", agentRunCtrl.Continue)
	pri.POST("/agent/run/:idRun/restart", agentRunCtrl.Restart)
	pri.GET("/agent/run/:idRun/stats", agentRunCtrl.Stats)
	pri.GET("/agent/run/:idRun/skills", agentRunCtrl.GetSkills)
	pri.PATCH("/agent/run/:idRun/skills", agentRunCtrl.PatchSkills)
	pri.GET("/project/:idProject/agent/runs", agentRunCtrl.GetRunsByProject)
	pri.GET("/project/:idProject/issue/:idIssuePublic/agent/run", agentRunCtrl.GetRunByIssue)

	// Gateway-facing callbacks. These share the ordinary authenticated group —
	// middleware.Auth accepts user JWTs and bot API keys alike — so each handler
	// authorizes itself against the run's own bot (requireRunBot/requireTaskBot).
	// Do not add a callback here without that check.
	pri.POST("/agent/run/:idRun/repo", agentRunCtrl.ReportRunRepo)
	pri.POST("/agent/task/:idTask/complete", agentRunCtrl.CompleteStage)
	pri.POST("/agent/task/:idTask/heartbeat", agentRunCtrl.TaskHeartbeat)
	pri.POST("/agent/task/:idTask/stats", agentRunCtrl.TaskStats)
	pri.POST("/agent/gateway/recovered", agentRunCtrl.GatewayRecovered)

	// Workflow event→state map (project owner only)
	pri.GET("/project/:idProject/workflow-event-state-map", workflowEventMapCtrl.GetMappings)
	pri.PUT("/project/:idProject/workflow-event-state-map", workflowEventMapCtrl.ReplaceMappings)
	pri.POST("/project/:idProject/issue/:idIssuePublic/assign-agent", issueCtrl.AssignAgent)
	pri.GET("/project/:idProject/skills", projectSkillCtrl.Get)
	pri.PUT("/project/:idProject/skills", projectSkillCtrl.Replace)
	pri.GET("/project/:idProject/agents/overview", agentOverviewCtrl.Get)

	// In production the Go binary serves the Angular build itself (single
	// container, no nginx). Gated by SERVE_STATIC so dev — which serves the
	// frontend via `ng serve` — is unaffected.
	if viper.GetBool("SERVE_STATIC") {
		registerStaticServing(engine, viper.GetString("STATIC_DIR"))
	}

	return &Router{engine: engine}
}
