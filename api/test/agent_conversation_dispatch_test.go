package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify/notifytest"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
)

// AgentConversationDispatchSuite drives a chat turn end to end: a user message
// queues a turn, the scheduler dispatches it to a fake gateway with a run token,
// the agent replies through complete_stage and the reply lands in the conversation.
//
// Projects: P1 owner+agent+member, P3 owner+agent (the owner is removed from it
// by one test).
type AgentConversationDispatchSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	Svc         *service.AgentConversationService
	Owner       *model.User
	OwnerToken  string
	MemberToken string
	Agent       int64
	AgentKey    string
	Agents      []int64
	P1          int64
	P3          int64
	Gateway     *httptest.Server
	Events      chan []byte
	Sched       *agent.Scheduler
}

func TestAgentConversationDispatchSuite(t *testing.T) {
	suite.Run(t, new(AgentConversationDispatchSuite))
}

func (s *AgentConversationDispatchSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	ctx := context.Background()
	s.Svc = injector.GetAgentConversationService()

	s.OwnerToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat dispatch owner","email":"chat-dispatch-owner@test.sk","password":"kreslo"}`)
	owner, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-dispatch-owner@test.sk")
	s.Require().NoError(err)
	s.Owner = owner
	s.MemberToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat dispatch member","email":"chat-dispatch-member@test.sk","password":"kreslo"}`)
	member, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-dispatch-member@test.sk")
	s.Require().NoError(err)

	s.Events = make(chan []byte, 64)
	s.Gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.Events <- body
		w.WriteHeader(http.StatusOK)
	}))

	s.Agent, s.AgentKey = s.createAgent("chat dispatch bot", s.Gateway.URL)

	s.P1 = createProject(s.T(), s.App, s.Token, "chat-dispatch-p1")
	s.P3 = createProject(s.T(), s.App, s.Token, "chat-dispatch-p3")
	s.addMember(s.P1, s.Owner.IdUser, s.Agent, member.IdUser)
	s.addMember(s.P3, s.Owner.IdUser, s.Agent)

	// The tick scans every agent with open runs; only this suite's agents reach
	// the real dispatcher, so leftovers of other suites never hit a gateway.
	s.Sched = agent.NewScheduler(s.App.Pool, injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(), injector.GetIssueRepository(),
		&agentsOnlyDispatcher{suite: s, real: injector.GetDispatcher()}, injector.GetChatTurns(), injector.GetNotifier())
}

func (s *AgentConversationDispatchSuite) TearDownSuite() {
	s.cleanRuns()
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = ANY($1)", []int64{s.P1, s.P3})
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = ANY($1)", s.Agents)
	s.Gateway.Close()
}

// SetupTest starts every test with an idle agent: no open run, no queued event.
func (s *AgentConversationDispatchSuite) SetupTest() {
	s.cleanRuns()
	for {
		select {
		case <-s.Events:
		default:
			return
		}
	}
}

func (s *AgentConversationDispatchSuite) cleanRuns() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_user_agent = ANY($1)", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run_event WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = ANY($1))", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = ANY($1))", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_user_agent = ANY($1)", s.Agents)
}

type agentsOnlyDispatcher struct {
	suite *AgentConversationDispatchSuite
	real  agent.StageDispatcher
}

func (d *agentsOnlyDispatcher) isOurs(run *model.AgentRun) bool {
	for _, idAgent := range d.suite.Agents {
		if run.IdUserAgent == idAgent {
			return true
		}
	}
	return false
}

func (d *agentsOnlyDispatcher) DispatchStageExecute(ctx context.Context, run *model.AgentRun, task *model.AgentTask) {
	if d.isOurs(run) {
		d.real.DispatchStageExecute(ctx, run, task)
	}
}

func (d *agentsOnlyDispatcher) DispatchChatTurn(ctx context.Context, run *model.AgentRun, task *model.AgentTask, runToken string) {
	if d.isOurs(run) {
		d.real.DispatchChatTurn(ctx, run, task, runToken)
	}
}

func (s *AgentConversationDispatchSuite) createAgent(name, gatewayURL string) (int64, string) {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.Token)
	s.requireOk(res)
	var created model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	s.Agents = append(s.Agents, created.IdUser)

	gwRes := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", created.IdUser),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gatewayURL), s.Token)
	s.requireOk(gwRes)
	if created.RawKey != "" {
		return created.IdUser, created.RawKey
	}
	keyRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", created.IdUser), `{"name":"chat-dispatch-key"}`, s.Token)
	s.requireOk(keyRes)
	var key model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(keyRes.Body).Decode(&key))
	return created.IdUser, key.RawKey
}

func (s *AgentConversationDispatchSuite) addMember(idProject int64, idsUser ...int64) {
	for _, idUser := range idsUser {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
}

func (s *AgentConversationDispatchSuite) requireOk(res *http.Response) {
	s.T().Helper()
	if res.StatusCode != http.StatusOK {
		s.FailNow(fmt.Sprintf("status %d: %s", res.StatusCode, readBody(s.T(), res)))
	}
}

func (s *AgentConversationDispatchSuite) status(method, url, body, token string) int {
	s.T().Helper()
	res := Request(s.T(), s.App, method, url, body, token)
	readBody(s.T(), res)
	return res.StatusCode
}

func (s *AgentConversationDispatchSuite) create(idProject *int64) *model.AgentConversation {
	conv, err := s.Svc.CreateConversation(context.Background(), s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: s.Agent, IdProject: idProject})
	s.Require().NoError(err)
	return conv
}

func (s *AgentConversationDispatchSuite) post(conv *model.AgentConversation, text string) *model.AgentConversationMessage {
	s.T().Helper()
	message, err := s.Svc.CreateMessage(context.Background(), s.Owner, conv.IdConversation, text)
	s.Require().NoError(err)
	return message
}

