package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// AgentChatScopeSuite pins the conversation-scoped token: during a chat turn the
// agent reads only the conversation's projects that both it and the owner can
// read, writes nothing but the completion of its own task, and its normal key is
// not affected.
//
// Projects: P1 owner+agent, P2 agent only, P3 owner+agent (the owner is removed
// from it by one test).
type AgentChatScopeSuite struct {
	suite.Suite
	App        *issue.Application
	Token      string
	Owner      *model.User
	Agent      int64
	AgentKey   string
	P1         int64
	P2         int64
	P3         int64
	IssueP1    model.Issue
	IssueP2    model.Issue
	Attachment model.AttachmentRes
	IdPageP1   int64
	IdRunP2    int64
	IdTaskP2   int64
}

func TestAgentChatScopeSuite(t *testing.T) {
	suite.Run(t, new(AgentChatScopeSuite))
}

func (s *AgentChatScopeSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	ctx := context.Background()

	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat scope owner","email":"chat-scope-owner@test.sk","password":"kreslo"}`)
	owner, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-scope-owner@test.sk")
	s.Require().NoError(err)
	s.Owner = owner

	s.Agent, s.AgentKey = s.createAgent("chat scope bot")

	s.P1 = createProject(s.T(), s.App, s.Token, "chat-scope-p1")
	s.P2 = createProject(s.T(), s.App, s.Token, "chat-scope-p2")
	s.P3 = createProject(s.T(), s.App, s.Token, "chat-scope-p3")
	s.addMember(s.P1, s.Owner.IdUser, s.Agent)
	s.addMember(s.P2, s.Agent)
	s.addMember(s.P3, s.Owner.IdUser, s.Agent)

	s.IssueP1 = s.createIssue(s.P1, "chat scope issue p1")
	s.IssueP2 = s.createIssue(s.P2, "chat scope issue p2")
	s.Attachment = uploadAttachmentOk(s.T(), s.App, s.Token, model.IssueRecipientType, s.IssueP1.IdIssue, "scope.png", pngContent)

	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/wiki/page", s.P1),
		`{"space":"project","title":"Chat scope page","body":"scope"}`, s.Token)
	s.requireOk(res)
	var page model.WikiPage
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&page))
	s.IdPageP1 = page.IdPage

	// An ordinary issue run of the same agent in P2, with an active task: the chat
	// token must reach neither the run nor the task.
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, kind, phase, stage_plan)
		VALUES ($1, $2, $3, 'issue', 'in_progress', '[]')
		RETURNING id_run`, s.IssueP2.IdIssue, s.Agent, s.P2).Scan(&s.IdRunP2))
	s.IdTaskP2 = s.activeTask(s.IdRunP2, "brainstorming")
}

func (s *AgentChatScopeSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_user_agent = $1", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run_event WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = $1)", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.task WHERE id_user_agent = $1", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_user_agent = $1", s.Agent)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = ANY($1)", []int64{s.P1, s.P2, s.P3})
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = ANY($1)", []int64{s.Agent, s.Owner.IdUser})
}

func (s *AgentChatScopeSuite) createAgent(name string) (int64, string) {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.Token)
	s.requireOk(res)
	var created model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	_, err := s.App.Pool.Exec(context.Background(),
		`INSERT INTO agent.gateway (id_user_agent, gateway_url, webhook_secret) VALUES ($1, 'http://gateway.test', decode('00','hex'))`,
		created.IdUser)
	s.Require().NoError(err)
	if created.RawKey != "" {
		return created.IdUser, created.RawKey
	}

	keyRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", created.IdUser), `{"name":"chat-scope-key"}`, s.Token)
	s.requireOk(keyRes)
	var key model.CreateApiKeyRes
	s.Require().NoError(json.NewDecoder(keyRes.Body).Decode(&key))
	return created.IdUser, key.RawKey
}

func (s *AgentChatScopeSuite) addMember(idProject int64, idsUser ...int64) {
	for _, idUser := range idsUser {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
}

func (s *AgentChatScopeSuite) createIssue(idProject int64, title string) model.Issue {
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.requireOk(res)
	var created model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	return created
}

func (s *AgentChatScopeSuite) activeTask(idRun int64, stage string) int64 {
	task, err := injector.GetAgentTaskRepository().Insert(context.Background(), idRun, s.Agent, stage, 1)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(context.Background(),
		"UPDATE agent.task SET status = 'active', started_at = now() WHERE id_task = $1", task.IdTask)
	s.Require().NoError(err)
	return task.IdTask
}

// chatTurn is a conversation with a running turn: its chat run, the active task
// and the token the agent got for it.
type chatTurn struct {
	token  string
	idRun  int64
	idTask int64
}

func (s *AgentChatScopeSuite) startTurn(idProject *int64) chatTurn {
	ctx := context.Background()
	conversation, err := injector.GetAgentConversationService().CreateConversation(ctx, s.Owner,
		model.CreateAgentConversationReq{IdUserAgent: s.Agent, IdProject: idProject})
	s.Require().NoError(err)

	var turn chatTurn
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_user_agent, id_project, kind, phase, stage_plan)
		VALUES ($1, $2, 'chat', 'in_progress', '[]')
		RETURNING id_run`, s.Agent, idProject).Scan(&turn.idRun))
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.conversation SET id_run = $2 WHERE id_conversation = $1",
		conversation.IdConversation, turn.idRun)
	s.Require().NoError(err)

	raw, hash, err := agent.MintRunToken()
	s.Require().NoError(err)
	s.Require().NoError(injector.GetAgentRunRepository().SetRunToken(ctx, turn.idRun, hash, time.Now().Add(time.Hour)))
	turn.token = raw
	turn.idTask = s.activeTask(turn.idRun, "chat")
	return turn
}

