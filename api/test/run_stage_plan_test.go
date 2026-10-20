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
	"github.com/stretchr/testify/suite"
)

type RunStagePlanSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	AgentUserID int64
	IdProject   int64
	IdSkill     int64
}

func (s *RunStagePlanSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	seedBuiltinSkills(s.T())

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"runstageplanbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	s.AgentUserID = agent.IdUser

	s.IdProject = createProject(s.T(), s.App, s.Token, "run-stage-plan-project")

	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.AgentUserID), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode)

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	s.IdSkill = skillByName(s.T(), listSkills(s.T(), s.App, s.Token), "Testing rules").IdSkill
	putRes := putProjectSkills(s.T(), s.App, s.Token, s.IdProject, []model.UpdateProjectSkillReq{
		{IdSkill: s.IdSkill, Stage: "design"},
	})
	s.Require().Equal(http.StatusOK, putRes.StatusCode)
}

func (s *RunStagePlanSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", s.AgentUserID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", s.AgentUserID)
}

func (s *RunStagePlanSuite) SetupTest() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

func (s *RunStagePlanSuite) createRun(title string) model.AgentRun {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"body","assignedTo":%d,"estimated":0}`, title, s.AgentUserID),
		s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))

	runRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d/agent/run", s.IdProject, iss.IdIssuePublic), "", s.Token)
	s.Require().Equal(http.StatusOK, runRes.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(runRes.Body).Decode(&snapshot))
	s.Require().NotZero(snapshot.IdRun, "assigning a bot must create a run")
	return snapshot.AgentRun
}

func (s *RunStagePlanSuite) getStagePlan(idRun int64) model.AgentRunStagePlanRes {
	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d/stage-plan", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var payload model.AgentRunStagePlanRes
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&payload))
	return payload
}

func (s *RunStagePlanSuite) patchStagePlanEntry(idRun int64, body string) *http.Response {
	return Request(s.T(), s.App, "PATCH", fmt.Sprintf("/api/private/agent/run/%d/stage-plan", idRun), body, s.Token)
}

func (s *RunStagePlanSuite) stage(payload model.AgentRunStagePlanRes, name string) model.AgentRunStagePlanEntry {
	for _, stage := range payload.Stages {
		if stage.Name == name {
			return stage
		}
	}
	s.Failf("stage missing", "stage %q not in the run's stage plan", name)
	return model.AgentRunStagePlanEntry{}
}

func (s *RunStagePlanSuite) TestFreshRunInheritsProjectMatrix() {
	run := s.createRun("inherits-matrix")
	payload := s.getStagePlan(run.IdRun)

	s.Len(payload.Stages, 4, "pickup is never a skill stage")
	s.Equal([]int64{s.IdSkill}, s.stage(payload, "design").IdsSkill)
	s.Empty(s.stage(payload, "brainstorming").IdsSkill)
	s.False(s.stage(payload, "design").Dispatched)
}

func (s *RunStagePlanSuite) TestPatchUndispatchedStage() {
	run := s.createRun("patch-stage")

	body := fmt.Sprintf(`{"stage":"implementation","idsSkill":[%d]}`, s.IdSkill)
	res := s.patchStagePlanEntry(run.IdRun, body)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal([]int64{s.IdSkill}, s.stage(s.getStagePlan(run.IdRun), "implementation").IdsSkill)
}

func (s *RunStagePlanSuite) TestPatchDispatchedStageConflicts() {
	run := s.createRun("patch-dispatched")

	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO agent.task (id_run, stage, attempt_no, status)
		VALUES ($1, 'design', 1, 'active')`, run.IdRun)
	s.Require().NoError(err)

	body := fmt.Sprintf(`{"stage":"design","idsSkill":[%d]}`, s.IdSkill)
	res := s.patchStagePlanEntry(run.IdRun, body)
	s.Equal(http.StatusConflict, res.StatusCode)
	s.True(s.stage(s.getStagePlan(run.IdRun), "design").Dispatched)
}

func (s *RunStagePlanSuite) TestPatchPickupStageRejected() {
	run := s.createRun("patch-pickup")

	res := s.patchStagePlanEntry(run.IdRun, `{"stage":"pickup","idsSkill":[]}`)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *RunStagePlanSuite) TestRestartPreservesOverriddenSkills() {
	run := s.createRun("restart-preserves")

	body := `{"stage":"design","idsSkill":[]}`
	patch := s.patchStagePlanEntry(run.IdRun, body)
	s.Require().Equal(http.StatusOK, patch.StatusCode)

	restart := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/restart", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, restart.StatusCode)
	var ids struct {
		NewIdRun int64 `json:"newIdRun"`
	}
	s.Require().NoError(json.NewDecoder(restart.Body).Decode(&ids))

	s.Empty(s.stage(s.getStagePlan(ids.NewIdRun), "design").IdsSkill)
}

func (s *RunStagePlanSuite) reportModels() {
	err := injector.GetAgentGatewayRepository().SetModels(context.Background(), s.AgentUserID, model.GatewayRecoveredReq{
		Models:        []string{"openai/a", "openai/b"},
		ModelDefault:  "openai/a",
		MaxTurnsLimit: 700,
	})
	s.Require().NoError(err)
}

