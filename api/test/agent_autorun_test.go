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

// AgentAutoRunSuite covers: assigning an issue to an agent (with a configured
// gateway) must trigger an agent run. Before the fix only the single-issue edit
// path (EditIssue) triggered a run; creating an issue already assigned to an agent,
// or assigning an agent via bulk-edit, saved the assignment but silently started no
// run — no automation, no error, no feedback.
type AgentAutoRunSuite struct {
	suite.Suite
	App         *issue.Application
	Token       string
	AgentUserID int64
	HumanID     int64
	IdProject   int64
}

func (s *AgentAutoRunSuite) createAgent(name string) int64 {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isBot":true}`, name), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	return created.IdUser
}

func (s *AgentAutoRunSuite) addMember(idUser int64) {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

// runCountForIssue counts agent runs attached to the issue with the given public id.
func (s *AgentAutoRunSuite) runCountForIssue(idIssuePublic int64) int {
	var count int
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent.run r
		JOIN issues.issue i ON i.id_issue = r.id_issue
		WHERE i.id_issue_public = $1 AND i.id_project = $2`,
		idIssuePublic, s.IdProject,
	).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *AgentAutoRunSuite) createIssue(body string) model.Issue {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject), body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))
	return iss
}

func (s *AgentAutoRunSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	s.AgentUserID = s.createAgent("autorunbot")

	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"autorunhuman","email":"autorunhuman@test.sk","password":"kreslo"}`)
	s.HumanID = idOfUser(s.T(), s.App, s.Token, "autorunhuman@test.sk")

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"autorun-project","color":"#123456"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	s.addMember(s.AgentUserID)
	s.addMember(s.HumanID)

	// The agent needs a gateway or no run can ever dispatch.
	gwRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.AgentUserID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Require().Equal(http.StatusOK, gwRes.StatusCode)
}

func (s *AgentAutoRunSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.bot_gateway WHERE id_user_bot = $1", s.AgentUserID)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user IN ($1, $2)", s.AgentUserID, s.HumanID)
}

func (s *AgentAutoRunSuite) SetupTest() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

// Creating an issue already assigned to an agent starts a run.
func (s *AgentAutoRunSuite) TestCreateAssignedToAgentStartsRun() {
	iss := s.createIssue(fmt.Sprintf(
		`{"title":"create-bot","description":"body","assignedTo":%d,"estimated":0}`, s.AgentUserID))
	s.Equal(1, s.runCountForIssue(iss.IdIssuePublic),
		"creating an issue assigned to a bot must start exactly one run")
}

// Assigning an agent via bulk-edit starts a run.
func (s *AgentAutoRunSuite) TestBulkAssignToAgentStartsRun() {
	iss := s.createIssue(`{"title":"bulk-bot","description":"body","estimated":0}`)
	s.Require().Equal(0, s.runCountForIssue(iss.IdIssuePublic), "unassigned issue must start with no run")

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/batch", s.IdProject),
		fmt.Sprintf(`{"issues":[{"idIssuePublic":%d,"idUserAssigned":%d}]}`, iss.IdIssuePublic, s.AgentUserID),
		s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal(1, s.runCountForIssue(iss.IdIssuePublic),
		"bulk-assigning a bot must start exactly one run")
}

// Control: assigning a human (not an agent) must never start a run.
func (s *AgentAutoRunSuite) TestCreateAssignedToHumanNoRun() {
	iss := s.createIssue(fmt.Sprintf(
		`{"title":"create-human","description":"body","assignedTo":%d,"estimated":0}`, s.HumanID))
	s.Equal(0, s.runCountForIssue(iss.IdIssuePublic),
		"assigning a human must not start a run")
}

func TestAgentAutoRunSuite(t *testing.T) {
	suite.Run(t, new(AgentAutoRunSuite))
}