func (s *AgentConversationDispatchSuite) load(conv *model.AgentConversation) *model.AgentConversation {
	s.T().Helper()
	loaded, err := s.Svc.LoadConversation(context.Background(), s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	return loaded
}

func (s *AgentConversationDispatchSuite) chatRun(conv *model.AgentConversation) *model.AgentRun {
	s.T().Helper()
	loaded := s.load(conv)
	s.Require().NotNil(loaded.IdRun, "the first message creates the chat run")
	run, err := injector.GetAgentRunRepository().LoadById(context.Background(), *loaded.IdRun)
	s.Require().NoError(err)
	return run
}

func (s *AgentConversationDispatchSuite) messages(conv *model.AgentConversation) []*model.AgentConversationMessage {
	s.T().Helper()
	messages, err := injector.GetAgentConversationRepository().LoadMessages(context.Background(), conv.IdConversation)
	s.Require().NoError(err)
	return messages
}

func (s *AgentConversationDispatchSuite) tasks(idRun int64) []*model.AgentTask {
	s.T().Helper()
	tasks, err := injector.GetAgentTaskRepository().LoadByRun(context.Background(), idRun)
	s.Require().NoError(err)
	return tasks
}

func (s *AgentConversationDispatchSuite) tick() {
	s.T().Helper()
	s.Require().NoError(s.Sched.TickOnce(context.Background()))
}

// chatEvent is the stage_execute webhook as the fake gateway receives it.
type chatEvent struct {
	IdRun     int64  `json:"idRun"`
	IdProject int64  `json:"idProject"`
	Event     string `json:"event"`
	Payload   struct {
		IdTask        int64           `json:"idTask"`
		Stage         string          `json:"stage"`
		RunToken      string          `json:"runToken"`
		Model         *string         `json:"model"`
		Effort        *string         `json:"effort"`
		Repo          json.RawMessage `json:"repo"`
		ContextBundle struct {
			Project *struct {
				IdProject int64 `json:"idProject"`
			} `json:"project"`
			ReviewThread     []json.RawMessage `json:"reviewThread"`
			HistoryTruncated bool              `json:"historyTruncated"`
			Chat             struct {
				IdConversation int64  `json:"idConversation"`
				Mode           string `json:"mode"`
				Projects       []struct {
					IdProject int64  `json:"idProject"`
					Name      string `json:"name"`
					IsMain    bool   `json:"isMain"`
				} `json:"projects"`
				HeadCommit       *string `json:"headCommit"`
				DraftFromMessage *struct {
					IdMessage int64  `json:"idMessage"`
					Body      string `json:"body"`
				} `json:"draftFromMessage"`
			} `json:"chat"`
		} `json:"contextBundle"`
	} `json:"payload"`
	raw map[string]any
}

func (e *chatEvent) thread(t *testing.T) []model.AgentConversationMessage {
	out := make([]model.AgentConversationMessage, 0, len(e.Payload.ContextBundle.ReviewThread))
	for _, raw := range e.Payload.ContextBundle.ReviewThread {
		var message model.AgentConversationMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatalf("decoding thread entry: %v", err)
		}
		out = append(out, message)
	}
	return out
}

func bodies(messages []model.AgentConversationMessage) []string {
	out := make([]string, len(messages))
	for i, message := range messages {
		out[i] = message.Body
	}
	return out
}

// nextEvent returns the next stage_execute delivered to the fake gateway.
func (s *AgentConversationDispatchSuite) nextEvent() *chatEvent {
	s.T().Helper()
	select {
	case raw := <-s.Events:
		event := &chatEvent{}
		s.Require().NoError(json.Unmarshal(raw, event))
		s.Require().NoError(json.Unmarshal(raw, &event.raw))
		s.Require().Equal("stage_execute", event.Event)
		return event
	case <-time.After(15 * time.Second):
		s.FailNow("the gateway never received a stage_execute")
		return nil
	}
}

func (s *AgentConversationDispatchSuite) requireNoEvent() {
	s.T().Helper()
	select {
	case raw := <-s.Events:
		s.FailNow("unexpected dispatch: " + string(raw))
	case <-time.After(700 * time.Millisecond):
	}
}

// startTurn posts a message and dispatches the turn it queues.
func (s *AgentConversationDispatchSuite) startTurn(conv *model.AgentConversation, text string) *chatEvent {
	s.T().Helper()
	s.post(conv, text)
	s.tick()
	return s.nextEvent()
}

func (s *AgentConversationDispatchSuite) complete(idTask int64, body, token string) {
	s.T().Helper()
	s.Require().Equal(http.StatusOK, s.status("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", idTask), body, token))
}

// answer returns the newest agent answer of the conversation.
func (s *AgentConversationDispatchSuite) answer(conv *model.AgentConversation) *model.AgentConversationMessage {
	s.T().Helper()
	messages := s.messages(conv)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Author == constants.ConversationAuthorAgent && messages[i].Kind == constants.ConversationMessageText {
			return messages[i]
		}
	}
	s.FailNow("the conversation has no agent answer")
	return nil
}

const chatReplyBody = `{"outcome":"output_submitted","message":"Hello from the agent","messageKind":"chat_reply"}`

func (s *AgentConversationDispatchSuite) requireErr(err error, code string) {
	s.T().Helper()
	var appErr *errs.Error
	s.Require().True(errs.As(err, &appErr), "want %s, got %v", code, err)
	s.Equal(code, appErr.Code)
}

func (s *AgentConversationDispatchSuite) issuesURL(idProject int64) string {
	return fmt.Sprintf("/api/private/project/%d/issue", idProject)
}