// requireOk reads the body only on failure, so a passing response stays decodable.
func (s *AgentChatScopeSuite) requireOk(res *http.Response) {
	s.T().Helper()
	if res.StatusCode != http.StatusOK {
		s.FailNow(fmt.Sprintf("status %d: %s", res.StatusCode, readBody(s.T(), res)))
	}
}

func (s *AgentChatScopeSuite) status(method, url, body, token string) int {
	s.T().Helper()
	res := Request(s.T(), s.App, method, url, body, token)
	readBody(s.T(), res)
	return res.StatusCode
}

func (s *AgentChatScopeSuite) projectIssues(idProject int64) string {
	return fmt.Sprintf("/api/private/project/%d/issue", idProject)
}

func (s *AgentChatScopeSuite) TestChatScope_RejectsProjectOutsideConversation() {
	turn := s.startTurn(ptr(s.P1))

	s.Equal(http.StatusForbidden, s.status("GET", s.projectIssues(s.P2), "", turn.token))
	s.Equal(http.StatusOK, s.status("GET", s.projectIssues(s.P1), "", turn.token))

	res := Request(s.T(), s.App, "GET", "/api/private/project", "", turn.token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var projects []model.Project
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&projects))
	s.Require().Len(projects, 1)
	s.Equal(s.P1, projects[0].IdProject)

	res = Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/user-role", s.P1), "", turn.token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var role model.UserRoleRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&role))
	s.Equal(model.RoleViewer, role.Role, "a chat turn reads, whatever the agent's own role")

	res = Request(s.T(), s.App, "GET", "/api/private/state", "", turn.token)
	s.requireOk(res)
	var states []model.State
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&states))
	s.Require().NotEmpty(states)
	for _, state := range states {
		s.Equal(s.P1, state.IdProject, "meta lists cover only the conversation's projects")
	}
	s.Equal(http.StatusOK, s.status("GET", "/api/private/user", "", turn.token))

	// The same agent with its normal key keeps its full access.
	s.Equal(http.StatusOK, s.status("GET", s.projectIssues(s.P2), "", s.AgentKey))
	res = Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/user-role", s.P1), "", s.AgentKey)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&role))
	s.Equal(model.RoleMember, role.Role)
}

