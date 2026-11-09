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
	Effort   *string `json:"effort"`
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
	ensureGitIntegration(s.T(), s.App, s.Token, s.IdProject)
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
	s.App.Pool.Exec(ctx, "UPDATE agent.gateway SET models_without_thinking = '{}' WHERE id_user_agent = $1", s.AgentUserID)
	s.reportModels(`{"models":["openai/kimi","ollama/qwen3:8b"],"modelDefault":"openai/kimi"}`)
	s.setAgentStageDefaults(`{}`)
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

func (s *ModelSelectionSuite) reportModelThinking(body string) {
	res := Request(s.T(), s.App, "POST", "/api/private/agent/gateway/thinking", body, s.AgentApiKey)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
}

func (s *ModelSelectionSuite) setAgentStageDefaults(stageDefaults string) {
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		fmt.Sprintf(`{"gatewayUrl":%q,"stageDefaults":%s}`, s.Gateway.URL, stageDefaults),
		s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
}

func (s *ModelSelectionSuite) setAppSettings(body string) {
	res := Request(s.T(), s.App, http.MethodPatch, "/api/private/admin/settings", body, s.Token)
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
	s.Len(entry.DefaultsByStage, 5, "every stage an agent runs")
	s.Equal(250, entry.DefaultsByStage["design"].MaxTurns)
	s.Equal(250, entry.DefaultsByStage["release_notes"].MaxTurns)
	s.Equal(500, entry.DefaultsByStage["implementation"].MaxTurns)
	s.NotContains(entry.DefaultsByStage, "pickup")
}

func (s *ModelSelectionSuite) TestAgentTurnLimitsOverrideTheAppSettingsOfThatStage() {
	s.setAgentStageDefaults(`{"brainstorming":{"maxTurns":120}}`)

	entry := s.overview()

	s.Equal(120, entry.DefaultsByStage["brainstorming"].MaxTurns)
	s.Equal(250, entry.DefaultsByStage["design"].MaxTurns)
}

func (s *ModelSelectionSuite) TestAppSettingsSetTheTurnDefaultsPerStage() {
	s.setAppSettings(`{"agentStageDefaults":{"release_notes":{"maxTurns":60},"implementation":{"maxTurns":900}}}`)
	defer s.setAppSettings(`{"agentStageDefaults":{"release_notes":{"maxTurns":250},"implementation":{"maxTurns":500}}}`)

	entry := s.overview()

	s.Equal(60, entry.DefaultsByStage["release_notes"].MaxTurns)
	s.Equal(900, entry.DefaultsByStage["implementation"].MaxTurns)
	s.Equal(250, entry.DefaultsByStage["implementation_plan"].MaxTurns)

	res := Request(s.T(), s.App, http.MethodGet, "/api/private/settings", "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var settings model.AppSettingsRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&settings))
	s.Equal(model.AgentStageOptions{MaxTurns: ptr(60)}, settings.AgentStageDefaults["release_notes"])
}

func (s *ModelSelectionSuite) TestAppSettingsRejectAStageWithoutAnAgent() {
	res := Request(s.T(), s.App, http.MethodPatch, "/api/private/admin/settings", `{"agentStageDefaults":{"pickup":{"maxTurns":10}}}`, s.Token)

	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *ModelSelectionSuite) TestTurnsNeverExceedTheGatewayLimit() {
	s.reportModels(`{"models":["openai/kimi"],"modelDefault":"openai/kimi","maxTurnsLimit":300}`)
	run := s.assignOk(`"maxTurnsByStage":{"design":2000}`)

	entry := s.overview()
	s.Require().NotNil(entry.MaxTurnsLimit)
	s.Equal(300, *entry.MaxTurnsLimit)
	s.Equal(250, entry.DefaultsByStage["design"].MaxTurns)
	s.Equal(300, entry.DefaultsByStage["implementation"].MaxTurns, "the dock shows the default the stage will really get")

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
		`{"gatewayUrl":"http://gw:9090","stageDefaults":{"design":{"maxTurns":90},"implementation":{"maxTurns":null,"effort":null}}}`, s.Token)
	body := readBody(s.T(), created)
	s.Require().Equal(http.StatusOK, created.StatusCode, body)

	var gateway model.AgentGateway
	s.Require().NoError(json.Unmarshal([]byte(body), &gateway))
	s.Equal(map[string]model.AgentStageOptions{"design": {MaxTurns: ptr(90)}}, gateway.StageDefaults,
		"a stage left empty inherits, so it is not stored")
}

