package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify/notifytest"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/stretchr/testify/suite"
)

// ChatThinkingSuite pins who sees a chat's thinking: the live stream and the
// replay of each turn reach the conversation owner and nobody else.
//
// Project P1: owner + agent + member.
type ChatThinkingSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	Svc         *service.AgentConversationService
	Owner       *model.User
	OwnerToken  string
	Member      *model.User
	MemberToken string
	Agent       int64
	AgentKey    string
	P1          int64
}

func TestChatThinkingSuite(t *testing.T) {
	suite.Run(t, new(ChatThinkingSuite))
}

func (s *ChatThinkingSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	ctx := context.Background()
	s.Svc = injector.GetAgentConversationService()

	s.OwnerToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat thinking owner","email":"chat-thinking-owner@test.sk","password":"kreslo"}`)
	owner, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-thinking-owner@test.sk")
	s.Require().NoError(err)
	s.Owner = owner
	s.MemberToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat thinking member","email":"chat-thinking-member@test.sk","password":"kreslo"}`)
	member, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-thinking-member@test.sk")
	s.Require().NoError(err)
	s.Member = member

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", `{"name":"chat thinking bot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	s.Require().NotEmpty(created.RawKey, "an agent is created with its API key")
	s.Agent = created.IdUser
	s.AgentKey = created.RawKey
	_, err = s.App.Pool.Exec(ctx,
		`INSERT INTO agent.gateway (id_user_agent, gateway_url, webhook_secret) VALUES ($1, 'http://gateway.test', decode('00','hex'))`,
		s.Agent)
	s.Require().NoError(err)

	s.P1 = createProject(s.T(), s.App, s.Token, "chat-thinking-p1")
	for _, idUser := range []int64{s.Owner.IdUser, s.Agent, s.Member.IdUser} {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", s.P1),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
}

func (s *ChatThinkingSuite) TearDownSuite() {
	s.cleanRuns()
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.P1)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", s.Agent)
}

func (s *ChatThinkingSuite) SetupTest() {
	s.cleanRuns()
}

func (s *ChatThinkingSuite) cleanRuns() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_user_agent = $1", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run_event WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = $1)", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = $1)", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_user_agent = $1", s.Agent)
}

// startChat creates a conversation, posts its first message (which creates the
// chat run) and puts the run in progress, the way the scheduler leaves it.
func (s *ChatThinkingSuite) startChat(idProject *int64) (idConversation, idRun int64) {
	ctx := context.Background()
	conv, err := s.Svc.CreateConversation(ctx, s.Owner, model.CreateAgentConversationReq{IdUserAgent: s.Agent, IdProject: idProject})
	s.Require().NoError(err)
	_, err = s.Svc.CreateMessage(ctx, s.Owner, conv.IdConversation, "what do you think?")
	s.Require().NoError(err)
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Require().NotNil(loaded.IdRun)
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.run SET phase = 'in_progress' WHERE id_run = $1", *loaded.IdRun)
	s.Require().NoError(err)
	return conv.IdConversation, *loaded.IdRun
}

