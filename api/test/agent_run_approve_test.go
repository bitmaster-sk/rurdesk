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

// AgentRunApproveSuite covers approving a design stage that embedded multiple
// mockups: the approve request may carry a mockupRef identifying the chosen
// variant, which must be persisted on the run. A plain approve (no ref) must
// keep working unchanged.
type AgentRunApproveSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	IdProject     int64
	IdIssuePublic int64
}

func (s *AgentRunApproveSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"approve-test-project","color":"#abcdef"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"approve issue","description":"body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *AgentRunApproveSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

// insertAwaitingApprovalRun seeds a single run parked in awaiting_approval so
// the approve endpoint accepts it.
func (s *AgentRunApproveSuite) insertAwaitingApprovalRun() int64 {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, stage_plan)
		SELECT id_issue, $1, $2, 'awaiting_approval', '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.currentUserID(), s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)
	return idRun
}

func (s *AgentRunApproveSuite) currentUserID() int64 {
	res := Request(s.T(), s.App, "GET", "/api/private/user", "", s.Token)
	var user model.User
	json.NewDecoder(res.Body).Decode(&user)
	return user.IdUser
}

func (s *AgentRunApproveSuite) Test_ApproveWithMockupRefPersistsIt() {
	idRun := s.insertAwaitingApprovalRun()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/approve", idRun),
		`{"mockupRef":"Mockup B"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var run model.AgentRun
	json.NewDecoder(res.Body).Decode(&run)
	s.Require().NotNil(run.ApprovedMockupRef, "approvedMockupRef must be set")
	s.Equal("Mockup B", *run.ApprovedMockupRef)

	// And it must survive a reload from the DB.
	runRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/agent/run/%d", idRun), "", s.Token)
	var reloaded model.AgentRunWithEvents
	json.NewDecoder(runRes.Body).Decode(&reloaded)
	s.Require().NotNil(reloaded.ApprovedMockupRef)
	s.Equal("Mockup B", *reloaded.ApprovedMockupRef)
}

func (s *AgentRunApproveSuite) Test_ApproveWithoutMockupRefStillWorks() {
	idRun := s.insertAwaitingApprovalRun()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/approve", idRun),
		`{}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var run model.AgentRun
	json.NewDecoder(res.Body).Decode(&run)
	s.Equal("in_progress", run.Phase)
	s.Nil(run.ApprovedMockupRef, "no ref chosen → approvedMockupRef stays nil")
}

func Test_RunAgentRunApproveSuite(t *testing.T) {
	suite.Run(t, new(AgentRunApproveSuite))
}
