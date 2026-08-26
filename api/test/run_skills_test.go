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

type RunSkillsSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	BotUserID int64
	IdProject int64
	IdSkill   int64
}

func (s *RunSkillsSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	seedBuiltinSkills(s.T())

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"runskillbot","isBot":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var bot struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&bot))
	s.BotUserID = bot.IdUser

	s.IdProject = createProject(s.T(), s.App, s.Token, "run-skills-project")

	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode)

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.BotUserID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	s.IdSkill = skillByName(s.T(), listSkills(s.T(), s.App, s.Token), "Testing rules").IdSkill
	putRes := putProjectSkills(s.T(), s.App, s.Token, s.IdProject, []model.UpdateProjectSkillReq{
		{IdSkill: s.IdSkill, Stage: "design"},
	})
	s.Require().Equal(http.StatusOK, putRes.StatusCode)
}

func (s *RunSkillsSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.bot_gateway WHERE id_user_bot = $1", s.BotUserID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *RunSkillsSuite) SetupTest() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

func (s *RunSkillsSuite) createRun(title string) model.AgentRun {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"body","assignedTo":%d,"estimated":0}`, title, s.BotUserID),
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

func (s *RunSkillsSuite) getRunSkills(idRun int64) []model.AgentRunStageSkills {
	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d/skills", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var payload []model.AgentRunStageSkills
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&payload))
	return payload
}

func (s *RunSkillsSuite) stage(payload []model.AgentRunStageSkills, name string) model.AgentRunStageSkills {
	for _, stage := range payload {
		if stage.Name == name {
			return stage
		}
	}
	s.Failf("stage missing", "stage %q not in run skills payload", name)
	return model.AgentRunStageSkills{}
}

func (s *RunSkillsSuite) TestFreshRunInheritsProjectMatrix() {
	run := s.createRun("inherits-matrix")
	payload := s.getRunSkills(run.IdRun)

	s.Len(payload, 4, "pickup is never a skill stage")
	s.Equal([]int64{s.IdSkill}, s.stage(payload, "design").IdsSkill)
	s.Empty(s.stage(payload, "brainstorming").IdsSkill)
	s.False(s.stage(payload, "design").Dispatched)
}

func (s *RunSkillsSuite) TestPatchUndispatchedStage() {
	run := s.createRun("patch-stage")

	body := fmt.Sprintf(`{"stage":"implementation","idsSkill":[%d]}`, s.IdSkill)
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/agent/run/%d/skills", run.IdRun), body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal([]int64{s.IdSkill}, s.stage(s.getRunSkills(run.IdRun), "implementation").IdsSkill)
}

func (s *RunSkillsSuite) TestPatchDispatchedStageConflicts() {
	run := s.createRun("patch-dispatched")

	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO agent.task (id_run, stage, attempt_no, status)
		VALUES ($1, 'design', 1, 'active')`, run.IdRun)
	s.Require().NoError(err)

	body := fmt.Sprintf(`{"stage":"design","idsSkill":[%d]}`, s.IdSkill)
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/agent/run/%d/skills", run.IdRun), body, s.Token)
	s.Equal(http.StatusConflict, res.StatusCode)
	s.True(s.stage(s.getRunSkills(run.IdRun), "design").Dispatched)
}

func (s *RunSkillsSuite) TestPatchPickupStageRejected() {
	run := s.createRun("patch-pickup")

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/agent/run/%d/skills", run.IdRun),
		`{"stage":"pickup","idsSkill":[]}`, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *RunSkillsSuite) TestRestartPreservesOverriddenSkills() {
	run := s.createRun("restart-preserves")

	body := `{"stage":"design","idsSkill":[]}`
	patch := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/agent/run/%d/skills", run.IdRun), body, s.Token)
	s.Require().Equal(http.StatusOK, patch.StatusCode)

	restart := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/restart", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, restart.StatusCode)
	var ids struct {
		NewIdRun int64 `json:"newIdRun"`
	}
	s.Require().NoError(json.NewDecoder(restart.Body).Decode(&ids))

	s.Empty(s.stage(s.getRunSkills(ids.NewIdRun), "design").IdsSkill)
}

func (s *RunSkillsSuite) TestInsertMaterializesSkillsIntoStagePlan() {
	repo := injector.GetAgentRunRepository()
	ctx := context.Background()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"direct-insert","description":"body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))

	stagePlan, err := injector.GetStagePlanService().Build(map[string][]int64{"design": {s.IdSkill}})
	s.Require().NoError(err)
	run, err := repo.Insert(ctx, iss.IdIssue, s.BotUserID, s.IdProject, stagePlan)
	s.Require().NoError(err)

	var plan model.StagePlan
	s.Require().NoError(json.Unmarshal(run.StagePlan, &plan))
	for _, entry := range plan.Stages {
		if entry.Name == "design" {
			s.Equal([]int64{s.IdSkill}, entry.IdsSkill)
			return
		}
	}
	s.Fail("design stage missing from stage plan")
}

func Test_RunSkillsSuite(t *testing.T) {
	suite.Run(t, new(RunSkillsSuite))
}
