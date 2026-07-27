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

type BotGatewaySuite struct {
	suite.Suite
	App       *issue.Application
	Token     string // bootstrap admin
	UserToken string // non-admin human
	BotUserID int64
	HumanID   int64
	IdProject int64
}

func (s *BotGatewaySuite) createBot(name string) int64 {
	res := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":%q,"isBot":true}`, name), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	return created.IdUser
}

func (s *BotGatewaySuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	s.BotUserID = s.createBot("gwbot")

	s.UserToken = createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"gwhuman","email":"gwhuman@test.sk","password":"kreslo"}`)
	s.HumanID = idOfUser(s.T(), s.App, s.Token, "gwhuman@test.sk")

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"gw-test-project","color":"#aabbcc"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)
}

func (s *BotGatewaySuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.bot_gateway WHERE id_user_bot = $1", s.BotUserID)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user IN ($1, $2)", s.BotUserID, s.HumanID)
}

func (s *BotGatewaySuite) gatewayPath(idUser int64) string {
	return fmt.Sprintf("/api/private/admin/user/%d/gateway", idUser)
}

// Assigning a gateway-less bot to an issue must be rejected with 422 so the
// issue is never committed with an assignee that can never dispatch a run.
func (s *BotGatewaySuite) TestAssignGatewaylessBotRejects() {
	nogwBotID := s.createBot("nogwbot")
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", nogwBotID)

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, nogwBotID), s.Token)

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"gw-test issue","description":"gw-test","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(issueRes.Body).Decode(&iss))

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic),
		fmt.Sprintf(`{"title":"gw-test issue","description":"gw-test","assignedTo":%d,"estimated":0}`, nogwBotID),
		s.Token)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)

	getRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic), "", s.Token)
	s.Require().Equal(http.StatusOK, getRes.StatusCode)
	var updated model.Issue
	s.Require().NoError(json.NewDecoder(getRes.Body).Decode(&updated))
	s.Nil(updated.AssignedTo, "issue must remain unassigned after rejected gateway-less bot assignment")
}

// Gateway management is instance-admin only.
func (s *BotGatewaySuite) TestNonAdminForbidden() {
	path := s.gatewayPath(s.BotUserID)
	body := `{"gatewayUrl":"http://gw:9090"}`

	s.Equal(http.StatusForbidden, Request(s.T(), s.App, "GET", path, "", s.UserToken).StatusCode)
	s.Equal(http.StatusForbidden, Request(s.T(), s.App, "POST", path, body, s.UserToken).StatusCode)
	s.Equal(http.StatusForbidden, Request(s.T(), s.App, "POST", path+"/token", "", s.UserToken).StatusCode)
	s.Equal(http.StatusForbidden, Request(s.T(), s.App, "DELETE", path, "", s.UserToken).StatusCode)
}

// Gateways exist only for bot users.
func (s *BotGatewaySuite) TestGatewayForNonBotRejected() {
	res := Request(s.T(), s.App, "POST", s.gatewayPath(s.HumanID),
		`{"gatewayUrl":"http://gw:9090"}`, s.Token)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

// Full lifecycle: create (one-time token, default concurrency), read, duplicate
// rejection, token regeneration, delete, and 404s on the empty state.
func (s *BotGatewaySuite) TestGatewayLifecycle() {
	path := s.gatewayPath(s.BotUserID)
	body := `{"gatewayUrl":"http://gw:9090"}`

	createRes := Request(s.T(), s.App, "POST", path, body, s.Token)
	s.Require().Equal(http.StatusOK, createRes.StatusCode)
	var created model.CreateBotGatewayRes
	s.Require().NoError(json.NewDecoder(createRes.Body).Decode(&created))
	s.Len(created.TrackerToGatewayToken, 64, "one-time token must be 32 hex-encoded bytes")
	s.Equal(1, created.MaxConcurrent, "concurrency must default to the safe single-run value")
	s.Equal(s.BotUserID, created.IdUserBot)

	getRes := Request(s.T(), s.App, "GET", path, "", s.Token)
	s.Require().Equal(http.StatusOK, getRes.StatusCode)
	var loaded model.BotGateway
	s.Require().NoError(json.NewDecoder(getRes.Body).Decode(&loaded))
	s.Equal(created.IdBotGateway, loaded.IdBotGateway)
	s.Equal("http://gw:9090", loaded.GatewayUrl)

	dupRes := Request(s.T(), s.App, "POST", path, body, s.Token)
	s.Equal(http.StatusConflict, dupRes.StatusCode, "a bot has exactly one gateway")

	regenRes := Request(s.T(), s.App, "POST", path+"/token", "", s.Token)
	s.Require().Equal(http.StatusOK, regenRes.StatusCode)
	var regen model.CreateBotGatewayRes
	s.Require().NoError(json.NewDecoder(regenRes.Body).Decode(&regen))
	s.Len(regen.TrackerToGatewayToken, 64)
	s.NotEqual(created.TrackerToGatewayToken, regen.TrackerToGatewayToken,
		"regeneration must invalidate the old token")
	s.Equal(created.IdBotGateway, regen.IdBotGateway, "regeneration keeps the gateway record")

	s.Equal(http.StatusOK, Request(s.T(), s.App, "DELETE", path, "", s.Token).StatusCode)

	emptyRes := Request(s.T(), s.App, "GET", path, "", s.Token)
	s.Require().Equal(http.StatusOK, emptyRes.StatusCode)
	s.Equal("null", readBody(s.T(), emptyRes), "no gateway reads as null")

	s.Equal(http.StatusNotFound, Request(s.T(), s.App, "DELETE", path, "", s.Token).StatusCode)
	s.Equal(http.StatusNotFound, Request(s.T(), s.App, "POST", path+"/token", "", s.Token).StatusCode)
}

func TestBotGatewaySuite(t *testing.T) {
	suite.Run(t, new(BotGatewaySuite))
}
