package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// AgentConversationApiSuite drives the conversation REST endpoints as the client
// does: read state, the agent status in the header, delete and task drafts.
//
// Projects: P1 owner+other+agent, P2 other+agent (the owner cannot read it),
// P3 owner+agent.
type AgentConversationApiSuite struct {
	suite.Suite
	App        *issue.Application
	Token      string
	Owner      *model.User
	OwnerToken string
	Other      *model.User
	OtherToken string
	Agent      int64
	AgentKey   string
	Agents     []int64
	P1         int64
	P2         int64
	P3         int64
	Gateway    *httptest.Server
	Events     chan []byte
	Sched      *agent.Scheduler
}

func TestAgentConversationApiSuite(t *testing.T) {
	suite.Run(t, new(AgentConversationApiSuite))
}

func (s *AgentConversationApiSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	ctx := context.Background()

	s.OwnerToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat api owner","email":"chat-api-owner@test.sk","password":"kreslo"}`)
	owner, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-api-owner@test.sk")
	s.Require().NoError(err)
	s.Owner = owner
	s.OtherToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat api other","email":"chat-api-other@test.sk","password":"kreslo"}`)
	other, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-api-other@test.sk")
	s.Require().NoError(err)
	s.Other = other

	s.Events = make(chan []byte, 64)
	s.Gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.Events <- body
		w.WriteHeader(http.StatusOK)
	}))
	s.Agent, s.AgentKey = s.createAgent("chat api bot", s.Gateway.URL)

	s.P1 = createProject(s.T(), s.App, s.Token, "chat-api-p1")
	s.P2 = createProject(s.T(), s.App, s.Token, "chat-api-p2")
	s.P3 = createProject(s.T(), s.App, s.Token, "chat-api-p3")
	s.addMember(s.P1, s.Owner.IdUser, s.Other.IdUser, s.Agent)
	s.addMember(s.P2, s.Other.IdUser, s.Agent)
	s.addMember(s.P3, s.Owner.IdUser, s.Agent)

	s.Sched = agent.NewScheduler(s.App.Pool, injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(), injector.GetIssueRepository(),
		&suiteAgentsDispatcher{agents: &s.Agents, real: injector.GetDispatcher()}, injector.GetRunKinds(), injector.GetNotifier())
}

func (s *AgentConversationApiSuite) TearDownSuite() {
	s.cleanRuns()
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = ANY($1)", []int64{s.P1, s.P2, s.P3})
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = ANY($1)", s.Agents)
	s.Gateway.Close()
}

// SetupTest starts every test with an idle agent, no conversation and no event.
func (s *AgentConversationApiSuite) SetupTest() {
	s.cleanRuns()
	for {
		select {
		case <-s.Events:
		default:
			return
		}
	}
}

func (s *AgentConversationApiSuite) cleanRuns() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_user_agent = ANY($1)", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run_event WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = ANY($1))", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_user_agent = ANY($1))", s.Agents)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_user_agent = ANY($1)", s.Agents)
}

// suiteAgentsDispatcher hands only the suite's own agents to the real dispatcher,
// so leftovers of other suites never reach a gateway.
type suiteAgentsDispatcher struct {
	agents *[]int64
	real   agent.StageDispatcher
}

func (d *suiteAgentsDispatcher) DispatchStage(ctx context.Context, dispatch *agent.Dispatch) {
	if slices.Contains(*d.agents, dispatch.Run.IdUserAgent) {
		d.real.DispatchStage(ctx, dispatch)
	}
}

func (s *AgentConversationApiSuite) createAgent(name, gatewayURL string) (int64, string) {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.Token)
	s.requireStatus(http.StatusOK, res)
	var created model.AdminCreateUserRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	s.Agents = append(s.Agents, created.IdUser)

	gwRes := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", created.IdUser),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gatewayURL), s.Token)
	s.requireStatus(http.StatusOK, gwRes)
	s.Require().NotEmpty(created.RawKey)
	return created.IdUser, created.RawKey
}

func (s *AgentConversationApiSuite) addMember(idProject int64, idsUser ...int64) {
	for _, idUser := range idsUser {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.requireStatus(http.StatusOK, res)
	}
}

