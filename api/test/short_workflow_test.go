package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type ShortWorkflowSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	AgentUserID int64
	OutsiderID  int64
	IdProject   int64
}

func (s *ShortWorkflowSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"shortworkflowbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&agent))
	s.AgentUserID = agent.IdUser

	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"shortoutsider","email":"shortoutsider@test.sk","password":"kreslo1"}`)
	s.OutsiderID = idOfUser(s.T(), s.App, s.Token, "shortoutsider@test.sk")

	s.IdProject = createProject(s.T(), s.App, s.Token, "short-workflow-project")

	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.AgentUserID), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode)

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)
}

func (s *ShortWorkflowSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", s.AgentUserID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user IN ($1, $2)", s.AgentUserID, s.OutsiderID)
}

func (s *ShortWorkflowSuite) SetupTest() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

func (s *ShortWorkflowSuite) assignAgent(title string, isShortWorkflow bool) model.AgentRun {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))

	assign := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", s.IdProject, iss.IdIssuePublic),
		fmt.Sprintf(`{"idUserAgent":%d,"isShortWorkflow":%t}`, s.AgentUserID, isShortWorkflow), s.Token)
	s.Require().Equal(http.StatusOK, assign.StatusCode)
	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(assign.Body).Decode(&run))
	return run
}

func (s *ShortWorkflowSuite) patchWorkflow(idRun int64, isShortWorkflow bool, token string) *http.Response {
	return Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/agent/run/%d/workflow", idRun),
		fmt.Sprintf(`{"isShortWorkflow":%t}`, isShortWorkflow), token)
}

func (s *ShortWorkflowSuite) addTask(idRun int64, stage, status string) {
	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO agent.task (id_run, stage, attempt_no, status)
		VALUES ($1, $2, 1, $3)`, idRun, stage, status)
	s.Require().NoError(err)
}

func timelineStatuses(snapshot model.AgentRunWithEvents) map[string]string {
	statuses := map[string]string{}
	for _, stage := range snapshot.Stages {
		statuses[stage.Stage] = stage.Status
	}
	return statuses
}

func (s *ShortWorkflowSuite) TestAssignWithShortWorkflowSkipsDesignAndPlan() {
	run := s.assignAgent("short-assign", true)

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&snapshot))

	statuses := timelineStatuses(snapshot)
	s.Equal("skipped", statuses["design"])
	s.Equal("skipped", statuses["implementation_plan"])
	s.Equal("pending", statuses["brainstorming"])
	s.Equal("pending", statuses["implementation"])
}

func (s *ShortWorkflowSuite) TestAssignWithoutFlagKeepsFullWorkflow() {
	run := s.assignAgent("full-assign", false)

	var plan model.AgentStagePlan
	s.Require().NoError(json.Unmarshal(run.StagePlan, &plan))
	for _, entry := range plan.Stages {
		s.False(entry.Skip, "stage %s must run in the full workflow", entry.Name)
	}
}

func (s *ShortWorkflowSuite) TestPatchTogglesWorkflowBeforeDesign() {
	run := s.assignAgent("short-toggle", false)
	s.addTask(run.IdRun, constants.StageBrainstorming, constants.TaskStatusActive)

	on := s.patchWorkflow(run.IdRun, true, s.Token)
	s.Require().Equal(http.StatusOK, on.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(on.Body).Decode(&snapshot))
	s.Equal("skipped", timelineStatuses(snapshot)["design"])

	off := s.patchWorkflow(run.IdRun, false, s.Token)
	s.Require().Equal(http.StatusOK, off.StatusCode)
	s.Require().NoError(json.NewDecoder(off.Body).Decode(&snapshot))
	s.Equal("pending", timelineStatuses(snapshot)["design"])
}

func (s *ShortWorkflowSuite) TestPatchLocksOnceALaterStageRan() {
	tests := []struct {
		name             string
		isShortAtAssign  bool
		stage            string
		taskStatus       string
		isShortRequested bool
		expectedStatus   int
	}{
		{"design started", false, constants.StageDesign, constants.TaskStatusActive, true, http.StatusConflict},
		{"plan started", false, constants.StageImplementationPlan, constants.TaskStatusActive, true, http.StatusConflict},
		{"implementation started in a short run", true, constants.StageImplementation, constants.TaskStatusActive, false, http.StatusConflict},
		{"design failed", false, constants.StageDesign, constants.TaskStatusFailed, true, http.StatusConflict},
		{"design cancelled", false, constants.StageDesign, constants.TaskStatusCancelled, true, http.StatusOK},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			run := s.assignAgent("short-lock-"+tc.name, tc.isShortAtAssign)
			s.addTask(run.IdRun, tc.stage, tc.taskStatus)

			res := s.patchWorkflow(run.IdRun, tc.isShortRequested, s.Token)
			s.Equal(tc.expectedStatus, res.StatusCode)
		})
	}
}

func (s *ShortWorkflowSuite) TestPatchForbiddenOutsideTheProject() {
	run := s.assignAgent("short-forbidden", false)
	outsiderToken := loginUser(s.T(), s.App, "shortoutsider@test.sk", "kreslo1")

	res := s.patchWorkflow(run.IdRun, true, outsiderToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *ShortWorkflowSuite) TestSchedulerGoesFromBrainstormingToImplementation() {
	run := s.assignAgent("short-scheduler", true)
	s.addTask(run.IdRun, constants.StagePickup, constants.TaskStatusCompleted)
	s.addTask(run.IdRun, constants.StageBrainstorming, constants.TaskStatusCompleted)

	dispatcher := &stubStageDispatcher{}
	sched := agent.NewScheduler(s.App.Pool, injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(), injector.GetIssueRepository(), dispatcher, injector.GetNotifier())
	s.Require().NoError(sched.TickOnce(context.Background()))

	var dispatchedStages []string
	for _, task := range dispatcher.dispatched {
		if task.IdRun == run.IdRun {
			dispatchedStages = append(dispatchedStages, task.Stage)
		}
	}
	s.Equal([]string{constants.StageImplementation}, dispatchedStages)
}

func (s *ShortWorkflowSuite) TestRestartKeepsShortWorkflow() {
	run := s.assignAgent("short-restart", true)

	restart := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/restart", run.IdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, restart.StatusCode)
	var ids struct {
		NewIdRun int64 `json:"newIdRun"`
	}
	s.Require().NoError(json.NewDecoder(restart.Body).Decode(&ids))

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d", ids.NewIdRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&snapshot))
	s.Equal("skipped", timelineStatuses(snapshot)["design"])
	s.Equal("skipped", timelineStatuses(snapshot)["implementation_plan"])
}

func Test_ShortWorkflowSuite(t *testing.T) {
	suite.Run(t, new(ShortWorkflowSuite))
}
