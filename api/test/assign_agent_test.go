package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type AssignAgentSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	AgentUserID int64
	HumanID     int64
	IdProject   int64
	IdSkill     int64
}

func (s *AssignAgentSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	seedBuiltinSkills(s.T())

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"assignbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	s.AgentUserID = agent.IdUser

	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"assignhuman","email":"assignhuman@test.sk","password":"kreslo1"}`)
	s.HumanID = idOfUser(s.T(), s.App, s.Token, "assignhuman@test.sk")

	s.IdProject = createProject(s.T(), s.App, s.Token, "assign-agent-project")
	for _, idUser := range []int64{s.AgentUserID, s.HumanID} {
		member := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.Require().Equal(http.StatusOK, member.StatusCode)
	}

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	s.IdSkill = skillByName(s.T(), listSkills(s.T(), s.App, s.Token), "PR rules").IdSkill
}

func (s *AssignAgentSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", s.AgentUserID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user IN ($1, $2)", s.AgentUserID, s.HumanID)
}

func (s *AssignAgentSuite) SetupTest() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

func (s *AssignAgentSuite) createIssue(title string) model.Issue {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))
	return iss
}

func (s *AssignAgentSuite) assignAgent(idIssuePublic int64, body string, token string) *http.Response {
	return Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", s.IdProject, idIssuePublic),
		body, token)
}

func (s *AssignAgentSuite) runCount(idIssuePublic int64) int {
	var count int
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent.run r
		JOIN issues.issue i ON i.id_issue = r.id_issue
		WHERE i.id_issue_public = $1 AND i.id_project = $2`,
		idIssuePublic, s.IdProject).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *AssignAgentSuite) TestAssignCreatesExactlyOneRunWithRequestedSkills() {
	iss := s.createIssue("assign-happy")

	res := s.assignAgent(iss.IdIssuePublic,
		fmt.Sprintf(`{"idUserAgent":%d,"idsSkillByStage":{"implementation":[%d]}}`, s.AgentUserID, s.IdSkill),
		s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&run))

	var plan model.AgentStagePlan
	s.Require().NoError(json.Unmarshal(run.StagePlan, &plan))
	for _, entry := range plan.Stages {
		if entry.Name == "implementation" {
			s.Equal([]int64{s.IdSkill}, entry.IdsSkill)
		}
		if entry.Name == "design" {
			s.Empty(entry.IdsSkill, "only the requested stages get skills, not the project defaults")
		}
	}

	s.Equal(1, s.runCount(iss.IdIssuePublic), "assign-agent must not also fire the EditIssue assignee hook")

	issRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic), "", s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var reloaded model.Issue
	s.Require().NoError(json.NewDecoder(issRes.Body).Decode(&reloaded))
	s.Require().NotNil(reloaded.AssignedTo)
	s.Equal(s.AgentUserID, *reloaded.AssignedTo)
}

func (s *AssignAgentSuite) assignFromDialog(idIssuePublic, idUserAgent int64, idsSkillByStage string, isShortWorkflow bool) *http.Response {
	return s.assignAgent(idIssuePublic,
		fmt.Sprintf(`{"idUserAgent":%d,"idsSkillByStage":%s,"isShortWorkflow":%t}`, idUserAgent, idsSkillByStage, isShortWorkflow),
		s.Token)
}

func (s *AssignAgentSuite) startRun(idIssuePublic int64, completedStages ...string) model.AgentRun {
	res := s.assignAgent(idIssuePublic, fmt.Sprintf(`{"idUserAgent":%d}`, s.AgentUserID), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&run))

	ctx := context.Background()
	for _, stage := range completedStages {
		task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, s.AgentUserID, stage, 1)
		s.Require().NoError(err)
		_, err = s.App.Pool.Exec(ctx, "UPDATE agent.task SET status = 'completed' WHERE id_task = $1", task.IdTask)
		s.Require().NoError(err)
	}
	_, err := s.App.Pool.Exec(ctx, "UPDATE agent.run SET phase = 'in_progress' WHERE id_run = $1", run.IdRun)
	s.Require().NoError(err)
	return run
}