func (s *AgentConversationApiSuite) requireStatus(want int, res *http.Response) {
	s.T().Helper()
	if res.StatusCode != want {
		s.FailNow(fmt.Sprintf("want status %d, got %d: %s", want, res.StatusCode, readBody(s.T(), res)))
	}
}

// call sends a request and decodes a JSON response into out (when non-nil).
func (s *AgentConversationApiSuite) call(method, url, body, token string, want int, out any) {
	s.T().Helper()
	res := Request(s.T(), s.App, method, url, body, token)
	s.requireStatus(want, res)
	if out != nil {
		s.Require().NoError(json.NewDecoder(res.Body).Decode(out))
	}
}

// errCode sends a request expecting an error and returns its code.
func (s *AgentConversationApiSuite) errCode(method, url, body, token string, want int) string {
	s.T().Helper()
	var errBody struct {
		Code string `json:"code"`
	}
	s.call(method, url, body, token, want, &errBody)
	return errBody.Code
}

func (s *AgentConversationApiSuite) url(idConversation int64, suffix string) string {
	return fmt.Sprintf("/api/private/agent/conversation/%d%s", idConversation, suffix)
}

func (s *AgentConversationApiSuite) create(idProject *int64, token string) *model.AgentConversation {
	s.T().Helper()
	body := fmt.Sprintf(`{"idUserAgent":%d}`, s.Agent)
	if idProject != nil {
		body = fmt.Sprintf(`{"idUserAgent":%d,"idProject":%d}`, s.Agent, *idProject)
	}
	conv := &model.AgentConversation{}
	s.call("POST", "/api/private/agent/conversation", body, token, http.StatusOK, conv)
	s.Require().NotZero(conv.IdConversation)
	return conv
}

func (s *AgentConversationApiSuite) post(conv *model.AgentConversation, text, token string) *model.AgentConversationMessage {
	s.T().Helper()
	message := &model.AgentConversationMessage{}
	s.call("POST", s.url(conv.IdConversation, "/message"), fmt.Sprintf(`{"message":%q}`, text), token, http.StatusOK, message)
	return message
}

// reply stores an agent message the way a completed turn does.
func (s *AgentConversationApiSuite) reply(conv *model.AgentConversation, text string) {
	s.T().Helper()
	_, err := injector.GetAgentConversationRepository().InsertMessage(context.Background(), conv.IdConversation,
		constants.ConversationAuthorAgent, constants.ConversationMessageText, text, nil, nil)
	s.Require().NoError(err)
}

func (s *AgentConversationApiSuite) unreadCount(token string) int {
	s.T().Helper()
	var res struct {
		Count int `json:"count"`
	}
	s.call("GET", "/api/private/agent/conversation/unread-count", "", token, http.StatusOK, &res)
	return res.Count
}

func (s *AgentConversationApiSuite) list(query string) []*model.AgentConversationListItem {
	s.T().Helper()
	var items []*model.AgentConversationListItem
	s.call("GET", "/api/private/agent/conversation"+query, "", s.OwnerToken, http.StatusOK, &items)
	return items
}

func (s *AgentConversationApiSuite) detail(conv *model.AgentConversation) *model.AgentConversationDetail {
	s.T().Helper()
	detail := &model.AgentConversationDetail{}
	s.call("GET", s.url(conv.IdConversation, ""), "", s.OwnerToken, http.StatusOK, detail)
	return detail
}

func (s *AgentConversationApiSuite) runOf(conv *model.AgentConversation) int64 {
	s.T().Helper()
	loaded, err := injector.GetAgentConversationRepository().LoadConversation(context.Background(), conv.IdConversation)
	s.Require().NoError(err)
	s.Require().NotNil(loaded.IdRun, "the first message creates the chat run")
	return *loaded.IdRun
}

func (s *AgentConversationApiSuite) createIssue(idProject int64, title string) model.Issue {
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.requireStatus(http.StatusOK, res)
	var created model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	return created
}