func (s *AgentConversationDispatchSuite) TestPostMessage_QueuesTurnAndDispatchesChatStage() {
	conv := s.create(nil)

	message := s.post(conv, "Ahoj agent")
	s.Equal(constants.ConversationAuthorUser, message.Author)
	s.Equal(constants.ConversationMessageText, message.Kind)

	loaded := s.load(conv)
	s.True(loaded.IsLocked)
	s.Equal("Ahoj agent", loaded.Title)
	run := s.chatRun(conv)
	s.Equal(constants.RunKindChat, run.Kind)
	s.Equal(constants.PhaseQueued, run.Phase)
	s.Zero(run.IdProject, "a chat without a project runs without one")

	s.tick()
	event := s.nextEvent()
	s.Equal(run.IdRun, event.IdRun)
	s.Equal(constants.StageChat, event.Payload.Stage)
	s.True(strings.HasPrefix(event.Payload.RunToken, extctx.RunTokenPrefix))
	s.Equal([]string{"Ahoj agent"}, bodies(event.thread(s.T())))
	s.False(event.Payload.ContextBundle.HistoryTruncated)
	s.Equal(conv.IdConversation, event.Payload.ContextBundle.Chat.IdConversation)
	s.Equal(constants.ChatModeReply, event.Payload.ContextBundle.Chat.Mode)
	s.Empty(event.Payload.ContextBundle.Chat.Projects)
	payload := event.raw["payload"].(map[string]any)
	s.NotContains(payload, "repo", "no project, no repository (AC14)")

	s.Equal(constants.PhaseInProgress, s.chatRun(conv).Phase)
}

func (s *AgentConversationDispatchSuite) TestPostMessage_DispatchListsReadableProjects() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "what is in P1?")

	projects := event.Payload.ContextBundle.Chat.Projects
	s.Require().Len(projects, 1)
	s.Equal(s.P1, projects[0].IdProject)
	s.True(projects[0].IsMain)
	s.Equal(s.P1, s.chatRun(conv).IdProject)
}

func (s *AgentConversationDispatchSuite) TestChatTurn_WaitsForActiveIssueStage() {
	ctx := context.Background()
	activeIssue := s.createIssue("chat waits: active")
	queuedIssue := s.createIssue("chat waits: queued")
	_, err := s.App.Pool.Exec(ctx, "UPDATE issues.issue SET assigned_to = $1 WHERE id_issue = ANY($2)",
		s.Agent, []int64{activeIssue.IdIssue, queuedIssue.IdIssue})
	s.Require().NoError(err)

	stagePlan, err := injector.GetAgentStagePlanService().BuildFromChoices(constants.RunKindIssue, model.RunChoices{})
	s.Require().NoError(err)
	var idRunActive, idRunQueued int64
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, kind, phase, stage_plan)
		VALUES ($1, $2, $3, 'issue', 'in_progress', $4) RETURNING id_run`,
		activeIssue.IdIssue, s.Agent, s.P1, stagePlan).Scan(&idRunActive))
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, kind, phase, stage_plan)
		VALUES ($1, $2, $3, 'issue', 'queued', $4) RETURNING id_run`,
		queuedIssue.IdIssue, s.Agent, s.P1, stagePlan).Scan(&idRunQueued))
	designTask, err := injector.GetAgentTaskRepository().Insert(ctx, idRunActive, s.Agent, constants.StageDesign, 1)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.task SET status = 'active', started_at = now() WHERE id_task = $1", designTask.IdTask)
	s.Require().NoError(err)

	conv := s.create(ptr(s.P1))
	s.post(conv, "are you free?")
	s.tick()
	s.requireNoEvent()
	s.Equal(constants.PhaseQueued, s.chatRun(conv).Phase, "the chat waits for the running stage (AC11)")

	s.complete(designTask.IdTask, `{"outcome":"output_submitted","message":"the design","messageKind":"design"}`, s.AgentKey)
	s.tick()
	event := s.nextEvent()
	s.Equal(s.chatRun(conv).IdRun, event.IdRun, "the chat goes ahead of a queued issue run (Decision 4)")
	s.Equal(constants.StageChat, event.Payload.Stage)
	s.Empty(s.tasks(idRunQueued))
}

