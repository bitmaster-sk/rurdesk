package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type dispatchedStage struct {
	Model    *string `json:"model"`
	MaxTurns int     `json:"maxTurns"`
}

type ModelSelectionSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	AgentUserID int64
	AgentApiKey string
	IdProject   int64
	Gateway     *httptest.Server

	mu         sync.Mutex
	dispatched map[int64]chan dispatchedStage
}

func (s *ModelSelectionSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	s.dispatched = map[int64]chan dispatchedStage{}

	s.Gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var event struct {
			Event   string `json:"event"`
			Payload struct {
				IdTask int64 `json:"idTask"`
				dispatchedStage
			} `json:"payload"`
		}
		if json.Unmarshal(body, &event) == nil && event.Event == "stage_execute" {
			s.stageChannel(event.Payload.IdTask) <- event.Payload.dispatchedStage
		}
		w.WriteHeader(http.StatusOK)
	}))

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"modelbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64  `json:"idUser"`
		RawKey string `json:"rawKey"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	s.AgentUserID = agent.IdUser
	s.AgentApiKey = agent.RawKey

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		fmt.Sprintf(`{"gatewayUrl":%q}`, s.Gateway.URL), s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	s.IdProject = createProject(s.T(), s.App, s.Token, "model-selection-project")
	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.AgentUserID), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode)
}

func (s *ModelSelectionSuite) TearDownSuite() {
	ctx := context.Background()
	s.Gateway.Close()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", s.AgentUserID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", s.AgentUserID)
}

func (s *ModelSelectionSuite) SetupTest() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.reportModels(`{"models":["openai/kimi","ollama/qwen3:8b"],"modelDefault":"openai/kimi"}`)
	s.setAgentTurns(`null`, `null`)
}

func (s *ModelSelectionSuite) stageChannel(idTask int64) chan dispatchedStage {
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.dispatched[idTask]
	if !ok {
		channel = make(chan dispatchedStage, 1)
		s.dispatched[idTask] = channel
	}
	return channel
}

func (s *ModelSelectionSuite) reportModels(body string) {
	res := Request(s.T(), s.App, "POST", "/api/private/agent/gateway/recovered", body, s.AgentApiKey)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
}

func (s *ModelSelectionSuite) setAgentTurns(plan, implement string) {
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		fmt.Sprintf(`{"gatewayUrl":%q,"maxTurnsPlan":%s,"maxTurnsImplement":%s}`, s.Gateway.URL, plan, implement),
		s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
}

func (s *ModelSelectionSuite) overview() model.AgentOverview {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/agents/overview", s.IdProject), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var all []model.AgentOverview
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&all))
	for _, entry := range all {
		if entry.IdUserAgent == s.AgentUserID {
			return entry
		}
	}
	s.FailNow("agent missing from the overview")
	return model.AgentOverview{}
}

func (s *ModelSelectionSuite) assign(choices string) *http.Response {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"model selection","description":"body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))

	body := fmt.Sprintf(`{"idUserAgent":%d`, s.AgentUserID)
	if choices != "" {
		body += "," + choices
	}
	body += "}"
	return Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", s.IdProject, iss.IdIssuePublic),
		body, s.Token)
}

func (s *ModelSelectionSuite) assignOk(choices string) model.AgentRun {
	res := s.assign(choices)
	body := readBody(s.T(), res)
	s.Require().Equal(http.StatusOK, res.StatusCode, body)
	var run model.AgentRun
	s.Require().NoError(json.Unmarshal([]byte(body), &run))
	return run
}

func (s *ModelSelectionSuite) dispatch(run model.AgentRun, stage string) (*model.AgentTask, dispatchedStage) {
	ctx := context.Background()
	loaded, err := injector.GetAgentRunRepository().LoadById(ctx, run.IdRun)
	s.Require().NoError(err)
	task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, s.AgentUserID, stage, 1)
	s.Require().NoError(err)

	injector.GetDispatcher().DispatchStageExecute(ctx, loaded, task)

	var got dispatchedStage
	select {
	case got = <-s.stageChannel(task.IdTask):
	case <-time.After(10 * time.Second):
		s.FailNow("gateway never received the stage_execute event")
	}
	stored, err := injector.GetAgentTaskRepository().LoadById(ctx, task.IdTask)
	s.Require().NoError(err)
	return stored, got
}

func (s *ModelSelectionSuite) TestOverviewOffersTheReportedModelsAndTurnDefaults() {
	s.reportModels(`{"models":["openai/kimi","ollama/qwen3:8b"],"modelDefault":"ollama/qwen3:8b"}`)

	entry := s.overview()

	s.Equal([]string{"openai/kimi", "ollama/qwen3:8b"}, entry.Models)
	s.Require().NotNil(entry.ModelDefault)
	s.Equal("ollama/qwen3:8b", *entry.ModelDefault)
	s.Equal(250, entry.MaxTurnsPlan)
	s.Equal(500, entry.MaxTurnsImplement)
}

func (s *ModelSelectionSuite) TestAgentTurnLimitsOverrideTheAppSettings() {
	s.setAgentTurns(`120`, `null`)

	entry := s.overview()

	s.Equal(120, entry.MaxTurnsPlan)
	s.Equal(500, entry.MaxTurnsImplement)
}

func (s *ModelSelectionSuite) TestAppSettingsSetTheTurnDefaults() {
	setTurns := func(body string) {
		res := Request(s.T(), s.App, http.MethodPatch, "/api/private/admin/settings", body, s.Token)
		s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
	}
	setTurns(`{"agentMaxTurnsPlan":90,"agentMaxTurnsImplement":900}`)
	defer setTurns(`{"agentMaxTurnsPlan":250,"agentMaxTurnsImplement":500}`)

	entry := s.overview()

	s.Equal(90, entry.MaxTurnsPlan)
	s.Equal(900, entry.MaxTurnsImplement)
}

func (s *ModelSelectionSuite) TestTurnsNeverExceedTheGatewayLimit() {
	s.reportModels(`{"models":["openai/kimi"],"modelDefault":"openai/kimi","maxTurnsLimit":300}`)
	run := s.assignOk(`"maxTurnsByStage":{"design":2000}`)

	entry := s.overview()
	s.Require().NotNil(entry.MaxTurnsLimit)
	s.Equal(300, *entry.MaxTurnsLimit)
	s.Equal(250, entry.MaxTurnsPlan)
	s.Equal(300, entry.MaxTurnsImplement, "the dock shows the default the stage will really get")

	for _, stage := range []string{"design", "implementation"} {
		_, got := s.dispatch(run, stage)
		s.Equal(300, got.MaxTurns, stage)
	}
}

func (s *ModelSelectionSuite) TestCreatingAGatewayStoresItsTurnLimits() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", `{"name":"turnsbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", agent.IdUser)

	created := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", agent.IdUser),
		`{"gatewayUrl":"http://gw:9090","maxTurnsPlan":90,"maxTurnsImplement":null}`, s.Token)
	body := readBody(s.T(), created)
	s.Require().Equal(http.StatusOK, created.StatusCode, body)

	var gateway model.AgentGateway
	s.Require().NoError(json.Unmarshal([]byte(body), &gateway))
	s.Require().NotNil(gateway.MaxTurnsPlan)
	s.Equal(90, *gateway.MaxTurnsPlan)
	s.Nil(gateway.MaxTurnsImplement)
}