// startIssueStage makes the agent busy with an issue stage in the project.
func (s *AgentConversationApiSuite) startIssueStage(idProject int64) model.Issue {
	ctx := context.Background()
	created := s.createIssue(idProject, "chat api busy")
	stagePlan, err := injector.GetAgentStagePlanService().BuildFromChoices(constants.RunKindIssue, model.RunChoices{})
	s.Require().NoError(err)
	var idRun int64
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, kind, phase, stage_plan)
		VALUES ($1, $2, $3, 'issue', 'in_progress', $4) RETURNING id_run`,
		created.IdIssue, s.Agent, idProject, stagePlan).Scan(&idRun))
	task, err := injector.GetAgentTaskRepository().Insert(ctx, idRun, s.Agent, constants.StageDesign, 1)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.task SET status = 'active', started_at = now() WHERE id_task = $1", task.IdTask)
	s.Require().NoError(err)
	return created
}

func (s *AgentConversationApiSuite) tick() {
	s.T().Helper()
	s.Require().NoError(s.Sched.TickOnce(context.Background()))
}

// nextEvent returns the next webhook delivered to the fake gateway.
func (s *AgentConversationApiSuite) nextEvent(name string) *chatEvent {
	s.T().Helper()
	select {
	case raw := <-s.Events:
		event := &chatEvent{}
		s.Require().NoError(json.Unmarshal(raw, event))
		s.Require().Equal(name, event.Event, string(raw))
		return event
	case <-time.After(15 * time.Second):
		s.FailNow("the gateway never received " + name)
		return nil
	}
}

func (s *AgentConversationApiSuite) TestUnread_CountsConversationsWithNewAgentReply() {
	first := s.create(ptr(s.P1), s.OwnerToken)
	second := s.create(ptr(s.P3), s.OwnerToken)
	quiet := s.create(nil, s.OwnerToken)
	s.post(quiet, "no answer yet", s.OwnerToken)
	s.Zero(s.unreadCount(s.OwnerToken), "the owner's own message is not unread")

	s.reply(first, "first answer")
	s.reply(first, "second answer")
	s.reply(second, "another answer")
	s.Equal(2, s.unreadCount(s.OwnerToken), "conversations are counted, not replies (AC1)")
	s.Zero(s.unreadCount(s.OtherToken), "another user's count never includes them")

	s.call("POST", s.url(first.IdConversation, "/read"), "", s.OwnerToken, http.StatusNoContent, nil)
	s.Equal(1, s.unreadCount(s.OwnerToken))
	unread := map[int64]bool{}
	for _, item := range s.list("") {
		unread[item.IdConversation] = item.IsUnread
	}
	s.Equal(map[int64]bool{first.IdConversation: false, second.IdConversation: true, quiet.IdConversation: false}, unread)

	s.reply(first, "a later answer")
	s.Equal(2, s.unreadCount(s.OwnerToken), "a reply after reading is unread again")
}

func (s *AgentConversationApiSuite) TestPin_PinnedListFirstThenByActivity() {
	older := s.create(nil, s.OwnerToken)
	pinnedFirst := s.create(nil, s.OwnerToken)
	pinnedSecond := s.create(nil, s.OwnerToken)
	newest := s.create(nil, s.OwnerToken)
	ids := func() []int64 {
		var got []int64
		for _, item := range s.list("") {
			got = append(got, item.IdConversation)
		}
		return got
	}
	s.Equal([]int64{newest.IdConversation, pinnedSecond.IdConversation, pinnedFirst.IdConversation, older.IdConversation}, ids())

	s.call("PUT", s.url(pinnedFirst.IdConversation, "/pin"), `{"isPinned":true}`, s.OwnerToken, http.StatusNoContent, nil)
	s.call("PUT", s.url(pinnedSecond.IdConversation, "/pin"), `{"isPinned":true}`, s.OwnerToken, http.StatusNoContent, nil)
	s.Equal([]int64{pinnedSecond.IdConversation, pinnedFirst.IdConversation, newest.IdConversation, older.IdConversation}, ids(),
		"pinned first, the latest pin on top, the rest by activity")
	pinned := map[int64]bool{}
	for _, item := range s.list("") {
		pinned[item.IdConversation] = item.PinnedAt != nil
	}
	s.Equal(map[int64]bool{pinnedFirst.IdConversation: true, pinnedSecond.IdConversation: true, newest.IdConversation: false, older.IdConversation: false}, pinned)

	s.call("PUT", s.url(pinnedSecond.IdConversation, "/pin"), `{"isPinned":true}`, s.OwnerToken, http.StatusNoContent, nil)
	s.Equal(pinnedSecond.IdConversation, ids()[0], "pinning a pinned conversation again keeps its place")

	s.call("PUT", s.url(pinnedSecond.IdConversation, "/pin"), `{"isPinned":false}`, s.OwnerToken, http.StatusNoContent, nil)
	s.Equal([]int64{pinnedFirst.IdConversation, newest.IdConversation, pinnedSecond.IdConversation, older.IdConversation}, ids(),
		"an unpinned conversation returns to the activity order")
	s.Nil(s.detail(pinnedSecond).Conversation.PinnedAt)
	s.NotNil(s.detail(pinnedFirst).Conversation.PinnedAt)
}

func (s *AgentConversationApiSuite) TestPin_RejectsMissingFlag() {
	conv := s.create(nil, s.OwnerToken)
	s.Equal("VALIDATION_FAILED", s.errCode("PUT", s.url(conv.IdConversation, "/pin"), `{}`, s.OwnerToken, http.StatusBadRequest))
}

func (s *AgentConversationApiSuite) TestList_FilterByProject() {
	onP1 := s.create(ptr(s.P1), s.OwnerToken)
	onP3 := s.create(ptr(s.P3), s.OwnerToken)
	withoutProject := s.create(nil, s.OwnerToken)
	s.create(ptr(s.P1), s.OtherToken)

	s.post(onP1, "count my tokens", s.OwnerToken)
	turn, err := injector.GetAgentTaskRepository().Insert(context.Background(), s.runOf(onP1), s.Agent, constants.StageChat, 1)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(context.Background(), "UPDATE agent.task SET tokens_used = 1234 WHERE id_task = $1", turn.IdTask)
	s.Require().NoError(err)

	ids := func(items []*model.AgentConversationListItem) []int64 {
		out := make([]int64, len(items))
		for i, item := range items {
			out[i] = item.IdConversation
		}
		return out
	}
	s.ElementsMatch([]int64{onP1.IdConversation, onP3.IdConversation, withoutProject.IdConversation}, ids(s.list("")),
		"only the caller's conversations")

	filtered := s.list(fmt.Sprintf("?idProject=%d", s.P1))
	s.Equal([]int64{onP1.IdConversation}, ids(filtered), "the project filter keeps conversations about that project (AC2)")
	s.Equal(1234, filtered[0].TokensUsed)
	s.Equal("count my tokens", filtered[0].Title)

	s.errCode("GET", "/api/private/agent/conversation?idProject=abc", "", s.OwnerToken, http.StatusBadRequest)
}

func (s *AgentConversationApiSuite) TestList_SumsTurnCosts() {
	unpriced := s.create(ptr(s.P1), s.OwnerToken)
	priced := s.create(ptr(s.P3), s.OwnerToken)
	estimated := s.create(nil, s.OwnerToken)

	for _, conv := range []*model.AgentConversation{unpriced, priced, estimated} {
		s.post(conv, "start the run", s.OwnerToken)
	}
	attempts := map[int64]int{}
	addTurn := func(conv *model.AgentConversation, cost *float64, isEstimated bool) {
		attempts[conv.IdConversation]++
		turn, err := injector.GetAgentTaskRepository().Insert(context.Background(), s.runOf(conv), s.Agent, constants.StageChat, attempts[conv.IdConversation])
		s.Require().NoError(err)
		_, err = s.App.Pool.Exec(context.Background(),
			"UPDATE agent.task SET cost_usd = $2, cost_estimated = $3 WHERE id_task = $1", turn.IdTask, cost, isEstimated)
		s.Require().NoError(err)
	}
	addTurn(unpriced, nil, false)
	addTurn(priced, ptr(0.25), false)
	addTurn(priced, ptr(0.5), false)
	addTurn(priced, nil, false)
	addTurn(estimated, ptr(0.125), false)
	addTurn(estimated, ptr(0.25), true)

	costs := map[int64]*model.AgentConversationListItem{}
	for _, item := range s.list("") {
		costs[item.IdConversation] = item
	}
	s.Nil(costs[unpriced.IdConversation].CostUsd, "no task has a cost")
	s.False(costs[unpriced.IdConversation].CostEstimated)
	s.Require().NotNil(costs[priced.IdConversation].CostUsd)
	s.InDelta(0.75, *costs[priced.IdConversation].CostUsd, 1e-9, "a task without a cost adds nothing")
	s.False(costs[priced.IdConversation].CostEstimated)
	s.Require().NotNil(costs[estimated.IdConversation].CostUsd)
	s.InDelta(0.375, *costs[estimated.IdConversation].CostUsd, 1e-9)
	s.True(costs[estimated.IdConversation].CostEstimated, "one estimated task marks the sum estimated")

	detail := s.detail(priced)
	s.Require().NotNil(detail.Stats.TotalCostUsd)
	s.InDelta(0.75, *detail.Stats.TotalCostUsd, 1e-9, "the header reads the same sum from the detail stats")
}

func (s *AgentConversationApiSuite) TestDetail_ListsProjectsAgentAndStats() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.call("POST", s.url(conv.IdConversation, "/project"), fmt.Sprintf(`{"idProject":%d}`, s.P3), s.OwnerToken, http.StatusNoContent, nil)

	detail := s.detail(conv)
	s.Equal(conv.IdConversation, detail.Conversation.IdConversation)
	s.Equal([]model.AgentConversationProject{
		{IdProject: s.P1, Name: "chat-api-p1", IsMain: true},
		{IdProject: s.P3, Name: "chat-api-p3", IsMain: false},
	}, detail.Projects)
	s.Require().NotNil(detail.Agent)
	s.Equal(s.Agent, detail.Agent.IdUser)
	s.Equal(constants.AgentStatusIdle, detail.AgentStatus.State)
	s.Require().NotNil(detail.Stats, "a conversation without a run has empty stats")
	s.Zero(detail.Stats.TotalTokensUsed)

	s.call("DELETE", s.url(conv.IdConversation, fmt.Sprintf("/project/%d", s.P3)), "", s.OwnerToken, http.StatusNoContent, nil)
	s.Len(s.detail(conv).Projects, 1)
}

func (s *AgentConversationApiSuite) TestAgentStatus_HidesIssueOfUnreadableProject() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.startIssueStage(s.P2)

	status := s.detail(conv).AgentStatus
	s.Equal(constants.AgentStatusBusy, status.State)
	s.Nil(status.IdIssuePublic, "the owner cannot read P2 (Decision 6)")
	s.Nil(status.IdProject)
	s.Nil(status.Stage)

	// The project member who can read P2 sees the issue in their own chat.
	otherConv := s.create(ptr(s.P1), s.OtherToken)
	otherDetail := &model.AgentConversationDetail{}
	s.call("GET", s.url(otherConv.IdConversation, ""), "", s.OtherToken, http.StatusOK, otherDetail)
	s.Equal(constants.AgentStatusBusy, otherDetail.AgentStatus.State)
	s.Require().NotNil(otherDetail.AgentStatus.IdProject)
	s.Equal(s.P2, *otherDetail.AgentStatus.IdProject)
	s.NotNil(otherDetail.AgentStatus.IdIssuePublic)
}

func (s *AgentConversationApiSuite) TestAgentStatus_ShowsReadableIssueAndQueuePosition() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	busy := s.startIssueStage(s.P1)

	status := s.detail(conv).AgentStatus
	s.Equal(constants.AgentStatusBusy, status.State)
	s.Require().NotNil(status.IdIssuePublic)
	s.Equal(busy.IdIssuePublic, *status.IdIssuePublic)
	s.Require().NotNil(status.IdProject)
	s.Equal(s.P1, *status.IdProject)
	s.Require().NotNil(status.Stage)
	s.Equal(constants.StageDesign, *status.Stage)
	s.Nil(status.QueuePosition)

	// Another user's chat queued first, then this one: both wait for the stage.
	otherConv := s.create(ptr(s.P1), s.OtherToken)
	s.post(otherConv, "me first", s.OtherToken)
	s.post(conv, "are you free?", s.OwnerToken)
	status = s.detail(conv).AgentStatus
	s.Equal(constants.AgentStatusQueued, status.State)
	s.Require().NotNil(status.QueuePosition)
	s.Equal(2, *status.QueuePosition)
	s.Require().NotNil(status.IdIssuePublic, "the stage it waits for is readable")
	s.Equal(busy.IdIssuePublic, *status.IdIssuePublic)
}

func (s *AgentConversationApiSuite) TestAgentStatus_OtherUsersChatShowsBusyOnly() {
	otherConv := s.create(ptr(s.P1), s.OtherToken)
	s.post(otherConv, "private question", s.OtherToken)
	s.tick()
	s.nextEvent("stage_execute")

	conv := s.create(ptr(s.P1), s.OwnerToken)
	status := s.detail(conv).AgentStatus
	s.Equal(constants.AgentStatusBusy, status.State)
	s.Nil(status.IdIssuePublic, "a chat is private to its owner")
	s.Nil(status.IdProject)
	s.Nil(status.Stage)

	ownDetail := &model.AgentConversationDetail{}
	s.call("GET", s.url(otherConv.IdConversation, ""), "", s.OtherToken, http.StatusOK, ownDetail)
	s.Equal(constants.AgentStatusBusy, ownDetail.AgentStatus.State)
	s.Require().NotNil(ownDetail.AgentStatus.Stage, "the chat's own turn is running")
	s.Equal(constants.StageChat, *ownDetail.AgentStatus.Stage)
	s.Nil(ownDetail.AgentStatus.IdIssuePublic)
}

func (s *AgentConversationApiSuite) TestDelete_SendsRunDiscardedAndRemovesRows() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.post(conv, "start a turn", s.OwnerToken)
	idRun := s.runOf(conv)
	s.tick()
	s.nextEvent("stage_execute")

	s.Equal("NOT_FOUND", s.errCode("DELETE", s.url(conv.IdConversation, ""), "", s.OtherToken, http.StatusNotFound))

	s.call("DELETE", s.url(conv.IdConversation, ""), "", s.OwnerToken, http.StatusNoContent, nil)

	event := s.nextEvent("run_discarded")
	s.Equal(idRun, event.IdRun, "the gateway removes .agent-runs/<idRun> and stops the running turn")
	var conversations, runs, tasks, messages int
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM agent.conversation WHERE id_conversation = $1),
		       (SELECT count(*) FROM agent.run WHERE id_run = $2),
		       (SELECT count(*) FROM agent.task WHERE id_run = $2),
		       (SELECT count(*) FROM agent.conversation_message WHERE id_conversation = $1)`,
		conv.IdConversation, idRun).Scan(&conversations, &runs, &tasks, &messages))
	s.Zero(conversations)
	s.Zero(runs)
	s.Zero(tasks)
	s.Zero(messages)
	s.errCode("GET", s.url(conv.IdConversation, ""), "", s.OwnerToken, http.StatusNotFound)
}

