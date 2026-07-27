package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type AgentAuditSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	BotApiKey     string
	BotUserID     int64
	IdProject     int64
	IdIssuePublic int64
	GatewaySecret []byte
}

func (s *AgentAuditSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"auditbot","email":"auditbot@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"auditbot@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, loginRes.StatusCode)
	var tk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&tk)
	botToken := tk.Token

	botUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", botToken)
	var botUser model.User
	json.NewDecoder(botUserRes.Body).Decode(&botUser)
	s.BotUserID = botUser.IdUser

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE users.user SET is_bot = TRUE WHERE id_user = $1", s.BotUserID)
	s.Require().NoError(err)
	s.App.Cache.Del(context.Background(), botToken)

	keyRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", s.BotUserID),
		`{"name":"audit-bot-key"}`, s.Token)
	s.Require().Equal(http.StatusOK, keyRes.StatusCode)
	var apiKey model.CreateApiKeyRes
	json.NewDecoder(keyRes.Body).Decode(&apiKey)
	s.BotApiKey = apiKey.RawKey

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"audit-test-project","color":"#ccbbaa"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)

	var secret []byte
	err = s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.bot_gateway(id_user_bot, gateway_url, max_concurrent, webhook_secret)
		VALUES ($1, 'http://stub:9090', 1, decode(md5(random()::text), 'hex'))
		RETURNING webhook_secret`,
		s.BotUserID,
	).Scan(&secret)
	s.Require().NoError(err)
	s.GatewaySecret = secret

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"audit test issue","description":"audit test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *AgentAuditSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *AgentAuditSuite) insertQueuedRun() int64 {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, stage_plan)
		SELECT id_issue, $1, $2, 'queued', '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)
	return idRun
}

func (s *AgentAuditSuite) signedHeaders(body string) map[string]string {
	ts := time.Now().Unix()
	sig := agent.SignPayload(s.GatewaySecret, ts, []byte(body))
	return map[string]string{
		"Authorization":       s.BotApiKey,
		"X-Tracker-Signature": sig,
		"X-Tracker-Event-Id":  uuid.New().String(),
		"X-Tracker-Sequence":  "1",
	}
}

// Test_PhaseTransitionRecorded verifies that a phase transition records a run_event
// row with the correct from_phase/to_phase values. It drives a live user
// transition (queued → cancelled); the former gateway `/start` pickup callback
// was removed with the gateway-protocol rework, so this exercises a transition
// that still exists.
func (s *AgentAuditSuite) Test_PhaseTransitionRecorded() {
	idRun := s.insertQueuedRun()

	cancelRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/cancel", idRun), "{}", s.Token)
	s.Require().Equal(http.StatusOK, cancelRes.StatusCode)

	runRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/agent/run/%d", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, runRes.StatusCode)

	var runWithEvents model.AgentRunWithEvents
	json.NewDecoder(runRes.Body).Decode(&runWithEvents)

	s.Require().GreaterOrEqual(len(runWithEvents.Events), 1, "the transition must record a run_event")

	var found bool
	for _, ev := range runWithEvents.Events {
		if ev.ToPhase != nil && *ev.ToPhase == "cancelled" {
			s.Require().NotNil(ev.FromPhase, "the event must record the originating phase")
			s.Equal("queued", *ev.FromPhase, "from_phase must be the run's prior phase")
			found = true
		}
	}
	s.True(found, "queued→cancelled transition must be recorded with correct from/to phase")
}

// Test_ActorTypeResolved verifies that gateway-driven transitions set actor_type='gateway'
// and user-driven transitions set actor_type='user' with id_user populated.
func (s *AgentAuditSuite) Test_ActorTypeResolved() {
	idRun := s.insertQueuedRun()

	// Gateway-driven transition
	RequestWithHeaders(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/start", idRun), "",
		s.signedHeaders(""))

	// User-driven transition: cancel
	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/cancel", idRun), "{}", s.Token)

	runRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/agent/run/%d", idRun), "", s.Token)
	var runWithEvents model.AgentRunWithEvents
	json.NewDecoder(runRes.Body).Decode(&runWithEvents)

	for _, ev := range runWithEvents.Events {
		if ev.ToPhase != nil && *ev.ToPhase == "pickup" {
			s.Equal("gateway", ev.ActorType, "gateway transition must use actor_type=gateway")
			s.Nil(ev.IdUser, "gateway transitions must not set id_user")
		}
		if ev.ToPhase != nil && *ev.ToPhase == "cancelled" {
			s.Equal("user", ev.ActorType, "user cancel must use actor_type=user")
			s.NotNil(ev.IdUser, "user transitions must set id_user")
		}
	}
}

func Test_RunAgentAuditSuite(t *testing.T) {
	suite.Run(t, new(AgentAuditSuite))
}