func (s *AssignAgentSuite) assignHuman(idIssuePublic int64) {
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, idIssuePublic),
		fmt.Sprintf(`{"idProject":%d,"assignedTo":%d}`, s.IdProject, s.HumanID), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *AssignAgentSuite) snapshot(idIssuePublic int64) model.AgentRunWithEvents {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d/agent/run", s.IdProject, idIssuePublic), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&snapshot))
	return snapshot
}

func (s *AssignAgentSuite) assignee(idIssuePublic int64) *int64 {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, idIssuePublic), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var reloaded model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&reloaded))
	return reloaded.AssignedTo
}

func stagePlanOf(s *suite.Suite, stagePlan json.RawMessage) map[string]model.AgentStagePlanEntry {
	var plan model.AgentStagePlan
	s.Require().NoError(json.Unmarshal(stagePlan, &plan))
	byName := map[string]model.AgentStagePlanEntry{}
	for _, entry := range plan.Stages {
		byName[entry.Name] = entry
	}
	return byName
}

func (s *AssignAgentSuite) TestUnassignedAgentLeavesTheRunParked() {
	iss := s.createIssue("assign-parked")
	s.startRun(iss.IdIssuePublic)
	s.False(s.snapshot(iss.IdIssuePublic).IsParked)

	s.assignHuman(iss.IdIssuePublic)

	s.True(s.snapshot(iss.IdIssuePublic).IsParked, "the run's agent no longer holds the issue, so nothing dispatches")
}

func (s *AssignAgentSuite) TestAssignAgainResumesTheParkedRunWithTheDialogChoices() {
	iss := s.createIssue("assign-resume")
	original := s.startRun(iss.IdIssuePublic, "pickup", "brainstorming")
	s.assignHuman(iss.IdIssuePublic)

	res := s.assignFromDialog(iss.IdIssuePublic, s.AgentUserID,
		fmt.Sprintf(`{"brainstorming":[%d],"design":[%d]}`, s.IdSkill, s.IdSkill), true)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&run))

	s.Equal(original.IdRun, run.IdRun, "the parked run continues instead of a new one")
	s.Equal(1, s.runCount(iss.IdIssuePublic))
	plan := stagePlanOf(&s.Suite, run.StagePlan)
	s.Empty(plan["brainstorming"].IdsSkill, "brainstorming already ran, so its skills are settled")
	s.Equal([]int64{s.IdSkill}, plan["design"].IdsSkill)
	s.True(plan["design"].Skip, "the short workflow is applied while design has not started")

	s.False(s.snapshot(iss.IdIssuePublic).IsParked)
	s.Require().NotNil(s.assignee(iss.IdIssuePublic))
	s.Equal(s.AgentUserID, *s.assignee(iss.IdIssuePublic))
}

func (s *AssignAgentSuite) TestAssignAgainRefusesALockedWorkflowChange() {
	iss := s.createIssue("assign-locked")
	s.startRun(iss.IdIssuePublic, "pickup", "brainstorming", "design")
	s.assignHuman(iss.IdIssuePublic)

	res := s.assignFromDialog(iss.IdIssuePublic, s.AgentUserID, `{}`, true)

	s.Require().Equal(http.StatusConflict, res.StatusCode)
	s.Equal("WORKFLOW_LOCKED", errorCode(s.T(), res))
	s.Require().NotNil(s.assignee(iss.IdIssuePublic))
	s.Equal(s.HumanID, *s.assignee(iss.IdIssuePublic), "a refused assign must not change the assignee")
	s.True(s.snapshot(iss.IdIssuePublic).IsParked)
}