// startTurn inserts an active chat task, as the scheduler activates a turn. Each
// turn of the run is a further attempt of its chat stage.
func (s *ChatThinkingSuite) startTurn(idRun int64) int64 {
	var idTask int64
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.task (id_run, id_user_agent, stage, attempt_no, status, chat_mode, started_at, last_heartbeat_at)
		SELECT $1, $2, 'chat', coalesce(max(attempt_no), 0) + 1, 'active', 'reply', clock_timestamp(), now()
		FROM agent.task WHERE id_run = $1 AND stage = 'chat'
		RETURNING id_task`,
		idRun, s.Agent,
	).Scan(&idTask))
	return idTask
}

func (s *ChatThinkingSuite) postThinking(idTask int64, seq int, text string) {
	s.T().Helper()
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/task/%d/thinking", idTask),
		fmt.Sprintf(`{"seq":%d,"events":[{"kind":"thinking","text":%q,"at":1}]}`, seq, text), s.AgentKey)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
}

func (s *ChatThinkingSuite) completeTurn(idTask int64) {
	s.T().Helper()
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/task/%d/complete", idTask),
		`{"outcome":"output_submitted","message":"my answer","messageKind":"chat_reply"}`, s.AgentKey)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
}

func (s *ChatThinkingSuite) turnURL(idConversation, idTask int64) string {
	return fmt.Sprintf("/api/private/agent/conversation/%d/thinking?idTask=%d", idConversation, idTask)
}

func (s *ChatThinkingSuite) readTurn(idConversation, idTask int64) model.AgentThinkingRes {
	s.T().Helper()
	res := Request(s.T(), s.App, "GET", s.turnURL(idConversation, idTask), "", s.OwnerToken)
	body := readBody(s.T(), res)
	s.Require().Equal(http.StatusOK, res.StatusCode, body)
	var thinking model.AgentThinkingRes
	s.Require().NoError(json.Unmarshal([]byte(body), &thinking))
	return thinking
}

func (s *ChatThinkingSuite) status(url, token string) int {
	s.T().Helper()
	res := Request(s.T(), s.App, "GET", url, "", token)
	readBody(s.T(), res)
	return res.StatusCode
}

func texts(events model.AgentThinkingEvents) []string {
	out := make([]string, len(events))
	for i, event := range events {
		out[i] = event.Text
	}
	return out
}

// recordingThinking is the thinking service with its notices recorded.
func (s *ChatThinkingSuite) recordingThinking() (*agent.ThinkingService, *notifytest.Recorder) {
	settings, err := injector.GetAppSettingsService()
	s.Require().NoError(err)
	recorder := &notifytest.Recorder{}
	return agent.NewThinkingService(
		injector.GetAgentThinkingRepository(),
		injector.GetAgentTaskRepository(),
		injector.GetAgentRunRepository(),
		injector.GetAgentConversationRepository(),
		agent.NewThinkingNotifier(recorder, injector.GetProjectRepository()),
		settings,
	), recorder
}

func (s *ChatThinkingSuite) TestChatThinking_NoticeGoesToOwnerOnly() {
	cases := map[string]*int64{
		"chat in a project": ptr(s.P1),
		"chat without one":  nil,
	}
	for name, idProject := range cases {
		s.Run(name, func() {
			s.cleanRuns()
			idConversation, idRun := s.startChat(idProject)
			idTask := s.startTurn(idRun)
			thinking, recorder := s.recordingThinking()

			s.Require().NoError(thinking.Create(context.Background(), idTask, 1,
				model.AgentThinkingEvents{{Kind: model.ThinkingKindThinking, Text: "private reasoning", At: 1}}))

			notices := recorder.Notices()
			s.Require().Len(notices, 1)
			s.Equal(notify.SubjectAgentThinking, notices[0].Subject)
			s.Equal([]int64{s.Owner.IdUser}, notices[0].IdsUser, "the other members of P1 must not see it")
			payload, ok := notices[0].Payload.(*model.AgentThinkingNotice)
			s.Require().True(ok)
			s.Require().NotNil(payload.IdConversation)
			s.Equal(idConversation, *payload.IdConversation)
			s.Equal(idTask, payload.IdTask)
		})
	}
}

func (s *ChatThinkingSuite) TestChatThinking_RunEndpointHidesChatRun() {
	_, idRun := s.startChat(ptr(s.P1))
	idTask := s.startTurn(idRun)
	s.postThinking(idTask, 1, "private reasoning")

	url := fmt.Sprintf("/api/private/agent/run/%d/thinking?stage=chat", idRun)
	s.Equal(http.StatusNotFound, s.status(url, s.MemberToken), "a member of the main project")
	s.Equal(http.StatusNotFound, s.status(url, s.OwnerToken), "the owner reads it per turn instead")
	s.Equal(http.StatusNotFound, s.status(url, s.Token), "an admin")
}

func (s *ChatThinkingSuite) TestChatThinking_ReplayPerTurn() {
	idConversation, idRun := s.startChat(ptr(s.P1))
	first := s.startTurn(idRun)
	s.postThinking(first, 1, "first turn, step one")
	s.postThinking(first, 2, "first turn, step two")
	s.completeTurn(first)

	_, err := s.App.Pool.Exec(context.Background(), "UPDATE agent.run SET phase = 'in_progress' WHERE id_run = $1", idRun)
	s.Require().NoError(err)
	second := s.startTurn(idRun)
	s.postThinking(second, 1, "second turn, step one")
	s.postThinking(second, 2, "second turn, step two")

	firstRes := s.readTurn(idConversation, first)
	s.True(firstRes.IsComplete)
	s.Equal(first, firstRes.IdTask)
	s.Equal([]string{"first turn, step one", "first turn, step two"}, texts(firstRes.Events))

	secondRes := s.readTurn(idConversation, second)
	s.False(secondRes.IsComplete, "the second turn is still running")
	s.Equal(second, secondRes.IdTask)
	s.Equal(2, secondRes.LastSeq)
	s.Equal([]string{"second turn, step one", "second turn, step two"}, texts(secondRes.Events))
}

func (s *ChatThinkingSuite) TestChatThinking_OtherUserOrForeignTask404() {
	idConversation, idRun := s.startChat(ptr(s.P1))
	idTask := s.startTurn(idRun)
	s.postThinking(idTask, 1, "private reasoning")

	s.Require().Equal(http.StatusOK, s.status(s.turnURL(idConversation, idTask), s.OwnerToken), "the owner reads the turn")
	s.Equal(http.StatusNotFound, s.status(s.turnURL(idConversation, idTask), s.MemberToken), "another user")
	s.Equal(http.StatusNotFound, s.status(s.turnURL(idConversation, idTask), s.Token), "an admin")

	issueTask := s.insertIssueTask()
	s.Equal(http.StatusNotFound, s.status(s.turnURL(idConversation, issueTask), s.OwnerToken),
		"a task of another run is not a turn of this conversation")
	s.Equal(http.StatusNotFound, s.status(s.turnURL(idConversation, 0), s.OwnerToken))
}

func (s *ChatThinkingSuite) insertIssueTask() int64 {
	ctx := context.Background()
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", s.P1),
		`{"title":"chat thinking issue","description":"body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))

	var idRun, idTask int64
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'in_progress', '{"stages":[]}') RETURNING id_run`,
		created.IdIssue, s.Agent, s.P1).Scan(&idRun))
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.task (id_run, id_user_agent, stage, attempt_no, status)
		VALUES ($1, $2, 'design', 1, 'active') RETURNING id_task`,
		idRun, s.Agent).Scan(&idTask))
	return idTask
}