func (s *ModelSelectionSuite) TestGatewayRejectsInvalidStageDefaults() {
	for _, stageDefaults := range []string{
		`{"pickup":{"maxTurns":10}}`,
		`{"design":{"maxTurns":0}}`,
		`{"design":{"maxTurns":10001}}`,
		`{"design":{"effort":"auto"}}`,
	} {
		res := Request(s.T(), s.App, "PATCH",
			fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
			fmt.Sprintf(`{"gatewayUrl":%q,"stageDefaults":%s}`, s.Gateway.URL, stageDefaults), s.Token)
		s.Equal(http.StatusBadRequest, res.StatusCode, stageDefaults)
	}
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
	s.setAgentStageDefaults(`{"brainstorming":{"maxTurns":120}}`)
	run := s.assignOk(`"maxTurnsByStage":{"design":40}`)

	tests := []struct {
		stage string
		want  int
	}{
		{"design", 40},
		{"brainstorming", 120},
		{"implementation_plan", 250},
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
	s.Require().NoError(injector.GetAgentTaskRepository().UpdateStats(ctx, design.IdTask, tokens(100), nil, nil, nil, nil, nil, nil, nil, false))
	s.Require().NoError(injector.GetAgentTaskRepository().UpdateStats(ctx, brainstorming.IdTask, tokens(30), nil, nil, nil, nil, nil, nil, nil, false))

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d/stats", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var stats model.RunStatsRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&stats))

	s.Equal(map[string]int{"ollama/qwen3:8b": 100, "openai/kimi": 30}, stats.TokensByModel)
}

func (s *ModelSelectionSuite) TestOverviewOffersTheEffortDefaultsAndModelsWithoutThinking() {
	s.reportModels(`{"models":["openai/kimi","openai/gpt-4o","ollama/llama3.3"],"modelDefault":"openai/kimi"}`)
	s.reportModelThinking(`{"isThinkingByModel":{"ollama/llama3.3":false}}`)
	s.setAgentStageDefaults(`{"design":{"effort":"high"}}`)

	entry := s.overview()

	s.Equal(model.AgentEffortDefault{Effort: ptr("high"), Source: "agent"}, entry.DefaultsByStage["design"].Effort)
	s.Equal(model.AgentEffortDefault{Source: "model"}, entry.DefaultsByStage["implementation_plan"].Effort)
	s.Equal([]string{"openai/gpt-4o", "ollama/llama3.3"}, entry.ModelsWithoutThinking,
		"gpt-4o comes from the models.dev snapshot, llama3.3 from the gateway report")
}

func (s *ModelSelectionSuite) TestModelThinkingReportKeepsTheModelsItDidNotProbe() {
	s.reportModels(`{"models":["ollama/llama3.3","ollama/phi4","ollama/qwen3:8b"],"modelDefault":"ollama/qwen3:8b"}`)
	s.reportModelThinking(`{"isThinkingByModel":{"ollama/llama3.3":false,"ollama/phi4":false}}`)

	s.reportModelThinking(`{"isThinkingByModel":{"ollama/phi4":true,"ollama/qwen3:8b":true}}`)

	s.Equal([]string{"ollama/llama3.3"}, s.overview().ModelsWithoutThinking)
}

func (s *ModelSelectionSuite) TestGatewayRestartKeepsTheModelsWithoutThinking() {
	s.reportModels(`{"models":["ollama/llama3.3"],"modelDefault":"ollama/llama3.3"}`)
	s.reportModelThinking(`{"isThinkingByModel":{"ollama/llama3.3":false}}`)

	s.reportModels(`{"models":["ollama/llama3.3"],"modelDefault":"ollama/llama3.3"}`)

	s.Equal([]string{"ollama/llama3.3"}, s.overview().ModelsWithoutThinking)
}

func (s *ModelSelectionSuite) TestModelThinkingReportRequiresTheModels() {
	res := Request(s.T(), s.App, "POST", "/api/private/agent/gateway/thinking", `{}`, s.AgentApiKey)

	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *ModelSelectionSuite) TestAppSettingsSetTheEffortDefaults() {
	s.setAppSettings(`{"agentStageDefaults":{"release_notes":{"effort":"low"}}}`)
	defer s.setAppSettings(`{"agentStageDefaults":{"release_notes":{"effort":""}}}`)

	entry := s.overview()

	s.Equal(model.AgentEffortDefault{Source: "model"}, entry.DefaultsByStage["implementation"].Effort)
	s.Equal(model.AgentEffortDefault{Effort: ptr("low"), Source: "settings"}, entry.DefaultsByStage["release_notes"].Effort)
}

func (s *ModelSelectionSuite) TestAppSettingsRejectAnUnknownEffort() {
	res := Request(s.T(), s.App, http.MethodPatch, "/api/private/admin/settings", `{"agentStageDefaults":{"design":{"effort":"xhigh"}}}`, s.Token)

	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *ModelSelectionSuite) TestAssignRejectsAnUnknownEffort() {
	for _, choices := range []string{`"effort":"xhigh"`, `"effortByStage":{"design":"auto"}`} {
		res := s.assign(choices)
		s.Equal(http.StatusBadRequest, res.StatusCode, choices)
	}
}

func (s *ModelSelectionSuite) TestDispatchResolvesTheEffortStageThenRunThenAgent() {
	s.setAgentStageDefaults(`{"brainstorming":{"effort":"medium"}}`)
	withChoices := s.assignOk(`"effort":"low","effortByStage":{"design":"max"}`)
	withoutChoices := s.assignOk(``)

	tests := []struct {
		run   model.AgentRun
		stage string
		want  *string
	}{
		{withChoices, "design", ptr("max")},
		{withChoices, "implementation", ptr("low")},
		{withoutChoices, "brainstorming", ptr("medium")},
		{withoutChoices, "implementation", nil},
	}
	for _, tt := range tests {
		task, got := s.dispatch(tt.run, tt.stage)
		s.Equal(tt.want, got.Effort, tt.stage)
		s.Equal(tt.want, task.Effort, "the task records the effort it ran with")
	}
}

func (s *ModelSelectionSuite) TestDispatchSendsNoEffortForAModelWithoutThinking() {
	s.reportModels(`{"models":["openai/kimi","openai/gpt-4o"],"modelDefault":"openai/kimi"}`)
	run := s.assignOk(`"model":"openai/gpt-4o","effort":"high"`)

	task, got := s.dispatch(run, "design")

	s.Nil(got.Effort, "the gateway gets no effort for a model without thinking")
	s.Nil(task.Effort, "the stage card shows no effort for it")
}

func TestModelSelectionSuite(t *testing.T) {
	suite.Run(t, new(ModelSelectionSuite))
}
