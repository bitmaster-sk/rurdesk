package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/stretchr/testify/suite"
)

// AgentConversationSuite pins the conversation rules: which agent may talk about
// which project, who may see a conversation, and which projects a turn may read.
//
// Projects: P1 owner+A, P2 A+C (owner not in it), P3 owner+A, P4 owner+B,
// P5 owner+A (the owner is removed from it by one test).
type AgentConversationSuite struct {
	suite.Suite
	App     *issue.Application
	Token   string
	Svc     *service.AgentConversationService
	Owner   *model.User
	Other   *model.User
	AgentA  int64
	AgentB  int64
	AgentC  int64
	NoGw    int64
	P1      int64
	P2      int64
	P3      int64
	P4      int64
	P5      int64
	Agents  []int64
	Project []int64
}

func (s *AgentConversationSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	ctx := context.Background()
	s.Svc = injector.GetAgentConversationService()

	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"conversation owner","email":"conv-owner@test.sk","password":"kreslo"}`)
	owner, err := injector.GetUserRepository().LoadUserByEmail(ctx, "conv-owner@test.sk")
	s.Require().NoError(err)
	s.Owner = owner
	other, err := injector.GetUserRepository().LoadUserByEmail(ctx, "test@test.sk")
	s.Require().NoError(err)
	s.Other = other

	s.AgentA = s.createAgent("conv bot a", true)
	s.AgentB = s.createAgent("conv bot b", true)
	s.AgentC = s.createAgent("conv bot c", true)
	s.NoGw = s.createAgent("conv bot no gateway", false)
	s.Agents = []int64{s.AgentA, s.AgentB, s.AgentC, s.NoGw}

	s.P1 = createProject(s.T(), s.App, s.Token, "conv-p1")
	s.P2 = createProject(s.T(), s.App, s.Token, "conv-p2")
	s.P3 = createProject(s.T(), s.App, s.Token, "conv-p3")
	s.P4 = createProject(s.T(), s.App, s.Token, "conv-p4")
	s.P5 = createProject(s.T(), s.App, s.Token, "conv-p5")
	s.Project = []int64{s.P1, s.P2, s.P3, s.P4, s.P5}

	s.addMember(s.P1, s.Owner.IdUser, s.AgentA, s.NoGw)
	s.addMember(s.P2, s.AgentA, s.AgentC)
	s.addMember(s.P3, s.Owner.IdUser, s.AgentA)
	s.addMember(s.P4, s.Owner.IdUser, s.AgentB)
	s.addMember(s.P5, s.Owner.IdUser, s.AgentA)
}

func (s *AgentConversationSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_user_agent = ANY($1)", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.task WHERE id_user_agent = ANY($1)", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_user_agent = ANY($1)", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = ANY($1)", s.Project)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = ANY($1)", append(s.Agents, s.Owner.IdUser))
}

func (s *AgentConversationSuite) createAgent(name string, withGateway bool) int64 {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.Token)
	if res.StatusCode != http.StatusOK {
		s.FailNow(readBody(s.T(), res))
	}
	var agent model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	if withGateway {
		_, err := s.App.Pool.Exec(context.Background(),
			`INSERT INTO agent.gateway (id_user_agent, gateway_url, webhook_secret) VALUES ($1, 'http://gateway.test', decode('00','hex'))`,
			agent.IdUser)
		s.Require().NoError(err)
	}
	return agent.IdUser
}

func (s *AgentConversationSuite) addMember(idProject int64, idsUser ...int64) {
	for _, idUser := range idsUser {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
}

func (s *AgentConversationSuite) create(idAgent int64, idProject *int64) *model.AgentConversation {
	conv, err := s.Svc.CreateConversation(context.Background(), s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: idAgent, IdProject: idProject})
	s.Require().NoError(err)
	return conv
}

// lock sends the first message, which locks the main project.
func (s *AgentConversationSuite) lock(conv *model.AgentConversation) {
	_, err := s.Svc.CreateMessage(context.Background(), s.Owner, conv.IdConversation, "first message")
	s.Require().NoError(err)
	loaded, err := s.Svc.LoadConversation(context.Background(), s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Require().True(loaded.IsLocked)
}

// requireErr asserts the typed error the REST layer turns into the response.
func (s *AgentConversationSuite) requireErr(err error, code string, status int) {
	s.T().Helper()
	var appErr *errs.Error
	s.Require().True(errs.As(err, &appErr), "want %s, got %v", code, err)
	s.Equal(code, appErr.Code)
	s.Equal(status, appErr.HttpStatus())
}

func agentIds(users []*model.AgentConversationAgent) []int64 {
	ids := make([]int64, len(users))
	for i, u := range users {
		ids[i] = u.IdUser
	}
	return ids
}

func (s *AgentConversationSuite) TestCreate_DefaultsToProjectAgent_RejectsAgentOutsideProject() {
	conv := s.create(s.AgentA, ptr(s.P1))
	s.Equal(s.Owner.IdUser, conv.IdUser)
	s.Equal(s.AgentA, conv.IdUserAgent)
	s.Require().NotNil(conv.IdProject)
	s.Equal(s.P1, *conv.IdProject)
	s.False(conv.IsLocked)

	_, err := s.Svc.CreateConversation(context.Background(), s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: s.AgentB, IdProject: ptr(s.P1)})
	s.requireErr(err, "AGENT_NOT_IN_PROJECT", http.StatusUnprocessableEntity)
}

func (s *AgentConversationSuite) TestCreate_RejectsNonAgentAndAgentWithoutGateway() {
	_, err := s.Svc.CreateConversation(context.Background(), s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: s.Other.IdUser, IdProject: ptr(s.P1)})
	s.requireErr(err, "NOT_AN_AGENT", http.StatusUnprocessableEntity)

	_, err = s.Svc.CreateConversation(context.Background(), s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: s.NoGw, IdProject: ptr(s.P1)})
	s.requireErr(err, "AGENT_NO_GATEWAY", http.StatusUnprocessableEntity)
}

func (s *AgentConversationSuite) TestAvailableAgents_WithoutProject_OnlySharedProjects() {
	agents, err := s.Svc.LoadAgents(context.Background(), s.Owner, nil)
	s.Require().NoError(err)
	ids := agentIds(agents)
	s.Contains(ids, s.AgentA)
	s.NotContains(ids, s.AgentC, "C shares no project with the owner")
	s.NotContains(ids, s.NoGw, "an agent without a gateway cannot answer")

	agents, err = s.Svc.LoadAgents(context.Background(), s.Owner, ptr(s.P1))
	s.Require().NoError(err)
	ids = agentIds(agents)
	s.Contains(ids, s.AgentA)
	s.NotContains(ids, s.AgentB, "B is not a member of P1")

	_, err = s.Svc.CreateConversation(context.Background(), s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: s.AgentC})
	s.requireErr(err, "AGENT_NOT_IN_PROJECT", http.StatusUnprocessableEntity)
}

func (s *AgentConversationSuite) TestAttach_RejectsProjectOwnerOrAgentCannotRead() {
	conv := s.create(s.AgentA, ptr(s.P1))
	ctx := context.Background()

	err := s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P2)
	s.requireErr(err, "AGENT_NOT_IN_PROJECT", http.StatusUnprocessableEntity)

	err = s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P4)
	s.requireErr(err, "AGENT_NOT_IN_PROJECT", http.StatusUnprocessableEntity)

	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P3))
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Equal(s.P1, *loaded.IdProject, "a second project is context, not the main one")
	s.Equal([]int64{s.P3}, loaded.IdsProjectContext)
}

func (s *AgentConversationSuite) TestLocked_RejectsMainProjectChange_AllowsFirstProject() {
	ctx := context.Background()

	onP1 := s.create(s.AgentA, ptr(s.P1))
	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, onP1.IdConversation, s.P3))
	s.lock(onP1)
	err := s.Svc.RemoveProject(ctx, s.Owner, onP1.IdConversation, s.P1)
	s.requireErr(err, "AGENT_CONVERSATION_LOCKED", http.StatusConflict)
	s.Require().NoError(s.Svc.RemoveProject(ctx, s.Owner, onP1.IdConversation, s.P3),
		"a context project can always be detached")
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, onP1.IdConversation)
	s.Require().NoError(err)
	s.Equal(s.P1, *loaded.IdProject)
	s.Empty(loaded.IdsProjectContext)

	noProject := s.create(s.AgentA, nil)
	s.lock(noProject)
	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, noProject.IdConversation, s.P1))
	loaded, err = s.Svc.LoadConversation(ctx, s.Owner, noProject.IdConversation)
	s.Require().NoError(err)
	s.Require().NotNil(loaded.IdProject)
	s.Equal(s.P1, *loaded.IdProject)
	s.Empty(loaded.IdsProjectContext)
}

func (s *AgentConversationSuite) TestDetach_MainProjectBeforeFirstMessage() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))

	s.Require().NoError(s.Svc.RemoveProject(ctx, s.Owner, conv.IdConversation, s.P1))
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Nil(loaded.IdProject)
}

// Removing the main project leaves the context projects for the owner to promote; none of them
// is read in the meantime.
func (s *AgentConversationSuite) TestDetachMain_KeepsContextAndReadsNothing() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))
	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P3))

	s.Require().NoError(s.Svc.RemoveProject(ctx, s.Owner, conv.IdConversation, s.P1))
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Nil(loaded.IdProject)
	s.Equal([]int64{s.P3}, loaded.IdsProjectContext)

	readable, err := s.Svc.LoadReadableProjects(ctx, loaded)
	s.Require().NoError(err)
	s.Empty(readable)
}

func (s *AgentConversationSuite) TestAttach_PromotesContextProjectToMain() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))
	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P3))
	s.Require().NoError(s.Svc.RemoveProject(ctx, s.Owner, conv.IdConversation, s.P1))

	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P3))
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Require().NotNil(loaded.IdProject)
	s.Equal(s.P3, *loaded.IdProject)
	s.Empty(loaded.IdsProjectContext)
}

func (s *AgentConversationSuite) TestPostMessage_ContextWithoutMain_Refused() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))
	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P3))
	s.Require().NoError(s.Svc.RemoveProject(ctx, s.Owner, conv.IdConversation, s.P1))

	_, err := s.Svc.CreateMessage(ctx, s.Owner, conv.IdConversation, "hello")
	s.requireErr(err, "AGENT_CONVERSATION_MAIN_REQUIRED", http.StatusUnprocessableEntity)
	s.requireErr(s.Svc.Retry(ctx, s.Owner, conv.IdConversation), "AGENT_CONVERSATION_MAIN_REQUIRED", http.StatusUnprocessableEntity)
	s.requireErr(s.Svc.CreateTaskDraft(ctx, s.Owner, conv.IdConversation, 1), "AGENT_CONVERSATION_MAIN_REQUIRED", http.StatusUnprocessableEntity)

	messages, err := s.Svc.LoadMessages(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Empty(messages, "nothing is stored")

	s.Require().NoError(s.Svc.RemoveProject(ctx, s.Owner, conv.IdConversation, s.P3))
	_, err = s.Svc.CreateMessage(ctx, s.Owner, conv.IdConversation, "hello")
	s.NoError(err, "a chat with neither main nor context projects is a plain chat")
}

func (s *AgentConversationSuite) TestReadableProjects_NoMainProject_ReadsNothing() {
	ctx := context.Background()
	conv := s.create(s.AgentA, nil)
	_, err := s.App.Pool.Exec(ctx, `
		INSERT INTO agent.conversation_project(id_conversation, id_project) VALUES ($1, $2)`,
		conv.IdConversation, s.P3)
	s.Require().NoError(err)
	loaded, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)
	s.Require().Equal([]int64{s.P3}, loaded.IdsProjectContext)

	readable, err := s.Svc.LoadReadableProjects(ctx, loaded)
	s.Require().NoError(err)
	s.Empty(readable)
}

func (s *AgentConversationSuite) TestOtherUsersConversation_NotFound() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))

	_, err := s.Svc.LoadConversation(ctx, s.Other, conv.IdConversation)
	s.requireErr(err, "NOT_FOUND", http.StatusNotFound)
	s.requireErr(s.Svc.AddProject(ctx, s.Other, conv.IdConversation, s.P3), "NOT_FOUND", http.StatusNotFound)
	s.requireErr(s.Svc.RemoveProject(ctx, s.Other, conv.IdConversation, s.P1), "NOT_FOUND", http.StatusNotFound)
	_, err = s.Svc.DeleteConversation(ctx, s.Other, conv.IdConversation)
	s.requireErr(err, "NOT_FOUND", http.StatusNotFound)

	list, err := s.Svc.LoadConversations(ctx, s.Other, nil)
	s.Require().NoError(err)
	for _, c := range list {
		s.NotEqual(conv.IdConversation, c.IdConversation)
	}

	_, err = s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.NoError(err, "the rejected calls must not have touched it")
}

func (s *AgentConversationSuite) TestList_FiltersByMainProject() {
	ctx := context.Background()
	onP1 := s.create(s.AgentA, ptr(s.P1))
	onP3 := s.create(s.AgentA, ptr(s.P3))

	all, err := s.Svc.LoadConversations(ctx, s.Owner, nil)
	s.Require().NoError(err)
	ids := make([]int64, len(all))
	for i, c := range all {
		ids[i] = c.IdConversation
	}
	s.Contains(ids, onP1.IdConversation)
	s.Contains(ids, onP3.IdConversation)

	filtered, err := s.Svc.LoadConversations(ctx, s.Owner, ptr(s.P3))
	s.Require().NoError(err)
	s.NotEmpty(filtered)
	for _, c := range filtered {
		s.Equal(s.P3, *c.IdProject)
	}
}

func (s *AgentConversationSuite) TestReadableProjects_DropsProjectAfterOwnerRemoved() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))
	s.Require().NoError(s.Svc.AddProject(ctx, s.Owner, conv.IdConversation, s.P5))
	conv, err := s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)

	readable, err := s.Svc.LoadReadableProjects(ctx, conv)
	s.Require().NoError(err)
	s.ElementsMatch([]int64{s.P1, s.P5}, readable)

	res := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", s.P5, s.Owner.IdUser), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))

	readable, err = s.Svc.LoadReadableProjects(ctx, conv)
	s.Require().NoError(err)
	s.Equal([]int64{s.P1}, readable)
}

// insertChatTurns gives the conversation a chat run with three turns: one with a
// compacted thinking blob, one with live thinking rows, one without thinking.
func (s *AgentConversationSuite) insertChatTurns(conv *model.AgentConversation) (idRun int64, idsTask []int64) {
	ctx := context.Background()
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run(kind, id_user_agent, id_project, phase, stage_plan)
		VALUES ('chat', $1, $2, 'awaiting_input', '{"stages":[]}')
		RETURNING id_run`, conv.IdUserAgent, conv.IdProject).Scan(&idRun)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx,
		"UPDATE agent.conversation SET id_run = $1 WHERE id_conversation = $2", idRun, conv.IdConversation)
	s.Require().NoError(err)

	for attempt := 1; attempt <= 3; attempt++ {
		var idTask int64
		err = s.App.Pool.QueryRow(ctx, `
			INSERT INTO agent.task(id_run, id_user_agent, stage, attempt_no, status)
			VALUES ($1, $2, 'chat', $3, 'done')
			RETURNING id_task`, idRun, conv.IdUserAgent, attempt).Scan(&idTask)
		s.Require().NoError(err)
		idsTask = append(idsTask, idTask)
	}
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.task SET thinking_blob = '\\x00' WHERE id_task = $1", idsTask[0])
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx, `
		INSERT INTO agent.task_thinking(id_task, seq, event_index, kind, text)
		VALUES ($1, 1, 0, 'thinking', 'hmm')`, idsTask[1])
	s.Require().NoError(err)

	_, err = s.App.Pool.Exec(ctx, `
		INSERT INTO agent.conversation_message(id_conversation, author, id_user, body)
		VALUES ($1, 'user', $2, 'question')`, conv.IdConversation, conv.IdUser)
	s.Require().NoError(err)
	for _, idTask := range idsTask {
		_, err = s.App.Pool.Exec(ctx, `
			INSERT INTO agent.conversation_message(id_conversation, author, body, id_task)
			VALUES ($1, 'agent', 'answer', $2)`, conv.IdConversation, idTask)
		s.Require().NoError(err)
	}
	return idRun, idsTask
}