func (s *AgentConversationDispatchSuite) createIssue(title string) model.Issue {
	res := Request(s.T(), s.App, "POST", s.issuesURL(s.P1),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.requireOk(res)
	var created model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	return created
}

// controllerWith builds the run controller around a recording notifier, so every
// notice a completion sends is visible.
func (s *AgentConversationDispatchSuite) controllerWith(recorder *notifytest.Recorder) *controller.AgentRunController {
	thinking, err := injector.GetAgentThinkingService()
	s.Require().NoError(err)
	turns := agent.NewChatTurns(s.App.Pool, injector.GetAgentConversationRepository(),
		injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(), thinking, recorder)
	return controller.NewAgentRunController(
		injector.GetAgentRunRepository(),
		injector.GetAgentTaskRepository(),
		injector.GetAgentTaskService(),
		thinking,
		injector.GetAgentGatewayRepository(),
		injector.GetIssueRepository(),
		injector.GetProjectRepository(),
		injector.GetMessageRepository(),
		injector.GetAttachmentLinkService(),
		injector.GetGitIntegrationRepository(),
		injector.GetAclService(),
		injector.GetAgentStagePlanService(),
		injector.GetAgentTaskConfigService(),
		injector.GetDispatcher(),
		turns,
		injector.GetWikiProposalService(),
		injector.GetReleaseService(),
		recorder,
		s.App.Pool,
	)
}

func (s *AgentConversationDispatchSuite) completeVia(ctrl *controller.AgentRunController, idTask int64, body string) int {
	agentUser, err := injector.GetUserRepository().LoadUser(context.Background(), s.Agent)
	s.Require().NoError(err)
	engine := gin.New()
	engine.POST("/complete/:idTask", func(c *gin.Context) {
		c.Request = c.Request.WithContext(extctx.WithUser(c.Request.Context(), *agentUser))
		ctrl.CompleteStage(c)
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/complete/%d", idTask), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w.Code
}

func (s *AgentConversationDispatchSuite) TestNotifyTurnStarted_TellsTheOwnerOnlyWhichTurnRuns() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "hello?")
	task, err := injector.GetAgentTaskRepository().LoadById(context.Background(), event.Payload.IdTask)
	s.Require().NoError(err)

	recorder := &notifytest.Recorder{}
	thinking, err := injector.GetAgentThinkingService()
	s.Require().NoError(err)
	turns := agent.NewChatTurns(s.App.Pool, injector.GetAgentConversationRepository(),
		injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(), thinking, recorder)
	turns.NotifyTurnStarted(context.Background(), s.chatRun(conv), task)

	notices := recorder.Notices()
	s.Require().Len(notices, 1)
	s.Equal(notify.SubjectAgentConversationTurn, notices[0].Subject)
	s.Equal(notify.ActionUpdate, notices[0].Action)
	s.Equal([]int64{s.Owner.IdUser}, notices[0].IdsUser, "a turn is private to the conversation owner")
	s.Equal(&model.AgentConversationTurnNotice{IdConversation: conv.IdConversation, IdTask: event.Payload.IdTask}, notices[0].Payload)
}

func (s *AgentConversationDispatchSuite) TestComplete_ChatReplyStoresAgentMessageAndNotifiesOwnerOnly() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "hello?")

	recorder := &notifytest.Recorder{}
	s.Require().Equal(http.StatusOK, s.completeVia(s.controllerWith(recorder), event.Payload.IdTask, chatReplyBody))

	notices := recorder.Notices()
	s.Require().Len(notices, 1, "only the owner hears about a chat reply")
	s.Equal(notify.SubjectAgentConversation, notices[0].Subject)
	s.Equal(notify.ActionCreate, notices[0].Action)
	s.Equal([]int64{s.Owner.IdUser}, notices[0].IdsUser)
	reply, ok := notices[0].Payload.(*model.AgentConversationMessage)
	s.Require().True(ok)
	s.Equal(constants.ConversationAuthorAgent, reply.Author)
	s.Equal(constants.ConversationMessageText, reply.Kind)
	s.Equal("Hello from the agent", reply.Body)
	s.Require().NotNil(reply.IdTask)
	s.Equal(event.Payload.IdTask, *reply.IdTask)
	s.False(reply.HasThinking, "the turn streamed no thinking")

	s.Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase)
	s.NotNil(s.load(conv).LastAgentMessageAt)
	messages := s.messages(conv)
	s.Require().Len(messages, 2)
	s.Equal("Hello from the agent", messages[1].Body)
	s.Equal(http.StatusUnauthorized, s.status("GET", s.issuesURL(s.P1), "", event.Payload.RunToken),
		"the turn's token dies with the turn")
}

// The reply notice tells the page the turn has thinking, so the reply offers it without a reload,
// whether the turn's thinking is stored in full or only as the tail kept at compaction.
func (s *AgentConversationDispatchSuite) TestComplete_ChatReplyNoticeSaysTheTurnHasThinking() {
	defer s.setThinkingPersisted(true)
	for _, isPersisted := range []bool{true, false} {
		s.Run(fmt.Sprintf("persisted=%t", isPersisted), func() {
			s.setThinkingPersisted(isPersisted)
			conv := s.create(ptr(s.P1))
			event := s.startTurn(conv, "hello?")
			thinking, err := injector.GetAgentThinkingService()
			s.Require().NoError(err)
			s.Require().NoError(thinking.Create(context.Background(), event.Payload.IdTask, 1,
				model.AgentThinkingEvents{{Kind: model.ThinkingKindThinking, Text: "reading gc.go", At: 1}}))

			recorder := &notifytest.Recorder{}
			s.Require().Equal(http.StatusOK, s.completeVia(s.controllerWith(recorder), event.Payload.IdTask, chatReplyBody))

			notices := recorder.Notices()
			s.Require().Len(notices, 1)
			reply, ok := notices[0].Payload.(*model.AgentConversationMessage)
			s.Require().True(ok)
			s.Equal("Hello from the agent", reply.Body)
			s.True(reply.HasThinking)
			s.Equal(reply.HasThinking, s.answer(conv).HasThinking, "the notice says what the conversation says")
			s.cleanRuns()
		})
	}
}

func (s *AgentConversationDispatchSuite) setThinkingPersisted(isPersisted bool) {
	s.T().Helper()
	s.Require().Equal(http.StatusOK, s.status("PATCH", "/api/private/admin/settings",
		fmt.Sprintf(`{"isAgentThinkingPersisted":%t}`, isPersisted), s.Token))
}

func (s *AgentConversationDispatchSuite) TestTwoMessagesDuringTurn_NextTurnHasBoth() {
	conv := s.create(ptr(s.P1))
	s.post(conv, "first")
	s.post(conv, "second")
	s.tick()
	first := s.nextEvent()
	s.Equal([]string{"first", "second"}, bodies(first.thread(s.T())), "two queued messages make one turn")
	run := s.chatRun(conv)
	s.Len(s.tasks(run.IdRun), 1)

	s.post(conv, "third")
	s.post(conv, "fourth")
	s.tick()
	s.requireNoEvent()
	s.Len(s.tasks(run.IdRun), 1, "one active turn per conversation")
	s.Equal(constants.PhaseInProgress, s.chatRun(conv).Phase)

	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)
	s.Equal(constants.PhaseQueued, s.chatRun(conv).Phase, "messages sent during the turn queue the next one")

	s.tick()
	second := s.nextEvent()
	// Posting order: the reply arrived after the two messages sent during the turn.
	s.Equal([]string{"first", "second", "third", "fourth", "Hello from the agent"}, bodies(second.thread(s.T())))
}