func (s *ModelSelectionSuite) TestRecoveredWithoutABodyKeepsTheReportedModels() {
	s.reportModels(``)

	s.Equal([]string{"openai/kimi", "ollama/qwen3:8b"}, s.overview().Models)
}

func (s *ModelSelectionSuite) TestAssignRejectsAModelTheAgentDoesNotOffer() {
	for _, choices := range []string{
		`"model":"anthropic/opus"`,
		`"modelByStage":{"design":"anthropic/opus"}`,
	} {
		res := s.assign(choices)
		s.Equal(http.StatusBadRequest, res.StatusCode, choices)
		s.Contains(readBody(s.T(), res), "error.agent_model_not_offered", choices)
	}
}

func (s *ModelSelectionSuite) TestAssignRejectsAnUnusableTurnLimit() {
	res := s.assign(`"maxTurnsByStage":{"design":0}`)

	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *ModelSelectionSuite) TestAssignStoresTheModelChoicesInThePlan() {
	run := s.assignOk(`"model":"ollama/qwen3:8b","modelByStage":{"design":"openai/kimi"},"maxTurnsByStage":{"implementation":700}`)

	var plan model.AgentStagePlan
	s.Require().NoError(json.Unmarshal(run.StagePlan, &plan))
	s.Equal("ollama/qwen3:8b", plan.Model)
	for _, entry := range plan.Stages {
		switch entry.Name {
		case "design":
			s.Equal("openai/kimi", entry.Model)
		case "implementation":
			s.Equal(700, entry.MaxTurns)
		default:
			s.Empty(entry.Model, entry.Name)
			s.Zero(entry.MaxTurns, entry.Name)
		}
	}
}