func (s *AgentConversationSuite) TestLoadMessages_FlagsTurnsWithThinking() {
	conv := s.create(s.AgentA, ptr(s.P1))
	_, idsTask := s.insertChatTurns(conv)

	messages, err := injector.GetAgentConversationRepository().LoadMessages(context.Background(), conv.IdConversation)
	s.Require().NoError(err)
	s.Require().Len(messages, 4)
	s.Equal("user", messages[0].Author)
	s.False(messages[0].HasThinking)
	s.Equal(idsTask[0], *messages[1].IdTask)
	s.True(messages[1].HasThinking, "compacted thinking blob")
	s.True(messages[2].HasThinking, "live thinking rows")
	s.False(messages[3].HasThinking, "a turn without thinking")
}

// The notice of a new message carries the stored message, so a single message tells
// whether its turn has thinking just as the whole conversation does.
func (s *AgentConversationSuite) TestSingleMessage_FlagsTurnWithThinking() {
	ctx := context.Background()
	repo := injector.GetAgentConversationRepository()
	conv := s.create(s.AgentA, ptr(s.P1))
	_, idsTask := s.insertChatTurns(conv)
	messages, err := repo.LoadMessages(ctx, conv.IdConversation)
	s.Require().NoError(err)
	s.Require().Len(messages, 4)

	for i, want := range []bool{false, true, true, false} {
		message, err := repo.LoadMessage(ctx, conv.IdConversation, messages[i].IdMessage)
		s.Require().NoError(err)
		s.Equal(want, message.HasThinking, "message %d", i)
	}

	latest, err := repo.LoadLatestMessage(ctx, conv.IdConversation)
	s.Require().NoError(err)
	s.False(latest.HasThinking, "a turn without thinking")

	inserted, err := repo.InsertMessage(ctx, conv.IdConversation, "agent", "text", "again", nil, &idsTask[1])
	s.Require().NoError(err)
	s.True(inserted.HasThinking, "inserted for a turn with live thinking rows")
	s.Equal(idsTask[1], *inserted.IdTask)
	s.NotZero(inserted.IdMessage)
	s.False(inserted.CreatedAt.IsZero())

	latest, err = repo.LoadLatestMessage(ctx, conv.IdConversation)
	s.Require().NoError(err)
	s.Equal(inserted.IdMessage, latest.IdMessage)
	s.True(latest.HasThinking, "latest of a turn with thinking")

	plain, err := repo.InsertMessage(ctx, conv.IdConversation, "user", "text", "thanks", &conv.IdUser, nil)
	s.Require().NoError(err)
	s.False(plain.HasThinking, "a message without a turn")
}

func (s *AgentConversationSuite) TestDelete_RemovesConversationRunAndTurns() {
	ctx := context.Background()
	conv := s.create(s.AgentA, ptr(s.P1))
	idRun, idsTask := s.insertChatTurns(conv)

	_, err := s.Svc.DeleteConversation(ctx, s.Owner, conv.IdConversation)
	s.Require().NoError(err)

	_, err = s.Svc.LoadConversation(ctx, s.Owner, conv.IdConversation)
	s.requireErr(err, "NOT_FOUND", http.StatusNotFound)
	var runs, tasks, thinking, messages int
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM agent.run WHERE id_run = $1),
		       (SELECT count(*) FROM agent.task WHERE id_task = ANY($2)),
		       (SELECT count(*) FROM agent.task_thinking WHERE id_task = ANY($2)),
		       (SELECT count(*) FROM agent.conversation_message WHERE id_conversation = $3)`,
		idRun, idsTask, conv.IdConversation).Scan(&runs, &tasks, &thinking, &messages))
	s.Zero(runs)
	s.Zero(tasks)
	s.Zero(thinking)
	s.Zero(messages)
}

func TestAgentConversationSuite(t *testing.T) {
	suite.Run(t, new(AgentConversationSuite))
}