func (s *AgentChatScopeSuite) TestChatScope_NoProject_RejectsEveryProject() {
	turn := s.startTurn(nil)

	res := Request(s.T(), s.App, "GET", "/api/private/project", "", turn.token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var projects []model.Project
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&projects))
	s.Empty(projects)

	s.Equal(http.StatusForbidden, s.status("GET", s.projectIssues(s.P1), "", turn.token))
}

func (s *AgentChatScopeSuite) TestChatScope_RejectsWrites() {
	turn := s.startTurn(ptr(s.P1))
	issueURL := fmt.Sprintf("%s/%d", s.projectIssues(s.P1), s.IssueP1.IdIssuePublic)

	s.Equal(http.StatusForbidden, s.status("POST", s.projectIssues(s.P1),
		`{"title":"written by a chat","description":"x","estimated":0}`, turn.token))
	s.Equal(http.StatusForbidden, s.status("PATCH", issueURL, `{"title":"renamed by a chat"}`, turn.token))
	s.Equal(http.StatusForbidden, s.status("POST", "/api/private/message",
		fmt.Sprintf(`{"body":"from a chat","idRecipient":%d,"idMessageRecipientType":4}`, s.IssueP1.IdIssue), turn.token))
	s.Equal(http.StatusForbidden, s.status("POST",
		fmt.Sprintf("/api/private/agent/task/%d/complete", s.IdTaskP2), `{"outcome":"errored","errorReason":"x"}`, turn.token),
		"a run token completes only its own task")

	s.Equal(http.StatusOK, s.status("POST",
		fmt.Sprintf("/api/private/agent/task/%d/complete", turn.idTask),
		`{"outcome":"output_submitted","message":"a read-only answer","messageKind":"chat_reply"}`, turn.token))

	// The only write a run token makes is its own reply.
	ctx := context.Background()
	conversation, err := injector.GetAgentConversationRepository().LoadConversationByRun(ctx, turn.idRun)
	s.Require().NoError(err)
	messages, err := injector.GetAgentConversationRepository().LoadMessages(ctx, conversation.IdConversation)
	s.Require().NoError(err)
	s.Require().Len(messages, 1)
	s.Equal("agent", messages[0].Author)
	s.Equal("a read-only answer", messages[0].Body)
	run, err := injector.GetAgentRunRepository().LoadById(ctx, turn.idRun)
	s.Require().NoError(err)
	s.Equal("awaiting_input", run.Phase)
}

func (s *AgentChatScopeSuite) TestChatScope_OwnerRemovedFromProject() {
	turn := s.startTurn(ptr(s.P3))
	s.Require().Equal(http.StatusOK, s.status("GET", s.projectIssues(s.P3), "", turn.token))

	s.Require().Equal(http.StatusOK, s.status("DELETE",
		fmt.Sprintf("/api/private/project/%d/member/user/%d", s.P3, s.Owner.IdUser), "", s.Token))

	s.Equal(http.StatusForbidden, s.status("GET", s.projectIssues(s.P3), "", turn.token))
	s.Equal(http.StatusOK, s.status("GET", s.projectIssues(s.P3), "", s.AgentKey))
}

func (s *AgentChatScopeSuite) TestRunToken_InvalidAfterTaskCompleted() {
	turn := s.startTurn(ptr(s.P1))
	s.Require().Equal(http.StatusOK, s.status("GET", s.projectIssues(s.P1), "", turn.token))

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE agent.task SET status = 'completed', finished_at = now() WHERE id_task = $1", turn.idTask)
	s.Require().NoError(err)

	s.Equal(http.StatusUnauthorized, s.status("GET", s.projectIssues(s.P1), "", turn.token))
}