func (s *AgentConversationDispatchSuite) TestRunToken_PreviousTurnRejectedWhileNextActive() {
	conv := s.create(ptr(s.P1))
	first := s.startTurn(conv, "one")
	s.Require().Equal(http.StatusOK, s.status("GET", s.issuesURL(s.P1), "", first.Payload.RunToken))
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)

	second := s.startTurn(conv, "two")
	s.NotEqual(first.Payload.RunToken, second.Payload.RunToken)
	s.Equal(http.StatusUnauthorized, s.status("GET", s.issuesURL(s.P1), "", first.Payload.RunToken),
		"the previous turn's token never works again")
	s.Equal(http.StatusOK, s.status("GET", s.issuesURL(s.P1), "", second.Payload.RunToken))
}

func (s *AgentConversationDispatchSuite) TestGatewayRecovered_RequeuesChatTurn_UpToThreeAttempts() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "survive a restart")
	run := s.chatRun(conv)

	for attempt := 2; attempt <= 3; attempt++ {
		s.Require().Equal(http.StatusOK, s.status("POST", "/api/private/agent/gateway/recovered", `{}`, s.AgentKey))
		s.Equal(constants.PhaseQueued, s.chatRun(conv).Phase, "a restart re-queues the turn")
		s.Equal(http.StatusUnauthorized, s.status("GET", s.issuesURL(s.P1), "", event.Payload.RunToken))

		s.tick()
		event = s.nextEvent()
		s.Equal(run.IdRun, event.IdRun)
		s.Equal([]string{"survive a restart"}, bodies(event.thread(s.T())), "re-dispatched with the full history")
		s.Len(s.tasks(run.IdRun), attempt)
	}

	s.Require().Equal(http.StatusOK, s.status("POST", "/api/private/agent/gateway/recovered", `{}`, s.AgentKey))
	s.Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase, "the third restart gives up (AC13)")
	messages := s.messages(conv)
	last := messages[len(messages)-1]
	s.Equal(constants.ConversationAuthorSystem, last.Author)
	s.Equal(constants.ConversationMessageError, last.Kind)
	s.Equal(constants.FailReasonGatewayRestart, last.Body)
	s.tick()
	s.requireNoEvent()
	s.Len(s.tasks(run.IdRun), 3)
}

func (s *AgentConversationDispatchSuite) TestStageNotSubmitted_ShowsErrorAndReturnsToAwaitingInput() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "say nothing")

	s.complete(event.Payload.IdTask,
		`{"outcome":"errored","errorReason":"stage_not_submitted","errorDetail":"the agent exited"}`, s.AgentKey)

	s.Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase, "never stuck in in_progress (Review Focus 5)")
	messages := s.messages(conv)
	s.Require().Len(messages, 2)
	s.Equal(constants.ConversationAuthorSystem, messages[1].Author)
	s.Equal(constants.ConversationMessageError, messages[1].Kind)
	s.Equal("stage_not_submitted", messages[1].Body)
	s.Require().NotNil(messages[1].IdTask)
	s.Equal(event.Payload.IdTask, *messages[1].IdTask)

	task, err := injector.GetAgentTaskRepository().LoadById(context.Background(), event.Payload.IdTask)
	s.Require().NoError(err)
	s.Equal(constants.TaskStatusFailed, task.Status)
	s.Equal(http.StatusUnauthorized, s.status("GET", s.issuesURL(s.P1), "", event.Payload.RunToken))
}

func (s *AgentConversationDispatchSuite) TestHistoryOver512KiB_TruncatesOldestAndFlags() {
	conv := s.create(ptr(s.P1))
	s.post(conv, "oldest")
	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO agent.conversation_message (id_conversation, author, id_user, kind, body)
		SELECT $1, 'user', $2, 'text', repeat('x', 16384) FROM generate_series(1, 40)`,
		conv.IdConversation, s.Owner.IdUser)
	s.Require().NoError(err)
	s.post(conv, "newest")
	s.tick()
	event := s.nextEvent()

	s.True(event.Payload.ContextBundle.HistoryTruncated)
	thread := event.thread(s.T())
	s.NotEqual("oldest", thread[0].Body, "the oldest message is dropped first")
	s.Equal("newest", thread[len(thread)-1].Body)
	size := 0
	for _, raw := range event.Payload.ContextBundle.ReviewThread {
		size += len(raw)
	}
	s.LessOrEqual(size, 512*1024)
	s.Greater(len(thread), 20, "the thread keeps as much as fits")
}

func (s *AgentConversationDispatchSuite) TestHugeMessage_TruncatedAndOlderHistoryKept() {
	conv := s.create(ptr(s.P1))
	s.post(conv, "oldest")
	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO agent.conversation_message (id_conversation, author, id_user, kind, body)
		VALUES ($1, 'user', $2, 'text', repeat('y', 600 * 1024))`,
		conv.IdConversation, s.Owner.IdUser)
	s.Require().NoError(err)
	s.post(conv, "newest")
	s.tick()
	event := s.nextEvent()

	thread := event.thread(s.T())
	s.Equal([]string{"oldest", "newest"}, []string{thread[0].Body, thread[len(thread)-1].Body},
		"one huge message does not push the older history out")
	s.Require().Len(thread, 3)
	huge := thread[1].Body
	s.True(strings.HasPrefix(huge, strings.Repeat("y", 1024)), "the start of the huge message is kept")
	s.Contains(huge, "[message truncated")
	s.LessOrEqual(len(huge), constants.ChatMessageMaxBytes+128)
	s.False(event.Payload.ContextBundle.HistoryTruncated, "no message was dropped")
}

