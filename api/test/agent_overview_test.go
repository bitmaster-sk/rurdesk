package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AgentOverviewSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	BusyAgentID int64
	IdleAgentID int64
	IdProjectA  int64
	IdProjectB  int64
	IssueA      model.Issue
	IssueB      model.Issue
}

func emptyStagePlan(t *testing.T) json.RawMessage {
	t.Helper()
	plan, err := injector.GetStagePlanService().Build(nil)
	require.NoError(t, err)
	return plan
}

func (s *AgentOverviewSuite) createAgent(name string) int64 {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	return agent.IdUser
}

func (s *AgentOverviewSuite) createIssueIn(idProject int64, title string) model.Issue {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))
	return iss
}

func (s *AgentOverviewSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	ctx := context.Background()

	s.BusyAgentID = s.createAgent("overviewbusybot")
	s.IdleAgentID = s.createAgent("overviewidlebot")

	s.IdProjectA = createProject(s.T(), s.App, s.Token, "overview-project-a")
	s.IdProjectB = createProject(s.T(), s.App, s.Token, "overview-project-b")
	for _, idProject := range []int64{s.IdProjectA, s.IdProjectB} {
		for _, idAgent := range []int64{s.BusyAgentID, s.IdleAgentID} {
			member := Request(s.T(), s.App, "POST",
				fmt.Sprintf("/api/private/project/%d/member/user", idProject),
				fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idAgent), s.Token)
			s.Require().Equal(http.StatusOK, member.StatusCode)
		}
	}
	s.IssueA = s.createIssueIn(s.IdProjectA, "overview-a")
	s.IssueB = s.createIssueIn(s.IdProjectB, "overview-b")

	runA, err := injector.GetAgentRunRepository().Insert(ctx, s.IssueA.IdIssue, s.BusyAgentID, s.IdProjectA, emptyStagePlan(s.T()))
	s.Require().NoError(err)
	_, err = injector.GetAgentTaskRepository().Insert(ctx, runA.IdRun, s.BusyAgentID, "design", 1)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.task SET status = 'active' WHERE id_run = $1", runA.IdRun)
	s.Require().NoError(err)
	// A run under an active task is no longer queued; the scheduler does this too.
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.run SET phase = 'in_progress' WHERE id_run = $1", runA.IdRun)
	s.Require().NoError(err)

	_, err = injector.GetAgentRunRepository().Insert(ctx, s.IssueB.IdIssue, s.BusyAgentID, s.IdProjectB, emptyStagePlan(s.T()))
	s.Require().NoError(err)
}

func (s *AgentOverviewSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.task WHERE id_user_agent IN ($1, $2)", s.BusyAgentID, s.IdleAgentID)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_user_agent IN ($1, $2)", s.BusyAgentID, s.IdleAgentID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project IN ($1, $2)", s.IdProjectA, s.IdProjectB)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user IN ($1, $2)", s.BusyAgentID, s.IdleAgentID)
}

func (s *AgentOverviewSuite) overview(idProject int64, token string) ([]model.AgentOverview, int) {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/agents/overview", idProject), "", token)
	if res.StatusCode != http.StatusOK {
		return nil, res.StatusCode
	}
	var all []model.AgentOverview
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&all))
	return all, res.StatusCode
}

func (s *AgentOverviewSuite) agentRow(all []model.AgentOverview, idAgent int64) model.AgentOverview {
	for _, row := range all {
		if row.IdUserAgent == idAgent {
			return row
		}
	}
	s.Failf("bot missing", "bot %d not in overview", idAgent)
	return model.AgentOverview{}
}

func (s *AgentOverviewSuite) TestBusyAgentShowsCurrentRunOfThisProject() {
	all, _ := s.overview(s.IdProjectA, s.Token)
	busy := s.agentRow(all, s.BusyAgentID)

	s.True(busy.IsBusy)
	s.Require().NotNil(busy.Current)
	s.Equal(s.IssueA.IdIssuePublic, busy.Current.IdIssuePublic)
	s.Equal("design", busy.Current.Stage)
	s.Equal(1, busy.QueueCount, "queue count is global")
	s.Empty(busy.QueuedIdsIssuePublic, "the queued run belongs to project B, so its issue is not named here")
}

func (s *AgentOverviewSuite) TestOtherProjectSeesQueueButNotForeignCurrent() {
	all, _ := s.overview(s.IdProjectB, s.Token)
	busy := s.agentRow(all, s.BusyAgentID)

	s.True(busy.IsBusy, "busy is a property of the bot, not of the project")
	s.Nil(busy.Current, "the active run belongs to project A — its issue must not leak")
	s.Equal([]int64{s.IssueB.IdIssuePublic}, busy.QueuedIdsIssuePublic)
}

func (s *AgentOverviewSuite) TestIdleAgentIsIdle() {
	all, _ := s.overview(s.IdProjectA, s.Token)
	idle := s.agentRow(all, s.IdleAgentID)

	s.False(idle.IsBusy)
	s.Nil(idle.Current)
	s.Zero(idle.QueueCount)
	s.Nil(idle.AvgRunDurationMs7d, "no finished run in the window")
}

func (s *AgentOverviewSuite) TestOnlyAgentsOfThisProjectAreListed() {
	idOtherProject := createProject(s.T(), s.App, s.Token, "overview-project-c")
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", idOtherProject)

	all, _ := s.overview(idOtherProject, s.Token)

	for _, row := range all {
		s.NotEqual(s.BusyAgentID, row.IdUserAgent, "a bot that is not a member here must not be listed")
		s.NotEqual(s.IdleAgentID, row.IdUserAgent)
	}
}

func (s *AgentOverviewSuite) TestNonMemberForbidden() {
	outsider := createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"overview-outsider","email":"overview-outsider@test.sk","password":"kreslo1"}`)

	_, status := s.overview(s.IdProjectA, outsider)
	s.Equal(http.StatusForbidden, status)
}

func Test_AgentOverviewSuite(t *testing.T) {
	suite.Run(t, new(AgentOverviewSuite))
}
