package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// AgentTaskCostSuite covers the cost/token-breakdown reporting path: the
// gateway posts usage via POST /agent/task/:id/stats and the run stats
// endpoint aggregates it, with a NULL total when no task reported a price.
type AgentTaskCostSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	AgentApiKey   string
	AgentUserID   int64
	IdProject     int64
	IdIssuePublic int64
}

func (s *AgentTaskCostSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"costbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64  `json:"idUser"`
		RawKey string `json:"rawKey"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	s.AgentUserID = agent.IdUser
	s.AgentApiKey = agent.RawKey

	s.IdProject = createProject(s.T(), s.App, s.Token, "task-cost-project")
	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.AgentUserID), s.Token)

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"cost test issue","description":"cost test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *AgentTaskCostSuite) insertRun(taskCount int) (int64, []int64) {
	_, err := s.App.Pool.Exec(context.Background(), `DELETE FROM agent.task; DELETE FROM agent.run`)
	s.Require().NoError(err)

	var idRun int64
	err = s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_agent, id_project, phase, stage_plan)
		SELECT id_issue, $1, $2, 'in_progress', '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.AgentUserID, s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)

	idsTask := make([]int64, 0, taskCount)
	for i := 0; i < taskCount; i++ {
		var idTask int64
		err = s.App.Pool.QueryRow(context.Background(), `
			INSERT INTO agent.task(id_run, id_user_agent, stage, attempt_no, status)
			VALUES ($1, $2, $3, $4, 'active')
			RETURNING id_task`,
			idRun, s.AgentUserID, fmt.Sprintf("stage_%d", i), i+1,
		).Scan(&idTask)
		s.Require().NoError(err)
		idsTask = append(idsTask, idTask)
	}
	return idRun, idsTask
}

func (s *AgentTaskCostSuite) statsForRun(idRun int64) model.RunStatsRes {
	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d/stats", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var stats model.RunStatsRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&stats))
	return stats
}

// A stats post with cost and cache tokens is aggregated into the run totals:
// token sums plus totalCostUsd and costEstimated.
func (s *AgentTaskCostSuite) Test_StatsPostWithCostAggregates() {
	idRun, idsTask := s.insertRun(1)

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/stats", idsTask[0]),
		`{"tokensUsed":30,"durationMs":1200,"toolCallsCount":2,`+
			`"inputTokens":12,"outputTokens":3456,"cacheReadTokens":150000,"cacheWriteTokens":20000,`+
			`"costUsd":0.4213,"costEstimated":true}`,
		s.AgentApiKey)
	s.Equal(http.StatusOK, res.StatusCode)

	stats := s.statsForRun(idRun)
	s.Equal(30, stats.TotalTokensUsed)
	s.Equal(12, stats.TotalInputTokens)
	s.Equal(3456, stats.TotalOutputTokens)
	s.Equal(150000, stats.TotalCacheReadTokens)
	s.Equal(20000, stats.TotalCacheWriteTokens)
	s.Require().NotNil(stats.TotalCostUsd)
	s.InDelta(0.4213, *stats.TotalCostUsd, 1e-6)
	s.True(stats.CostEstimated)
}

// Two tasks, one with a price and one without: the total is the sum over the
// priced ones only, and costEstimated comes from the priced one.
func (s *AgentTaskCostSuite) Test_RunSumsCostOnlyFromPricedTasks() {
	idRun, idsTask := s.insertRun(2)

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/stats", idsTask[0]),
		`{"inputTokens":10,"outputTokens":20,"costUsd":0.10,"costEstimated":true}`, s.AgentApiKey)
	s.Equal(http.StatusOK, res.StatusCode)
	res = Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/stats", idsTask[1]),
		`{"inputTokens":5,"outputTokens":6}`, s.AgentApiKey)
	s.Equal(http.StatusOK, res.StatusCode)

	stats := s.statsForRun(idRun)
	s.Equal(15, stats.TotalInputTokens)
	s.Equal(26, stats.TotalOutputTokens)
	s.Require().NotNil(stats.TotalCostUsd)
	s.InDelta(0.10, *stats.TotalCostUsd, 1e-6)
	s.True(stats.CostEstimated)
}

// No task reported a price: totalCostUsd stays JSON null, never 0.
func (s *AgentTaskCostSuite) Test_RunWithoutAnyCostReturnsNull() {
	idRun, idsTask := s.insertRun(1)

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/stats", idsTask[0]),
		`{"tokensUsed":42,"inputTokens":30,"outputTokens":12}`, s.AgentApiKey)
	s.Equal(http.StatusOK, res.StatusCode)

	raw := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d/stats", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, raw.StatusCode)
	var body map[string]any
	s.Require().NoError(json.NewDecoder(raw.Body).Decode(&body))
	if v, ok := body["totalCostUsd"]; !ok || v != nil {
		s.T().Errorf("totalCostUsd = %v (present=%v), want explicit null", v, ok)
	}
	if body["costEstimated"] != false {
		s.T().Errorf("costEstimated = %v, want false", body["costEstimated"])
	}

	stats := s.statsForRun(idRun)
	s.Nil(stats.TotalCostUsd)
	s.False(stats.CostEstimated)
	s.Equal(42, stats.TotalTokensUsed)
	s.Equal(30, stats.TotalInputTokens)
	s.Equal(12, stats.TotalOutputTokens)
}

func TestAgentTaskCostSuite(t *testing.T) {
	suite.Run(t, new(AgentTaskCostSuite))
}