func (s *AgentConversationDispatchSuite) TestStatsHeadCommitChange_AddsCodeRefreshedMessage() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "look at the code")
	statsURL := fmt.Sprintf("/api/private/agent/task/%d/stats", event.Payload.IdTask)
	codeRefreshed := func() []*model.AgentConversationMessage {
		var found []*model.AgentConversationMessage
		for _, message := range s.messages(conv) {
			if message.Kind == constants.ConversationMessageCodeRefreshed {
				found = append(found, message)
			}
		}
		return found
	}

	s.Require().Equal(http.StatusOK, s.status("POST", statsURL, `{"headCommit":"aaa111"}`, s.AgentKey))
	s.Empty(codeRefreshed(), "the first worktree is not a refresh")
	s.Equal("aaa111", *s.load(conv).HeadCommit)

	s.Require().Equal(http.StatusOK, s.status("POST", statsURL, `{"headCommit":"bbb222"}`, s.AgentKey))
	s.Require().Equal(http.StatusOK, s.status("POST", statsURL, `{"headCommit":"bbb222"}`, s.AgentKey))
	refreshed := codeRefreshed()
	s.Require().Len(refreshed, 1)
	s.Equal(constants.ConversationAuthorSystem, refreshed[0].Author)
	s.Equal("bbb222", refreshed[0].Body)
	s.Equal("bbb222", *s.load(conv).HeadCommit)
}

func (s *AgentConversationDispatchSuite) TestPostMessage_OwnerLostMainProject_Forbidden() {
	conv := s.create(ptr(s.P3))
	s.Require().Equal(http.StatusOK, s.status("DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", s.P3, s.Owner.IdUser), "", s.Token))
	defer s.addMember(s.P3, s.Owner.IdUser)

	_, err := s.Svc.CreateMessage(context.Background(), s.Owner, conv.IdConversation, "still there?")
	s.requireErr(err, "AGENT_CONVERSATION_NO_ACCESS")
	s.Empty(s.messages(conv))
	s.Nil(s.load(conv).IdRun, "no turn is queued")
}

func (s *AgentConversationDispatchSuite) TestPostMessage_AgentWithoutGateway_Rejected() {
	ctx := context.Background()
	idAgent, _ := s.createAgent("chat dispatch bot without gateway", s.Gateway.URL)
	s.addMember(s.P1, idAgent)
	conv, err := s.Svc.CreateConversation(ctx, s.Owner, model.CreateAgentConversationReq{IdUserAgent: idAgent, IdProject: ptr(s.P1)})
	s.Require().NoError(err)

	_, err = s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", idAgent)
	s.Require().NoError(err)
	_, err = s.Svc.CreateMessage(ctx, s.Owner, conv.IdConversation, "anyone?")
	s.requireErr(err, "AGENT_NO_GATEWAY")

	_, err = s.App.Pool.Exec(ctx, "UPDATE users.user SET is_active = false WHERE id_user = $1", idAgent)
	s.Require().NoError(err)
	_, err = s.Svc.CreateMessage(ctx, s.Owner, conv.IdConversation, "anyone?")
	s.requireErr(err, "NOT_AN_AGENT")

	s.Empty(s.messages(conv))
	s.Nil(s.load(conv).IdRun, "no run is queued (Review Focus 3)")
}

func (s *AgentConversationDispatchSuite) overviewOf(idProject int64, token string) *model.AgentOverview {
	s.T().Helper()
	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/agents/overview", idProject), "", token)
	s.requireOk(res)
	var overview []*model.AgentOverview
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&overview))
	for _, entry := range overview {
		if entry.IdUserAgent == s.Agent {
			return entry
		}
	}
	s.FailNow("the agent is missing from the overview")
	return nil
}

// tokens7d is the agent's counter over all its tasks, so this asserts that a chat turn's tokens
// are counted in it, as seen from the main project's overview, not a per-project attribution.
func (s *AgentConversationDispatchSuite) TestChatTokens_CountedInAgentTokens7d() {
	before := s.overviewOf(s.P1, s.OwnerToken).Tokens7d
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "count my tokens")

	s.Require().Equal(http.StatusOK, s.status("POST",
		fmt.Sprintf("/api/private/agent/task/%d/stats", event.Payload.IdTask), `{"tokensUsed":4321}`, s.AgentKey))

	s.Equal(before+4321, s.overviewOf(s.P1, s.OwnerToken).Tokens7d, "chat tokens count into the agent's tokens7d (AC16)")
}

func (s *AgentConversationDispatchSuite) TestChatRun_HiddenFromProjectMembers() {
	conv := s.create(ptr(s.P1))
	s.startTurn(conv, "private question")
	run := s.chatRun(conv)

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/agent/runs", s.P1), "", s.MemberToken)
	s.requireOk(res)
	var runs []*model.AgentRun
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&runs))
	for _, listed := range runs {
		s.NotEqual(run.IdRun, listed.IdRun, "a chat run is not a project run")
	}
	for _, path := range []string{"", "/stats", "/stage-plan", "/wiki-read"} {
		s.Equal(http.StatusNotFound, s.status("GET", fmt.Sprintf("/api/private/agent/run/%d%s", run.IdRun, path), "", s.MemberToken), path)
	}
	s.Equal(http.StatusNotFound, s.status("POST", fmt.Sprintf("/api/private/agent/run/%d/cancel", run.IdRun), "", s.MemberToken))

	entry := s.overviewOf(s.P1, s.MemberToken)
	s.True(entry.IsBusy)
	s.Nil(entry.Current, "a member sees a busy agent, not the chat")

	recorder := &notifytest.Recorder{}
	agent.BroadcastRunUpdate(context.Background(), recorder, injector.GetProjectRepository(), injector.GetIssueRepository(),
		injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(), run)
	s.Empty(recorder.Notices(), "no run notice for a chat run")
}

func (s *AgentConversationDispatchSuite) TestHeartbeatSweep_ChatTurnEndsWithError() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "are you stuck?")
	run := s.chatRun(conv)
	// What the sweep's FailStaleHeartbeats does to the task before handing the run over.
	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE agent.task SET status = 'failed', finished_at = now(), error_reason = 'heartbeat_stale' WHERE id_task = $1",
		event.Payload.IdTask)
	s.Require().NoError(err)

	agent.FailRunOrChatTurn(context.Background(), injector.GetNotifier(), injector.GetProjectRepository(), injector.GetIssueRepository(),
		injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(), injector.GetChatTurns(), run.IdRun, constants.FailReasonHeartbeatStale)

	s.Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase, "a stale chat turn does not fail the conversation")
	messages := s.messages(conv)
	last := messages[len(messages)-1]
	s.Equal(constants.ConversationMessageError, last.Kind)
	s.Equal(constants.FailReasonHeartbeatStale, last.Body)
	s.Equal(http.StatusUnauthorized, s.status("GET", s.issuesURL(s.P1), "", event.Payload.RunToken))
}

