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

type AssignBotSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	BotUserID int64
	HumanID   int64
	IdProject int64
	IdSkill   int64
}

func (s *AssignBotSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	seedBuiltinSkills(s.T())

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"assignbot","isBot":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var bot struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&bot))
	s.BotUserID = bot.IdUser

	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"assignhuman","email":"assignhuman@test.sk","password":"kreslo1"}`)
	s.HumanID = idOfUser(s.T(), s.App, s.Token, "assignhuman@test.sk")

	s.IdProject = createProject(s.T(), s.App, s.Token, "assign-agent-project")
	for _, idUser := range []int64{s.BotUserID, s.HumanID} {
		member := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.Token)
		s.Require().Equal(http.StatusOK, member.StatusCode)
	}

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", s.BotUserID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	s.IdSkill = skillByName(s.T(), listSkills(s.T(), s.App, s.Token), "PR rules").IdSkill
}

func (s *AssignBotSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.bot_gateway WHERE id_user_bot = $1", s.BotUserID)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user IN ($1, $2)", s.BotUserID, s.HumanID)
}

func (s *AssignBotSuite) SetupTest() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx,
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

func (s *AssignBotSuite) createIssue(title string) model.Issue {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))
	return iss
}

func (s *AssignBotSuite) assignBot(idIssuePublic int64, body string, token string) *http.Response {
	return Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", s.IdProject, idIssuePublic),
		body, token)
}

func (s *AssignBotSuite) runCount(idIssuePublic int64) int {
	var count int
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent.run r
		JOIN issues.issue i ON i.id_issue = r.id_issue
		WHERE i.id_issue_public = $1 AND i.id_project = $2`,
		idIssuePublic, s.IdProject).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *AssignBotSuite) TestAssignCreatesExactlyOneRunWithRequestedSkills() {
	iss := s.createIssue("assign-happy")

	res := s.assignBot(iss.IdIssuePublic,
		fmt.Sprintf(`{"idUserBot":%d,"idsSkillByStage":{"implementation":[%d]}}`, s.BotUserID, s.IdSkill),
		s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&run))

	var plan model.StagePlan
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
	s.Equal(s.BotUserID, *reloaded.AssignedTo)
}

func (s *AssignBotSuite) TestSecondAssignWhileActiveConflicts() {
	iss := s.createIssue("assign-twice")

	first := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, s.BotUserID), s.Token)
	s.Require().Equal(http.StatusOK, first.StatusCode)

	second := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, s.BotUserID), s.Token)
	s.Equal(http.StatusConflict, second.StatusCode)
	s.Equal(1, s.runCount(iss.IdIssuePublic))
}

// The active-run conflict is refused before the transaction opens, so this pins
// the guard, not the 23505 rollback behind it — that needs a real race.
func (s *AssignBotSuite) TestRefusedAssignLeavesTheAssigneeUntouched() {
	iss := s.createIssue("assign-conflict-rollback")

	first := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, s.BotUserID), s.Token)
	s.Require().Equal(http.StatusOK, first.StatusCode)

	edit := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic),
		fmt.Sprintf(`{"idProject":%d,"assignedTo":%d}`, s.IdProject, s.HumanID), s.Token)
	s.Require().Equal(http.StatusOK, edit.StatusCode)

	second := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, s.BotUserID), s.Token)
	s.Require().Equal(http.StatusConflict, second.StatusCode)

	issRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic), "", s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var reloaded model.Issue
	s.Require().NoError(json.NewDecoder(issRes.Body).Decode(&reloaded))
	s.Require().NotNil(reloaded.AssignedTo)
	s.Equal(s.HumanID, *reloaded.AssignedTo, "a refused assign must not change the assignee")
}

func (s *AssignBotSuite) TestAssignAgentFromAnotherProjectRejected() {
	outsideBot := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"assignoutsidebot","isBot":true}`, s.Token)
	s.Require().Equal(http.StatusOK, outsideBot.StatusCode)
	var outsider struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(outsideBot.Body).Decode(&outsider))
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", outsider.IdUser)

	gw := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", outsider.IdUser),
		`{"gatewayUrl":"http://gw:9091"}`, s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)

	iss := s.createIssue("assign-outside-agent")
	res := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, outsider.IdUser), s.Token)

	s.Equal(http.StatusForbidden, res.StatusCode, "an agent that cannot read the project cannot be assigned to its issues")
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignBotSuite) TestAssignHumanRejected() {
	iss := s.createIssue("assign-human")

	res := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, s.HumanID), s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignBotSuite) TestAssignUnknownStageRejected() {
	iss := s.createIssue("assign-bad-stage")

	res := s.assignBot(iss.IdIssuePublic,
		fmt.Sprintf(`{"idUserBot":%d,"idsSkillByStage":{"pickup":[%d]}}`, s.BotUserID, s.IdSkill), s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignBotSuite) TestAssignWithoutAclForbidden() {
	iss := s.createIssue("assign-acl")

	outsiderToken := createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"assign-outsider","email":"assign-outsider@test.sk","password":"kreslo1"}`)

	res := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, s.BotUserID), outsiderToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func (s *AssignBotSuite) TestAssignBotWithoutGatewayRejected() {
	iss := s.createIssue("assign-no-gw")

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"gatewayless","isBot":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var bot struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&bot))
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", bot.IdUser)

	member := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, bot.IdUser), s.Token)
	s.Require().Equal(http.StatusOK, member.StatusCode, "membership must pass so the gateway check is what refuses")

	assign := s.assignBot(iss.IdIssuePublic, fmt.Sprintf(`{"idUserBot":%d}`, bot.IdUser), s.Token)
	s.Equal(http.StatusUnprocessableEntity, assign.StatusCode)
	s.Equal(0, s.runCount(iss.IdIssuePublic))
}

func Test_AssignBotSuite(t *testing.T) {
	suite.Run(t, new(AssignBotSuite))
}