func (s *AgentChatScopeSuite) TestRunToken_InvalidAfterExpiry() {
	turn := s.startTurn(ptr(s.P1))
	s.Require().Equal(http.StatusOK, s.status("GET", s.projectIssues(s.P1), "", turn.token))

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE agent.run SET run_token_expires_at = now() WHERE id_run = $1", turn.idRun)
	s.Require().NoError(err)

	s.Equal(http.StatusUnauthorized, s.status("GET", s.projectIssues(s.P1), "", turn.token))
	s.Equal(http.StatusUnauthorized, s.status("GET", s.projectIssues(s.P1), "", "rdr_"+fmt.Sprintf("%064d", 0)))
}

// Routes outside a project carry the agent's data from every project (its
// workload, notifications, direct messages, other runs), so a chat turn may not
// read them at all; the normal key still can.
func (s *AgentChatScopeSuite) TestChatScope_RejectsReadsOutsideProjectRoutes() {
	turn := s.startTurn(ptr(s.P1))
	routes := []string{
		"/api/private/my-issues",
		"/api/private/notification",
		fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=1", s.Owner.IdUser),
		"/api/private/message/unread",
		fmt.Sprintf("/api/private/agent/run/%d", s.IdRunP2),
		fmt.Sprintf("/api/private/attachment/%s", s.Attachment.IdAttachment),
		fmt.Sprintf("/api/private/wiki/page/%d/version", s.IdPageP1),
	}
	for _, route := range routes {
		s.Equal(http.StatusForbidden, s.status("GET", route, "", turn.token), route)
		s.Equal(http.StatusOK, s.status("GET", route, "", s.AgentKey), route)
	}
	s.Equal(http.StatusForbidden, s.status("GET", "/api/private/ws", "", turn.token))

	// No code: the repository and its merge requests stay out of a chat turn.
	gitIntegrations := fmt.Sprintf("/api/private/project/%d/git-integration", s.P1)
	s.Equal(http.StatusForbidden, s.status("GET", gitIntegrations, "", turn.token))
	s.Equal(http.StatusForbidden, s.status("GET", gitIntegrations+"/1/mr/1/file-content?path=main.go&ref=main", "", turn.token))
	s.Equal(http.StatusOK, s.status("GET", gitIntegrations, "", s.AgentKey))
	s.Equal(http.StatusForbidden, s.status("GET", "/api/private/admin/user", "", turn.token))
}

// Issue and project comments are task content the turn reads, within its scope.
func (s *AgentChatScopeSuite) TestChatScope_ReadsMessagesOfScopedProjectsOnly() {
	issueMessages := func(idIssue int64) string {
		return fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=4", idIssue)
	}
	turn := s.startTurn(ptr(s.P1))

	s.Equal(http.StatusOK, s.status("GET", issueMessages(s.IssueP1.IdIssue), "", turn.token))
	s.Equal(http.StatusOK, s.status("GET",
		fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=3", s.P1), "", turn.token))
	s.Equal(http.StatusForbidden, s.status("GET", issueMessages(s.IssueP2.IdIssue), "", turn.token))
	s.Equal(http.StatusOK, s.status("GET", issueMessages(s.IssueP2.IdIssue), "", s.AgentKey))

	noProject := s.startTurn(nil)
	s.Equal(http.StatusForbidden, s.status("GET", issueMessages(s.IssueP1.IdIssue), "", noProject.token))
}

// A run token resolves to exactly one conversation: a run cannot be shared.
func (s *AgentChatScopeSuite) TestRunToken_RunBelongsToOneConversation() {
	turn := s.startTurn(ptr(s.P1))
	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO agent.conversation (id_user, id_user_agent, id_project, id_run)
		VALUES ($1, $2, $3, $4)`, s.Owner.IdUser, s.Agent, s.P2, turn.idRun)
	s.Require().Error(err)
	s.Contains(err.Error(), "conversation_id_run_uidx")
}