func (s *AgentConversationDispatchSuite) TestRequestTaskDraft_DispatchesTaskDraftMode() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, 0), "VALIDATION_FAILED")

	first := s.startTurn(conv, "let's plan a feature")
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)

	s.Require().NoError(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, s.answer(conv).IdMessage))
	s.tick()
	draft := s.nextEvent()
	s.Equal(constants.ChatModeTaskDraft, draft.Payload.ContextBundle.Chat.Mode)
	s.complete(draft.Payload.IdTask,
		`{"outcome":"output_submitted","message":"# New feature\n\nDo it.","messageKind":"task_draft"}`, draft.Payload.RunToken)
	messages := s.messages(conv)
	s.Equal(constants.ConversationMessageTaskDraft, messages[len(messages)-1].Kind)

	next := s.startTurn(conv, "thanks")
	s.Equal(constants.ChatModeReply, next.Payload.ContextBundle.Chat.Mode, "the draft mode lasts one turn")
}

func (s *AgentConversationDispatchSuite) TestComplete_ChatRejectsIssueMessageKind() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "hi")

	s.Equal(http.StatusBadRequest, s.status("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", event.Payload.IdTask),
		`{"outcome":"output_submitted","message":"x","messageKind":"design"}`, event.Payload.RunToken))
	s.Equal(http.StatusBadRequest, s.status("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", event.Payload.IdTask),
		`{"outcome":"output_submitted","messageKind":"chat_reply"}`, event.Payload.RunToken))
	s.Equal(constants.PhaseInProgress, s.chatRun(conv).Phase)
}

func (s *AgentConversationDispatchSuite) recovered() {
	s.T().Helper()
	s.Require().Equal(http.StatusOK, s.status("POST", "/api/private/agent/gateway/recovered", `{}`, s.AgentKey))
}

func (s *AgentConversationDispatchSuite) TestGatewayRecovered_RestartLimitIsPerTurn() {
	conv := s.create(ptr(s.P1))
	s.startTurn(conv, "first question")
	for range constants.ChatTurnMaxRestarts - 1 {
		s.recovered()
		s.tick()
		s.nextEvent()
	}
	s.recovered()
	s.Require().Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase, "the first turn gave up")

	s.startTurn(conv, "second question")
	s.recovered()
	s.Equal(constants.PhaseQueued, s.chatRun(conv).Phase, "a new turn gets its own restart budget")
	s.tick()
	event := s.nextEvent()
	s.Equal([]string{"first question", constants.FailReasonGatewayRestart, "second question"}, bodies(event.thread(s.T())))
}

func (s *AgentConversationDispatchSuite) TestRequestTaskDraft_DuringActiveReplyTurn_RunsAfterIt() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	first := s.startTurn(conv, "what is the plan")
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)
	idAnswer := s.answer(conv).IdMessage
	reply := s.startTurn(conv, "tell me about the feature")

	s.Require().NoError(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, idAnswer))
	s.complete(reply.Payload.IdTask, chatReplyBody, reply.Payload.RunToken)
	s.Equal(constants.PhaseQueued, s.chatRun(conv).Phase, "the pending draft queues the next turn")

	s.tick()
	draft := s.nextEvent()
	s.Equal(constants.ChatModeTaskDraft, draft.Payload.ContextBundle.Chat.Mode)
	s.complete(draft.Payload.IdTask,
		`{"outcome":"output_submitted","message":"# Feature\n\nDo it.","messageKind":"task_draft"}`, draft.Payload.RunToken)
	s.Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase, "one draft per request")

	next := s.startTurn(conv, "thanks")
	s.Equal(constants.ChatModeReply, next.Payload.ContextBundle.Chat.Mode)
}

func (s *AgentConversationDispatchSuite) TestRequestTaskDraft_WhileReplyTurnQueued_RunsAfterIt() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	first := s.startTurn(conv, "what is the plan")
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)
	idAnswer := s.answer(conv).IdMessage
	s.post(conv, "a question first")
	s.Require().NoError(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, idAnswer))

	s.tick()
	reply := s.nextEvent()
	s.Equal(constants.ChatModeReply, reply.Payload.ContextBundle.Chat.Mode, "the queued turn stays a reply")
	s.complete(reply.Payload.IdTask, chatReplyBody, reply.Payload.RunToken)

	s.tick()
	draft := s.nextEvent()
	s.Equal(constants.ChatModeTaskDraft, draft.Payload.ContextBundle.Chat.Mode)
}

func (s *AgentConversationDispatchSuite) TestRequestTaskDraft_DraftAttemptKeepsModeAfterRestart() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	first := s.startTurn(conv, "plan it")
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)

	s.Require().NoError(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, s.answer(conv).IdMessage))
	s.tick()
	s.Equal(constants.ChatModeTaskDraft, s.nextEvent().Payload.ContextBundle.Chat.Mode)
	s.recovered()
	s.tick()
	s.Equal(constants.ChatModeTaskDraft, s.nextEvent().Payload.ContextBundle.Chat.Mode, "a re-queued attempt is the same turn")
}