func (s *RunStagePlanSuite) TestStagePlanListsWhatTheGatewayOffers() {
	s.reportModels()
	run := s.createRun("gateway-offer")

	payload := s.getStagePlan(run.IdRun)

	s.Equal([]string{"openai/a", "openai/b"}, payload.Models)
	s.Require().NotNil(payload.ModelDefault)
	s.Equal("openai/a", *payload.ModelDefault)
	s.Equal(250, payload.MaxTurnsPlan)
	s.Equal(500, payload.MaxTurnsImplement)
	s.Require().NotNil(payload.MaxTurnsLimit)
	s.Equal(700, *payload.MaxTurnsLimit)
	s.Nil(s.stage(payload, "design").Model)
	s.Nil(s.stage(payload, "design").MaxTurns)
}

func (s *RunStagePlanSuite) TestPatchChangesTheModelAndTurnsOfAStage() {
	s.reportModels()
	run := s.createRun("patch-model")

	res := s.patchStagePlanEntry(run.IdRun, `{"stage":"implementation","idsSkill":[],"model":"openai/b","maxTurns":120}`)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	implementation := s.stage(s.getStagePlan(run.IdRun), "implementation")
	s.Require().NotNil(implementation.Model)
	s.Equal("openai/b", *implementation.Model)
	s.Require().NotNil(implementation.MaxTurns)
	s.Equal(120, *implementation.MaxTurns)
}

func (s *RunStagePlanSuite) TestPatchWithoutAModelResetsTheStageToTheRunModel() {
	s.reportModels()
	run := s.createRun("patch-reset")
	s.Require().Equal(http.StatusOK, s.patchStagePlanEntry(run.IdRun, `{"stage":"design","model":"openai/b","maxTurns":90}`).StatusCode)

	res := s.patchStagePlanEntry(run.IdRun, `{"stage":"design","idsSkill":[]}`)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	design := s.stage(s.getStagePlan(run.IdRun), "design")
	s.Nil(design.Model)
	s.Nil(design.MaxTurns)
}

func (s *RunStagePlanSuite) TestPatchRejectsWhatTheGatewayDoesNotAllow() {
	s.reportModels()
	run := s.createRun("patch-reject")

	for _, body := range []string{
		`{"stage":"design","model":"anthropic/opus"}`,
		`{"stage":"design","maxTurns":-1}`,
		`{"stage":"design","maxTurns":10001}`,
	} {
		s.Equal(http.StatusBadRequest, s.patchStagePlanEntry(run.IdRun, body).StatusCode, body)
	}
	s.Nil(s.stage(s.getStagePlan(run.IdRun), "design").Model)
}

func (s *RunStagePlanSuite) TestAStageKeepsSavingAfterTheGatewayDropsItsModel() {
	s.reportModels()
	run := s.createRun("dropped-model")
	s.Require().Equal(http.StatusOK, s.patchStagePlanEntry(run.IdRun, `{"stage":"design","model":"openai/b"}`).StatusCode)
	err := injector.GetAgentGatewayRepository().SetModels(context.Background(), s.AgentUserID, model.GatewayRecoveredReq{
		Models:       []string{"openai/a"},
		ModelDefault: "openai/a",
	})
	s.Require().NoError(err)

	res := s.patchStagePlanEntry(run.IdRun, fmt.Sprintf(`{"stage":"design","idsSkill":[%d],"model":"openai/b"}`, s.IdSkill))

	s.Equal(http.StatusOK, res.StatusCode)
	s.Equal([]int64{s.IdSkill}, s.stage(s.getStagePlan(run.IdRun), "design").IdsSkill)
	s.Equal(http.StatusBadRequest, s.patchStagePlanEntry(run.IdRun, `{"stage":"implementation","model":"openai/b"}`).StatusCode,
		"a model the gateway no longer offers cannot be newly chosen")
}

func (s *RunStagePlanSuite) TestStagePlanMarksTheStagesAShortWorkflowSkips() {
	run := s.createRun("short-workflow-skip")
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/agent/run/%d/workflow", run.IdRun), `{"isShortWorkflow":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	payload := s.getStagePlan(run.IdRun)

	s.True(s.stage(payload, "design").Skip)
	s.True(s.stage(payload, "implementation_plan").Skip)
	s.False(s.stage(payload, "implementation").Skip)
}

func (s *RunStagePlanSuite) TestInsertMaterializesSkillsIntoStagePlan() {
	repo := injector.GetAgentRunRepository()
	ctx := context.Background()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"direct-insert","description":"body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))

	stagePlan, err := injector.GetAgentStagePlanService().BuildFromChoices(model.RunChoices{IdsSkillByStage: map[string][]int64{"design": {s.IdSkill}}})
	s.Require().NoError(err)
	run, err := repo.Insert(ctx, iss.IdIssue, s.AgentUserID, s.IdProject, stagePlan)
	s.Require().NoError(err)

	var plan model.AgentStagePlan
	s.Require().NoError(json.Unmarshal(run.StagePlan, &plan))
	for _, entry := range plan.Stages {
		if entry.Name == "design" {
			s.Equal([]int64{s.IdSkill}, entry.IdsSkill)
			return
		}
	}
	s.Fail("design stage missing from stage plan")
}

func Test_RunStagePlanSuite(t *testing.T) {
	suite.Run(t, new(RunStagePlanSuite))
}