func (s *AgentConversationApiSuite) TestDelete_WithoutRunSendsNothing() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.call("DELETE", s.url(conv.IdConversation, ""), "", s.OwnerToken, http.StatusNoContent, nil)
	select {
	case raw := <-s.Events:
		s.FailNow("unexpected webhook: " + string(raw))
	case <-time.After(500 * time.Millisecond):
	}
}

func (s *AgentConversationApiSuite) TestTaskDraft_SecondRequestWhilePending_Conflicts() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.post(conv, "we need a login page", s.OwnerToken)
	s.tick()
	reply := s.nextEvent("stage_execute")
	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", reply.Payload.IdTask), chatReplyBody, s.AgentKey, http.StatusOK, nil)
	var messages []*model.AgentConversationMessage
	s.call("GET", s.url(conv.IdConversation, "/message"), "", s.OwnerToken, http.StatusOK, &messages)
	body := fmt.Sprintf(`{"idMessage":%d}`, messages[len(messages)-1].IdMessage)
	s.call("POST", s.url(conv.IdConversation, "/task-draft"), body, s.OwnerToken, http.StatusNoContent, nil)

	s.Equal("AGENT_CONVERSATION_DRAFT_PENDING", s.errCode("POST", s.url(conv.IdConversation, "/task-draft"), body, s.OwnerToken, http.StatusConflict))
}