func (s *AgentConversationDispatchSuite) TestLateDispatchFailure_AfterRestartRequeue_IsIgnored() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	old := s.startTurn(conv, "hello")
	s.recovered()
	s.tick()
	current := s.nextEvent()

	// The old attempt's dispatch goroutine runs out of retries only now.
	run := s.chatRun(conv)
	oldTask, err := injector.GetAgentTaskRepository().LoadById(ctx, old.Payload.IdTask)
	s.Require().NoError(err)
	_, err = injector.GetChatTurns().Fail(ctx, run, oldTask, "dispatch_failed", "retries exhausted")
	s.Require().NoError(err)

	s.Equal([]string{"hello"}, bodies(derefMessages(s.messages(conv))), "no error message for a superseded attempt")
	s.Equal(constants.PhaseInProgress, s.chatRun(conv).Phase)
	s.Equal(http.StatusOK, s.status("GET", s.issuesURL(s.P1), "", current.Payload.RunToken), "the running attempt keeps its token")
}

func derefMessages(messages []*model.AgentConversationMessage) []model.AgentConversationMessage {
	out := make([]model.AgentConversationMessage, len(messages))
	for i, message := range messages {
		out[i] = *message
	}
	return out
}

func (s *AgentConversationDispatchSuite) TestDispatch_UsesConversationMainProject() {
	ctx := context.Background()
	conv := s.create(nil)
	first := s.startTurn(conv, "no project yet")
	s.Nil(first.Payload.ContextBundle.Project)

	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P1))
	s.recovered()
	s.tick()
	retried := s.nextEvent()

	s.Equal(s.P1, retried.IdProject, "the conversation's main project is the source of truth")
	s.Require().NotNil(retried.Payload.ContextBundle.Project)
	s.Equal(s.P1, retried.Payload.ContextBundle.Project.IdProject)
	projects := retried.Payload.ContextBundle.Chat.Projects
	s.Require().Len(projects, 1)
	s.True(projects[0].IsMain)
}

func (s *AgentConversationDispatchSuite) TestTaskDraft_RequiresAgentMessageOfConversation() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	other := s.create(ptr(s.P1))

	userMessage := s.post(conv, "plan a feature")
	s.tick()
	first := s.nextEvent()
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)
	answer := s.answer(conv)

	otherFirst := s.startTurn(other, "plan something else")
	s.complete(otherFirst.Payload.IdTask, chatReplyBody, otherFirst.Payload.RunToken)
	otherAnswer := s.answer(other)

	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, 0), "VALIDATION_FAILED")
	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, userMessage.IdMessage), "NOT_FOUND")
	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, otherAnswer.IdMessage), "NOT_FOUND")
	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, answer.IdMessage+1000000), "NOT_FOUND")
	s.Nil(s.load(conv).TaskDraftRequest, "a refused request leaves nothing pending")

	s.Require().NoError(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, answer.IdMessage))
	s.tick()
	draft := s.nextEvent()
	s.complete(draft.Payload.IdTask,
		`{"outcome":"output_submitted","message":"# Feature\n\nDo it.","messageKind":"task_draft"}`, draft.Payload.RunToken)

	var card *model.AgentConversationMessage
	for _, message := range s.messages(conv) {
		if message.Kind == constants.ConversationMessageTaskDraft {
			card = message
		}
	}
	s.Require().NotNil(card)
	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, card.IdMessage), "NOT_FOUND")
}

func (s *AgentConversationDispatchSuite) TestTaskDraft_DispatchCarriesDraftFromMessage() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	first := s.startTurn(conv, "plan a feature")
	s.Nil(first.Payload.ContextBundle.Chat.DraftFromMessage, "a reply turn quotes nothing")
	s.complete(first.Payload.IdTask,
		`{"outcome":"output_submitted","message":"First answer\n\n- step one","messageKind":"chat_reply"}`, first.Payload.RunToken)
	second := s.startTurn(conv, "and another thing")
	s.complete(second.Payload.IdTask, chatReplyBody, second.Payload.RunToken)
	messages := s.messages(conv)
	var quoted *model.AgentConversationMessage
	for _, message := range messages {
		if message.Author == constants.ConversationAuthorAgent && message.Body == "First answer\n\n- step one" {
			quoted = message
		}
	}
	s.Require().NotNil(quoted)

	s.Require().NoError(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, quoted.IdMessage))
	s.tick()
	draft := s.nextEvent()
	s.Equal(constants.ChatModeTaskDraft, draft.Payload.ContextBundle.Chat.Mode)
	from := draft.Payload.ContextBundle.Chat.DraftFromMessage
	s.Require().NotNil(from)
	s.Equal(quoted.IdMessage, from.IdMessage)
	s.Equal("First answer\n\n- step one", from.Body)

	s.recovered()
	s.tick()
	retried := s.nextEvent()
	s.Require().NotNil(retried.Payload.ContextBundle.Chat.DraftFromMessage, "a re-queued attempt quotes the same message")
	s.Equal(quoted.IdMessage, retried.Payload.ContextBundle.Chat.DraftFromMessage.IdMessage)

	s.complete(retried.Payload.IdTask,
		`{"outcome":"output_submitted","message":"# Feature\n\nDo it.","messageKind":"task_draft"}`, retried.Payload.RunToken)
	next := s.startTurn(conv, "thanks")
	s.Equal(constants.ChatModeReply, next.Payload.ContextBundle.Chat.Mode)
	s.Nil(next.Payload.ContextBundle.Chat.DraftFromMessage, "the quote lasts one draft")
}

func (s *AgentConversationDispatchSuite) TestReplyMode_AcceptsTaskDraftKind() {
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "please draft a task for the login page")
	s.Equal(constants.ChatModeReply, event.Payload.ContextBundle.Chat.Mode)

	s.complete(event.Payload.IdTask,
		`{"outcome":"output_submitted","message":"# Login page\n\nBuild it.","messageKind":"task_draft"}`, event.Payload.RunToken)

	messages := s.messages(conv)
	last := messages[len(messages)-1]
	s.Equal(constants.ConversationMessageTaskDraft, last.Kind)
	s.Equal(constants.ConversationAuthorAgent, last.Author)
	s.Equal(constants.PhaseAwaitingInput, s.chatRun(conv).Phase, "the turn completes normally")
	s.Nil(s.load(conv).TaskDraftRequest)
}