func (s *ModelSelectionSuite) TestDispatchResolvesTheModelStageThenRunThenDefault() {
	withRunModel := s.assignOk(`"model":"ollama/qwen3:8b","modelByStage":{"design":"openai/kimi"}`)
	withoutChoices := s.assignOk(``)

	tests := []struct {
		run   model.AgentRun
		stage string
		want  string
	}{
		{withRunModel, "design", "openai/kimi"},
		{withRunModel, "brainstorming", "ollama/qwen3:8b"},
		{withoutChoices, "design", "openai/kimi"},
	}
	for _, tt := range tests {
		task, got := s.dispatch(tt.run, tt.stage)
		s.Require().NotNil(got.Model, tt.stage)
		s.Equal(tt.want, *got.Model, tt.stage)
		s.Require().NotNil(task.Model, tt.stage)
		s.Equal(tt.want, *task.Model, "the task records the model it ran with")
	}
}

func (s *ModelSelectionSuite) TestDispatchResolvesTurnsStageThenAgentThenAppSettings() {
	s.setAgentTurns(`120`, `null`)
	run := s.assignOk(`"maxTurnsByStage":{"design":40}`)

	tests := []struct {
		stage string
		want  int
	}{
		{"design", 40},
		{"brainstorming", 120},
		{"implementation", 500},
	}
	for _, tt := range tests {
		task, got := s.dispatch(run, tt.stage)
		s.Equal(tt.want, got.MaxTurns, tt.stage)
		s.Require().NotNil(task.MaxTurns, tt.stage)
		s.Equal(tt.want, *task.MaxTurns, tt.stage)
	}
}

func (s *ModelSelectionSuite) TestDispatchFallsBackToTheDefaultWhenTheModelIsGone() {
	run := s.assignOk(`"model":"ollama/qwen3:8b"`)
	s.reportModels(`{"models":["openai/kimi"],"modelDefault":"openai/kimi"}`)

	task, got := s.dispatch(run, "design")

	s.Require().NotNil(got.Model)
	s.Equal("openai/kimi", *got.Model)
	s.Require().NotNil(task.Model)
	s.Equal("openai/kimi", *task.Model, "the task records the model that actually ran")

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&snapshot))

	hasFallbackEvent := false
	for _, event := range snapshot.Events {
		if event.Reason != nil && *event.Reason == "model_fallback" {
			hasFallbackEvent = true
		}
	}
	s.True(hasFallbackEvent, "the fallback is recorded on the run")
	for _, stage := range snapshot.Stages {
		if stage.Stage == "design" {
			s.Require().NotNil(stage.Model)
			s.Equal("openai/kimi", *stage.Model)
			s.Require().NotNil(stage.ModelFallbackFrom, "the timeline shows the fallback")
			s.Equal("ollama/qwen3:8b", *stage.ModelFallbackFrom)
		}
	}
}

func (s *ModelSelectionSuite) TestAnAgentThatNeverReportedLeavesTheModelToTheGateway() {
	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE agent.gateway SET models = '{}', model_default = NULL WHERE id_user_agent = $1", s.AgentUserID)
	s.Require().NoError(err)
	run := s.assignOk(``)

	task, got := s.dispatch(run, "design")

	s.Nil(got.Model)
	s.Nil(task.Model)
}

func (s *ModelSelectionSuite) TestRunStatsSplitTokensByModel() {
	run := s.assignOk(`"modelByStage":{"design":"ollama/qwen3:8b"}`)
	design, _ := s.dispatch(run, "design")
	brainstorming, _ := s.dispatch(run, "brainstorming")
	ctx := context.Background()
	tokens := func(count int) *int { return &count }
	s.Require().NoError(injector.GetAgentTaskRepository().UpdateStats(ctx, design.IdTask, tokens(100), nil, nil))
	s.Require().NoError(injector.GetAgentTaskRepository().UpdateStats(ctx, brainstorming.IdTask, tokens(30), nil, nil))

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d/stats", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var stats model.RunStatsRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&stats))

	s.Equal(map[string]int{"ollama/qwen3:8b": 100, "openai/kimi": 30}, stats.TokensByModel)
}

func TestModelSelectionSuite(t *testing.T) {
	suite.Run(t, new(ModelSelectionSuite))
}