func (s *AgentConversationApiSuite) TestTaskDraft_QueuesTurnWithTaskDraftMode() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.Equal("VALIDATION_FAILED", s.errCode("POST", s.url(conv.IdConversation, "/task-draft"), "{}", s.OwnerToken, http.StatusBadRequest),
		"the answer to draft from is required")
	s.Equal("NOT_FOUND", s.errCode("POST", s.url(conv.IdConversation, "/task-draft"), `{"idMessage":1}`, s.OwnerToken, http.StatusNotFound),
		"nothing to draft from before the first message")

	s.post(conv, "we need a login page", s.OwnerToken)
	s.tick()
	reply := s.nextEvent("stage_execute")
	s.Equal(constants.ChatModeReply, reply.Payload.ContextBundle.Chat.Mode)
	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", reply.Payload.IdTask), chatReplyBody, s.AgentKey, http.StatusOK, nil)

	var messages []*model.AgentConversationMessage
	s.call("GET", s.url(conv.IdConversation, "/message"), "", s.OwnerToken, http.StatusOK, &messages)
	idAnswer := messages[len(messages)-1].IdMessage
	s.call("POST", s.url(conv.IdConversation, "/task-draft"), fmt.Sprintf(`{"idMessage":%d}`, idAnswer), s.OwnerToken, http.StatusNoContent, nil)
	s.tick()
	draft := s.nextEvent("stage_execute")
	s.Equal(constants.ChatModeTaskDraft, draft.Payload.ContextBundle.Chat.Mode, "AC15")
	s.Equal(s.runOf(conv), draft.IdRun, "the draft is a turn of the same chat run, not a new run")
}