func (s *AssignAgentSuite) TestAssignAnotherAgentHandsTheParkedRunOver() {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"assignsecondbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var second struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&second))
	defer func() {
		ctx := context.Background()
		s.App.Pool.Exec(ctx,
			"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
		s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
		s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", second.IdUser)
	}()
	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, second.IdUser), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode)
	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", second.IdUser),
		`{"gatewayUrl":"http://gw:9092"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	iss := s.createIssue("assign-hand-over")
	original := s.startRun(iss.IdIssuePublic, "pickup")
	s.assignHuman(iss.IdIssuePublic)

	assign := s.assignFromDialog(iss.IdIssuePublic, second.IdUser, `{}`, false)
	s.Require().Equal(http.StatusOK, assign.StatusCode)
	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(assign.Body).Decode(&run))

	s.Equal(original.IdRun, run.IdRun)
	s.Equal(second.IdUser, run.IdUserAgent)
	s.Equal(1, s.runCount(iss.IdIssuePublic))
	s.False(s.snapshot(iss.IdIssuePublic).IsParked)
}

func (s *AssignAgentSuite) TestAssignWhileTheRunIsWorkingConflicts() {
	iss := s.createIssue("assign-working")
	original := s.startRun(iss.IdIssuePublic, "pickup")

	res := s.assignFromDialog(iss.IdIssuePublic, s.AgentUserID,
		fmt.Sprintf(`{"implementation":[%d]}`, s.IdSkill), true)

	s.Require().Equal(http.StatusConflict, res.StatusCode)
	s.Equal("ISSUE_HAS_ACTIVE_RUN", errorCode(s.T(), res))
	run := s.snapshot(iss.IdIssuePublic)
	s.Equal(original.StagePlan, run.StagePlan, "a working run's plan is not rewritten from the dialog")
}

func (s *AssignAgentSuite) TestAssignAgentFromAnotherProjectRejected() {
	outsideAgent := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"assignoutsidebot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, outsideAgent.StatusCode)
	var outsider struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(outsideAgent.Body).Decode(&outsider))
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", outsider.IdUser)

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", outsider.IdUser),
		`{"gatewayUrl":"http://gw:9091"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	iss := s.createIssue("assign-outside-agent")
	res := s.assignAgent(iss.IdIssuePublic, fmt.Sprintf(`{"idUserAgent":%d}`, outsider.IdUser), s.Token)

	s.Equal(http.StatusForbidden, res.StatusCode, "an agent that cannot read the project cannot be assigned to its issues")
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignAgentSuite) TestAssignHumanRejected() {
	iss := s.createIssue("assign-human")

	res := s.assignAgent(iss.IdIssuePublic, fmt.Sprintf(`{"idUserAgent":%d}`, s.HumanID), s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignAgentSuite) TestAssignUnknownStageRejected() {
	iss := s.createIssue("assign-bad-stage")

	res := s.assignAgent(iss.IdIssuePublic,
		fmt.Sprintf(`{"idUserAgent":%d,"idsSkillByStage":{"pickup":[%d]}}`, s.AgentUserID, s.IdSkill), s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignAgentSuite) TestAssignWithoutAclForbidden() {
	iss := s.createIssue("assign-acl")

	outsiderToken := createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"assign-outsider","email":"assign-outsider@test.sk","password":"kreslo1"}`)

	res := s.assignAgent(iss.IdIssuePublic, fmt.Sprintf(`{"idUserAgent":%d}`, s.AgentUserID), outsiderToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignAgentSuite) TestAssignAgentWithoutGatewayRejected() {
	iss := s.createIssue("assign-no-gw")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"gatewayless","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", agent.IdUser)

	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, agent.IdUser), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode, "membership must pass so the gateway check is what refuses")

	assign := s.assignAgent(iss.IdIssuePublic, fmt.Sprintf(`{"idUserAgent":%d}`, agent.IdUser), s.Token)
	s.Equal(http.StatusUnprocessableEntity, assign.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

// Assignee changes: assigning an agent continues work, comments wait for Send.

func (s *AssignAgentSuite) addTask(idRun int64, stage, status string) *model.AgentTask {
	ctx := context.Background()
	tasks, err := injector.GetAgentTaskRepository().LoadByRun(ctx, idRun)
	s.Require().NoError(err)
	task, err := injector.GetAgentTaskRepository().Insert(ctx, idRun, s.AgentUserID, stage, agent.ResolveNextAttemptNo(tasks, stage))
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx, "UPDATE agent.task SET status = $2, started_at = now() WHERE id_task = $1", task.IdTask, status)
	s.Require().NoError(err)
	return task
}

func (s *AssignAgentSuite) setPhase(idRun int64, phase string) {
	_, err := s.App.Pool.Exec(context.Background(), "UPDATE agent.run SET phase = $2 WHERE id_run = $1", idRun, phase)
	s.Require().NoError(err)
}

func (s *AssignAgentSuite) assignByEdit(idIssuePublic, idUser int64) {
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, idIssuePublic),
		fmt.Sprintf(`{"idProject":%d,"assignedTo":%d}`, s.IdProject, idUser), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *AssignAgentSuite) tasksOf(idRun int64) []*model.AgentTask {
	tasks, err := injector.GetAgentTaskRepository().LoadByRun(context.Background(), idRun)
	s.Require().NoError(err)
	return tasks
}

func (s *AssignAgentSuite) loadRun(idRun int64) *model.AgentRun {
	run, err := injector.GetAgentRunRepository().LoadById(context.Background(), idRun)
	s.Require().NoError(err)
	return run
}

// tick runs one scheduler pass and returns the tasks it dispatched for the run.
func (s *AssignAgentSuite) tick(idRun int64) []*model.AgentTask {
	dispatcher := &stubStageDispatcher{}
	sched := agent.NewScheduler(s.App.Pool, injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(), injector.GetIssueRepository(), dispatcher, injector.GetRunKinds(), injector.GetNotifier())
	s.Require().NoError(sched.TickOnce(context.Background()))
	var dispatched []*model.AgentTask
	for _, task := range dispatcher.dispatched {
		if task.IdRun == idRun {
			dispatched = append(dispatched, task)
		}
	}
	return dispatched
}

func (s *AssignAgentSuite) comment(idIssue int64, body string) {
	payload, err := json.Marshal(map[string]any{"idRecipient": idIssue, "idMessageRecipientType": 4, "message": body})
	s.Require().NoError(err)
	res := Request(s.T(), s.App, "POST", "/api/private/message", string(payload), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *AssignAgentSuite) sendFeedback(idRun int64) *http.Response {
	return Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/run/%d/feedback", idRun), "", s.Token)
}

func (s *AssignAgentSuite) createAgent(name, gatewayUrl string) int64 {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", fmt.Sprintf(`{"name":%q,"isAgent":true}`, name), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	s.T().Cleanup(func() {
		ctx := context.Background()
		s.App.Pool.Exec(ctx,
			"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
		s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
		s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", created.IdUser)
		s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", created.IdUser)
	})
	member := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, created.IdUser), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode)
	gw := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", created.IdUser),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gatewayUrl), s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)
	return created.IdUser
}

var allIssueStages = []string{
	constants.StagePickup, constants.StageBrainstorming, constants.StageDesign,
	constants.StageImplementationPlan, constants.StageImplementation,
}

func (s *AssignAgentSuite) TestUC1_AssignmentAfterADoneRunStartsANewRun() {
	iss := s.createIssue("uc1-done")
	original := s.startRun(iss.IdIssuePublic, allIssueStages...)
	s.setPhase(original.IdRun, constants.PhaseDone)
	s.assignHuman(iss.IdIssuePublic)

	s.assignByEdit(iss.IdIssuePublic, s.AgentUserID)

	s.Equal(2, s.runCount(iss.IdIssuePublic))
	s.NotEqual(original.IdRun, s.snapshot(iss.IdIssuePublic).IdRun)
}

func (s *AssignAgentSuite) TestUC2_ReassignRedoesTheInterruptedStage() {
	iss := s.createIssue("uc2-working")
	run := s.startRun(iss.IdIssuePublic, constants.StagePickup)
	s.addTask(run.IdRun, constants.StageBrainstorming, constants.TaskStatusActive)

	s.assignHuman(iss.IdIssuePublic)
	s.assignByEdit(iss.IdIssuePublic, s.AgentUserID)

	s.Equal(1, s.runCount(iss.IdIssuePublic))
	dispatched := s.tick(run.IdRun)
	s.Require().Len(dispatched, 1)
	s.Equal(constants.StageBrainstorming, dispatched[0].Stage)
	s.Equal(2, dispatched[0].AttemptNo)
	resumedAfter, _ := agent.ResolveAttemptContext(s.tasksOf(run.IdRun), dispatched[0])
	s.Equal("cancelled", resumedAfter)
}

func (s *AssignAgentSuite) TestUC2_ReassignRedoesACancelledRevision() {
	tests := []struct {
		name      string
		completed []string
		phase     string
		stage     string
	}{
		{"plan waiting for review", allIssueStages[:4], constants.PhaseAwaitingApproval, constants.StageImplementationPlan},
		{"pull request open", allIssueStages, constants.PhasePrOpen, constants.StageImplementation},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest() // an agent runs one stage at a time: drop the previous case's active task
			iss := s.createIssue("uc2-revision-" + tc.name)
			run := s.startRun(iss.IdIssuePublic, tc.completed...)
			s.setPhase(run.IdRun, tc.phase)
			s.comment(iss.IdIssue, "please change it")
			s.Require().Equal(http.StatusOK, s.sendFeedback(run.IdRun).StatusCode)
			s.Require().Len(s.tick(run.IdRun), 1, "the revision starts")

			s.assignHuman(iss.IdIssuePublic)
			s.assignByEdit(iss.IdIssuePublic, s.AgentUserID)

			dispatched := s.tick(run.IdRun)
			s.Require().Len(dispatched, 1)
			s.Equal(tc.stage, dispatched[0].Stage, "the cancelled revision is redone, not skipped")
			resumedAfter, isRevision := agent.ResolveAttemptContext(s.tasksOf(run.IdRun), dispatched[0])
			s.Equal("cancelled", resumedAfter)
			s.True(isRevision)
			s.NotEqual(constants.PhaseDone, s.loadRun(run.IdRun).Phase)
		})
	}
}

func (s *AssignAgentSuite) TestUC3UC4_AssigningAWaitingRunStartsNothing() {
	second := s.createAgent("uc9-second-bot", "http://gw:9093")
	tests := []struct {
		name       string
		isComment  bool
		idNewAgent int64
	}{
		{"UC3 new comments, same agent", true, s.AgentUserID},
		{"UC4 nothing new", false, s.AgentUserID},
		{"UC9 new comments, another agent", true, second},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest() // an agent runs one stage at a time: drop the previous case's active task
			iss := s.createIssue("uc3-" + tc.name)
			run := s.startRun(iss.IdIssuePublic, allIssueStages[:4]...)
			s.setPhase(run.IdRun, constants.PhaseAwaitingApproval)
			s.assignHuman(iss.IdIssuePublic)
			if tc.isComment {
				s.comment(iss.IdIssue, "one more thing")
			}
			tasksBefore := len(s.tasksOf(run.IdRun))

			s.assignByEdit(iss.IdIssuePublic, tc.idNewAgent)

			reloaded := s.loadRun(run.IdRun)
			s.Equal(constants.PhaseAwaitingApproval, reloaded.Phase, "assigning never sends the comments nor approves")
			s.Equal(tc.idNewAgent, reloaded.IdUserAgent)
			s.Len(s.tasksOf(run.IdRun), tasksBefore)
			s.Empty(s.tick(run.IdRun))
			snapshot := s.snapshot(iss.IdIssuePublic)
			s.Require().NotNil(snapshot.IdMessageSeen)
			count, err := injector.GetMessageRepository().CountNewFeedback(context.Background(), iss.IdIssue, *snapshot.IdMessageSeen)
			s.Require().NoError(err)
			if tc.isComment {
				s.Equal(1, count, "the comment still waits to be sent")
			} else {
				s.Zero(count)
			}
		})
	}
}

func (s *AssignAgentSuite) TestUC5_AssignmentContinuesAFailedOrCancelledRun() {
	second := s.createAgent("uc5-second-bot", "http://gw:9094")
	tests := []struct {
		name     string
		phase    string
		idAgent  int64
		isDialog bool
	}{
		{"failed, assignee field", constants.PhaseFailed, s.AgentUserID, false},
		{"cancelled, assignee field", constants.PhaseCancelled, s.AgentUserID, false},
		{"cancelled, assign dialog", constants.PhaseCancelled, s.AgentUserID, true},
		{"failed, another agent", constants.PhaseFailed, second, false},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest() // an agent runs one stage at a time: drop the previous case's active task
			iss := s.createIssue("uc5-" + tc.name)
			run := s.startRun(iss.IdIssuePublic, constants.StagePickup, constants.StageBrainstorming)
			s.addTask(run.IdRun, constants.StageDesign, constants.TaskStatusFailed)
			_, err := s.App.Pool.Exec(context.Background(), "UPDATE agent.run SET pr_id = '7' WHERE id_run = $1", run.IdRun)
			s.Require().NoError(err)
			s.setPhase(run.IdRun, tc.phase)
			s.assignHuman(iss.IdIssuePublic)

			if tc.isDialog {
				res := s.assignFromDialog(iss.IdIssuePublic, tc.idAgent, `{}`, false)
				s.Require().Equal(http.StatusOK, res.StatusCode)
			} else {
				s.assignByEdit(iss.IdIssuePublic, tc.idAgent)
			}

			s.Equal(1, s.runCount(iss.IdIssuePublic), "no new run")
			continued := s.loadRun(run.IdRun)
			s.Equal(constants.PhaseQueued, continued.Phase)
			s.Equal(tc.idAgent, continued.IdUserAgent)
			s.Require().NotNil(continued.PrId)
			s.Equal("7", *continued.PrId, "the PR is kept")
			dispatched := s.tick(run.IdRun)
			s.Require().Len(dispatched, 1)
			s.Equal(constants.StageDesign, dispatched[0].Stage, "finished stages are kept")
		})
	}
}

func (s *AssignAgentSuite) TestUC5_NothingToContinueStartsANewRun() {
	iss := s.createIssue("uc5-nothing-left")
	original := s.startRun(iss.IdIssuePublic, allIssueStages...)
	s.setPhase(original.IdRun, constants.PhaseFailed)
	s.assignHuman(iss.IdIssuePublic)

	s.assignByEdit(iss.IdIssuePublic, s.AgentUserID)

	s.Equal(2, s.runCount(iss.IdIssuePublic))
	s.Equal(constants.PhaseFailed, s.loadRun(original.IdRun).Phase)
}

func (s *AssignAgentSuite) TestUC5_ConcurrentAssignmentsContinueTheRunOnce() {
	iss := s.createIssue("uc5-concurrent")
	run := s.startRun(iss.IdIssuePublic, constants.StagePickup)
	s.addTask(run.IdRun, constants.StageBrainstorming, constants.TaskStatusFailed)
	s.setPhase(run.IdRun, constants.PhaseFailed)
	s.assignHuman(iss.IdIssuePublic)

	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := s.assignAgent(iss.IdIssuePublic, fmt.Sprintf(`{"idUserAgent":%d}`, s.AgentUserID), s.Token)
			statuses <- res.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)

	// The later assignment either finds the run already continuing (conflict) or, when it read the
	// issue before the first committed, takes it as a parked run and reassigns it: never a second run.
	isAnyAssigned := false
	for code := range statuses {
		s.Contains([]int{http.StatusOK, http.StatusConflict}, code)
		isAnyAssigned = isAnyAssigned || code == http.StatusOK
	}
	s.True(isAnyAssigned)
	s.Equal(1, s.runCount(iss.IdIssuePublic), "the loser never starts a second run")
	s.Equal(constants.PhaseQueued, s.loadRun(run.IdRun).Phase)
}

func (s *AssignAgentSuite) TestUC6_UnassignStopsTheStageAndKeepsTheRun() {
	iss := s.createIssue("uc6-unassign")
	run := s.startRun(iss.IdIssuePublic, constants.StagePickup)
	active := s.addTask(run.IdRun, constants.StageBrainstorming, constants.TaskStatusActive)

	s.assignHuman(iss.IdIssuePublic)

	reloaded := s.loadRun(run.IdRun)
	s.Equal(constants.PhaseInProgress, reloaded.Phase)
	for _, task := range s.tasksOf(run.IdRun) {
		if task.IdTask == active.IdTask {
			s.Equal(constants.TaskStatusCancelled, task.Status)
		}
	}
}

func (s *AssignAgentSuite) TestUC8_AnotherAgentRedoesTheStage() {
	second := s.createAgent("uc8-second-bot", "http://gw:9095")
	iss := s.createIssue("uc8-hand-over")
	run := s.startRun(iss.IdIssuePublic, constants.StagePickup)
	s.addTask(run.IdRun, constants.StageBrainstorming, constants.TaskStatusActive)

	s.assignByEdit(iss.IdIssuePublic, second)

	s.Equal(second, s.loadRun(run.IdRun).IdUserAgent)
	dispatched := s.tick(run.IdRun)
	s.Require().Len(dispatched, 1)
	s.Equal(constants.StageBrainstorming, dispatched[0].Stage)
	s.Require().NotNil(dispatched[0].IdUserAgent)
	s.Equal(second, *dispatched[0].IdUserAgent)
}

func (s *AssignAgentSuite) TestContinueAfterCancellingARevisionRedoesIt() {
	iss := s.createIssue("continue-cancelled-revision")
	run := s.startRun(iss.IdIssuePublic, allIssueStages[:4]...)
	s.setPhase(run.IdRun, constants.PhaseAwaitingApproval)
	s.comment(iss.IdIssue, "rework the plan")
	s.Require().Equal(http.StatusOK, s.sendFeedback(run.IdRun).StatusCode)
	s.Require().Len(s.tick(run.IdRun), 1)

	cancel := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/run/%d/cancel", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, cancel.StatusCode)
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/run/%d/continue", run.IdRun), "", s.Token)

	resBody := readBody(s.T(), res)
	s.Require().Equal(http.StatusOK, res.StatusCode, resBody)
	var body struct {
		NextStage string `json:"nextStage"`
	}
	s.Require().NoError(json.Unmarshal([]byte(resBody), &body))
	s.Equal(constants.StageImplementationPlan, body.NextStage)
}

func (s *AssignAgentSuite) TestContinuingARunCancelledWhileWaitingWaitsAgain() {
	tests := []struct {
		name            string
		completedStages []string
		phase           string
		resume          func(run model.AgentRun, iss model.Issue)
	}{
		{"review, assignee field", allIssueStages[:3], constants.PhaseAwaitingApproval, func(_ model.AgentRun, iss model.Issue) {
			s.assignByEdit(iss.IdIssuePublic, s.AgentUserID)
		}},
		{"question, assign dialog", allIssueStages[:2], constants.PhaseAwaitingInput, func(_ model.AgentRun, iss model.Issue) {
			res := s.assignFromDialog(iss.IdIssuePublic, s.AgentUserID, `{}`, false)
			s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
		}},
		{"PR review, assignee field", allIssueStages, constants.PhasePrOpen, func(_ model.AgentRun, iss model.Issue) {
			s.assignByEdit(iss.IdIssuePublic, s.AgentUserID)
		}},
		{"review, Continue", allIssueStages[:3], constants.PhaseAwaitingApproval, func(run model.AgentRun, _ model.Issue) {
			res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/run/%d/continue", run.IdRun), "", s.Token)
			s.Require().Equal(http.StatusOK, res.StatusCode, readBody(s.T(), res))
		}},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			iss := s.createIssue("cancelled-waiting-" + tc.name)
			run := s.startRun(iss.IdIssuePublic, tc.completedStages...)
			s.setPhase(run.IdRun, tc.phase)
			cancel := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/run/%d/cancel", run.IdRun), "", s.Token)
			s.Require().Equal(http.StatusOK, cancel.StatusCode)
			s.assignHuman(iss.IdIssuePublic)
			tasksBefore := len(s.tasksOf(run.IdRun))

			tc.resume(run, iss)

			s.Equal(1, s.runCount(iss.IdIssuePublic), "no new run")
			s.Equal(tc.phase, s.loadRun(run.IdRun).Phase, "continuing never approves nor answers for the person")
			s.Len(s.tasksOf(run.IdRun), tasksBefore)
			s.Empty(s.tick(run.IdRun))
		})
	}
}

func (s *AssignAgentSuite) TestSnapshotTellsWhetherTheAgentHoldsTheIssue() {
	iss := s.createIssue("snapshot-agent-assigned")
	run := s.startRun(iss.IdIssuePublic, allIssueStages...)
	s.setPhase(run.IdRun, constants.PhasePrOpen)

	held := s.snapshot(iss.IdIssuePublic)
	s.True(held.IsAgentAssigned)
	s.False(held.IsParked)

	s.assignHuman(iss.IdIssuePublic)

	reviewed := s.snapshot(iss.IdIssuePublic)
	s.False(reviewed.IsAgentAssigned, "a person reviews the PR: comments are not for the agent")
	s.False(reviewed.IsParked, "an open PR is never parked")
}

func Test_AssignAgentSuite(t *testing.T) {
	suite.Run(t, new(AssignAgentSuite))
}