func (s *ChatThinkingSuite) TestChatThinking_PersistOff_ReturnsTail() {
	s.setPersistence(false)
	defer s.setPersistence(true)

	idConversation, idRun := s.startChat(ptr(s.P1))
	idTask := s.startTurn(idRun)
	s.postThinking(idTask, 1, "kept only as the tail")
	s.completeTurn(idTask)

	res := s.readTurn(idConversation, idTask)
	s.False(res.IsComplete, "a tail must not pass as the full thinking")
	s.Equal([]string{"kept only as the tail"}, texts(res.Events))
}

func (s *ChatThinkingSuite) setPersistence(isPersisted bool) {
	res := Request(s.T(), s.App, "PATCH", "/api/private/admin/settings",
		fmt.Sprintf(`{"isAgentThinkingPersisted":%t}`, isPersisted), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *ChatThinkingSuite) TestDeleteConversation_RemovesThinking() {
	ctx := context.Background()
	idConversation, idRun := s.startChat(ptr(s.P1))
	done := s.startTurn(idRun)
	s.postThinking(done, 1, "compacted thinking")
	s.completeTurn(done)
	_, err := s.App.Pool.Exec(ctx, "UPDATE agent.run SET phase = 'in_progress' WHERE id_run = $1", idRun)
	s.Require().NoError(err)
	running := s.startTurn(idRun)
	s.postThinking(running, 1, "live thinking")

	_, err = s.Svc.DeleteConversation(ctx, s.Owner, idConversation)
	s.Require().NoError(err)

	var tasks, rows int
	s.Require().NoError(s.App.Pool.QueryRow(ctx,
		"SELECT count(*) FROM agent.task WHERE id_run = $1", idRun).Scan(&tasks))
	s.Require().NoError(s.App.Pool.QueryRow(ctx,
		"SELECT count(*) FROM agent.task_thinking WHERE id_task = ANY($1)", []int64{done, running}).Scan(&rows))
	s.Zero(tasks, "the turns and their compacted thinking go with the conversation")
	s.Zero(rows, "the live thinking rows go too")
}