func (s *AgentConversationApiSuite) TestMessages_OldestFirst() {
	conv := s.create(nil, s.OwnerToken)
	sent := s.post(conv, "first", s.OwnerToken)
	s.Equal(constants.ConversationAuthorUser, sent.Author)
	s.post(conv, "second", s.OwnerToken)

	var messages []*model.AgentConversationMessage
	s.call("GET", s.url(conv.IdConversation, "/message"), "", s.OwnerToken, http.StatusOK, &messages)
	bodies := make([]string, len(messages))
	for i, message := range messages {
		bodies[i] = message.Body
	}
	s.Equal([]string{"first", "second"}, bodies)

	s.errCode("POST", s.url(conv.IdConversation, "/message"), `{"message":""}`, s.OwnerToken, http.StatusBadRequest)
}

func (s *AgentConversationApiSuite) TestAvailableAgents_StaticRouteNextToId() {
	var agents []*model.User
	s.call("GET", fmt.Sprintf("/api/private/agent/conversation/agents?idProject=%d", s.P1), "", s.OwnerToken, http.StatusOK, &agents)
	idsAgent := make([]int64, len(agents))
	for i, agentUser := range agents {
		idsAgent[i] = agentUser.IdUser
	}
	s.Contains(idsAgent, s.Agent)

	// The owner cannot read P2: 403 here, 422 when creating or attaching.
	s.Equal("AGENT_CONVERSATION_NO_ACCESS",
		s.errCode("GET", fmt.Sprintf("/api/private/agent/conversation/agents?idProject=%d", s.P2), "", s.OwnerToken, http.StatusForbidden))
	s.Equal("AGENT_NOT_IN_PROJECT", s.errCode("POST", "/api/private/agent/conversation",
		fmt.Sprintf(`{"idUserAgent":%d,"idProject":%d}`, s.Agent, s.P2), s.OwnerToken, http.StatusUnprocessableEntity))
}

func (s *AgentConversationApiSuite) TestOtherUser_GetsNotFoundEverywhere() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.post(conv, "mine", s.OwnerToken)
	for _, route := range []struct{ method, suffix, body string }{
		{"GET", "", ""},
		{"GET", "/message", ""},
		{"POST", "/message", `{"message":"theirs"}`},
		{"POST", "/project", fmt.Sprintf(`{"idProject":%d}`, s.P3)},
		{"DELETE", fmt.Sprintf("/project/%d", s.P1), ""},
		{"POST", "/read", ""},
		{"PUT", "/pin", `{"isPinned":true}`},
		{"PUT", "/settings", `{"model":null}`},
		{"POST", "/task-draft", `{"idMessage":1}`},
		{"DELETE", "", ""},
	} {
		s.Equal("NOT_FOUND", s.errCode(route.method, s.url(conv.IdConversation, route.suffix), route.body, s.OtherToken, http.StatusNotFound),
			route.method+" "+route.suffix)
	}
	s.Len(s.detail(conv).Projects, 1, "the rejected calls changed nothing")
}

const chatErroredBody = `{"outcome":"errored","errorReason":"stage_not_submitted","errorDetail":"the agent exited"}`

func (s *AgentConversationApiSuite) TestRetry_RequeuesOnlyAFailedTurn() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	retryURL := s.url(conv.IdConversation, "/retry")
	s.Equal("AGENT_CONVERSATION_NOT_RETRYABLE", s.errCode("POST", retryURL, "", s.OwnerToken, http.StatusConflict),
		"no turn has run yet")

	s.post(conv, "where is the worktree removed?", s.OwnerToken)
	s.Equal("AGENT_CONVERSATION_NOT_RETRYABLE", s.errCode("POST", retryURL, "", s.OwnerToken, http.StatusConflict),
		"the turn is still queued")
	s.tick()
	failed := s.nextEvent("stage_execute")
	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", failed.Payload.IdTask), chatErroredBody, s.AgentKey, http.StatusOK, nil)

	s.Equal("NOT_FOUND", s.errCode("POST", retryURL, "", s.OtherToken, http.StatusNotFound))
	s.call("POST", retryURL, "", s.OwnerToken, http.StatusNoContent, nil)
	s.Equal("AGENT_CONVERSATION_NOT_RETRYABLE", s.errCode("POST", retryURL, "", s.OwnerToken, http.StatusConflict),
		"the retried turn is already queued")

	s.tick()
	retried := s.nextEvent("stage_execute")
	s.Equal(s.runOf(conv), retried.IdRun, "the retry is a turn of the same chat run")
	s.NotEqual(failed.Payload.IdTask, retried.Payload.IdTask)
	s.Equal(constants.ChatModeReply, retried.Payload.ContextBundle.Chat.Mode)

	var messages []*model.AgentConversationMessage
	s.call("GET", s.url(conv.IdConversation, "/message"), "", s.OwnerToken, http.StatusOK, &messages)
	authors := make([]string, len(messages))
	for i, message := range messages {
		authors[i] = message.Author
	}
	s.Equal([]string{constants.ConversationAuthorUser, constants.ConversationAuthorSystem}, authors,
		"a retry stores no message of its own")

	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", retried.Payload.IdTask), chatReplyBody, s.AgentKey, http.StatusOK, nil)
	s.Equal("AGENT_CONVERSATION_NOT_RETRYABLE", s.errCode("POST", retryURL, "", s.OwnerToken, http.StatusConflict),
		"the newest message is the agent's reply")
}

func (s *AgentConversationApiSuite) TestRetry_RechecksAccess() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.post(conv, "start a turn", s.OwnerToken)
	s.tick()
	failed := s.nextEvent("stage_execute")
	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", failed.Payload.IdTask), chatErroredBody, s.AgentKey, http.StatusOK, nil)

	ctx := context.Background()
	_, err := s.App.Pool.Exec(ctx, "UPDATE users.user SET is_active = false WHERE id_user = $1", s.Agent)
	s.Require().NoError(err)
	defer func() {
		_, _ = s.App.Pool.Exec(ctx, "UPDATE users.user SET is_active = true WHERE id_user = $1", s.Agent)
	}()

	s.Equal("NOT_AN_AGENT", s.errCode("POST", s.url(conv.IdConversation, "/retry"), "", s.OwnerToken, http.StatusUnprocessableEntity),
		"a retry is a send and is checked like one")
}

// The running turn's task lets a reloaded page replay its thinking right away.
func (s *AgentConversationApiSuite) TestAgentStatus_NamesTheOwnRunningTurn() {
	conv := s.create(ptr(s.P1), s.OwnerToken)
	s.Nil(s.detail(conv).AgentStatus.IdTask)

	s.post(conv, "start a turn", s.OwnerToken)
	s.Nil(s.detail(conv).AgentStatus.IdTask, "a queued turn has not started")
	s.tick()
	running := s.nextEvent("stage_execute")

	status := s.detail(conv).AgentStatus
	s.Require().NotNil(status.IdTask)
	s.Equal(running.Payload.IdTask, *status.IdTask)

	other := s.create(ptr(s.P1), s.OwnerToken)
	s.Nil(s.detail(other).AgentStatus.IdTask, "another conversation's turn stays anonymous")

	s.call("POST", fmt.Sprintf("/api/private/agent/task/%d/complete", running.Payload.IdTask), chatReplyBody, s.AgentKey, http.StatusOK, nil)
	s.Nil(s.detail(conv).AgentStatus.IdTask)
}
