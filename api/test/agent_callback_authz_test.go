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

// AgentCallbackAuthzSuite pins the authorization boundary on the gateway
// callbacks. They live in the ordinary authenticated route group, so the only
// thing standing between a logged-in user and another project's agent run is the
// per-handler bot-ownership check. A regression here lets any user complete
// stages, post messages authored as the agent, and trigger PR creation on runs
// they do not own — so each callback is asserted individually rather than
// trusting a shared middleware.
type AgentCallbackAuthzSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string // ordinary (non-bot) user: the attacker in these tests
	BotApiKey     string
	BotUserID     int64
	IdProject     int64
	IdIssuePublic int64
}

func (s *AgentCallbackAuthzSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"authzbot","email":"authzbot@test.sk","password":"kreslo"}`, s.Token)
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"authzbot@test.sk","password":"kreslo"}`, "")
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
		`{"name":"authz-bot-key"}`, s.Token)
	s.Require().Equal(http.StatusOK, keyRes.StatusCode)
	var apiKey model.CreateApiKeyRes
	json.NewDecoder(keyRes.Body).Decode(&apiKey)
	s.BotApiKey = apiKey.RawKey

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"authz-test-project","color":"#ccbbaa"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"authz test issue","description":"authz test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *AgentCallbackAuthzSuite) TearDownSuite() {
	s.purgeRuns()
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

// purgeRuns clears the project's runs. Tasks go first: they reference the run,
// and a partial-unique index allows only one active run per issue — a silently
// failed delete would surface as a confusing constraint violation on the next
// insert rather than here.
func (s *AgentCallbackAuthzSuite) purgeRuns() {
	_, err := s.App.Pool.Exec(context.Background(), `
		DELETE FROM agent.task
		WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)`,
		s.IdProject)
	s.Require().NoError(err)

	_, err = s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.Require().NoError(err)
}

// insertRunWithActiveTask creates a run owned by the bot plus one active task on
// it, the state the gateway callbacks expect to address.
func (s *AgentCallbackAuthzSuite) insertRunWithActiveTask() (idRun int64, idTask int64) {
	s.purgeRuns()

	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, stage_plan)
		SELECT id_issue, $1, $2, 'in_progress', '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)

	err = s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.task(id_run, id_user_bot, stage, attempt_no, status)
		VALUES ($1, $2, 'implementation', 1, 'active')
		RETURNING id_task`,
		idRun, s.BotUserID,
	).Scan(&idTask)
	s.Require().NoError(err)

	return idRun, idTask
}

// Test_ForeignUserCannotDriveRun is the regression guard: every gateway callback
// must reject a perfectly valid session belonging to someone who is not the
// run's bot. The user here is the project's own admin — proving that project
// membership, and even ownership, is not sufficient.
func (s *AgentCallbackAuthzSuite) Test_ForeignUserCannotDriveRun() {
	idRun, idTask := s.insertRunWithActiveTask()

	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{
			name:   "complete stage",
			method: "POST",
			url:    fmt.Sprintf("/api/private/agent/task/%d/complete", idTask),
			body:   `{"outcome":"output_submitted","message":"pwned"}`,
		},
		{
			name:   "task heartbeat",
			method: "POST",
			url:    fmt.Sprintf("/api/private/agent/task/%d/heartbeat", idTask),
			body:   "",
		},
		{
			name:   "task stats",
			method: "POST",
			url:    fmt.Sprintf("/api/private/agent/task/%d/stats", idTask),
			body:   `{"tokensUsed":999999}`,
		},
		{
			name:   "task thinking",
			method: "POST",
			url:    fmt.Sprintf("/api/private/agent/task/%d/thinking", idTask),
			body:   `{"seq":1,"events":[{"kind":"thinking","text":"injected","at":1}]}`,
		},
		{
			name:   "report run repo",
			method: "POST",
			url:    fmt.Sprintf("/api/private/agent/run/%d/repo", idRun),
			body:   `{"repoPath":"attacker/repo"}`,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			res := Request(s.T(), s.App, tt.method, tt.url, tt.body, s.Token)
			s.Equal(http.StatusForbidden, res.StatusCode,
				"a non-bot user must not reach %s", tt.url)
		})
	}
}

// A task that does not exist must read as not found, not as a permission
// problem — otherwise a typo in a task id looks like a revoked agent.
func (s *AgentCallbackAuthzSuite) Test_UnknownTaskIsNotFound() {
	res := Request(s.T(), s.App, "POST",
		"/api/private/agent/task/99999999/thinking",
		`{"seq":1,"events":[{"kind":"thinking","text":"nowhere","at":1}]}`, s.BotApiKey)

	s.Equal(http.StatusNotFound, res.StatusCode)
}

// Test_ForeignUserCannotCompleteStage_LeavesRunUntouched proves the rejection is
// a real no-op, not a late failure after the run has already been mutated.
func (s *AgentCallbackAuthzSuite) Test_ForeignUserCannotCompleteStage_LeavesRunUntouched() {
	idRun, idTask := s.insertRunWithActiveTask()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/complete", idTask),
		`{"outcome":"output_submitted","message":"pwned"}`, s.Token)
	s.Require().Equal(http.StatusForbidden, res.StatusCode)

	var phase, status string
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT r.phase, t.status
		FROM agent.run r JOIN agent.task t ON t.id_run = r.id_run
		WHERE r.id_run = $1 AND t.id_task = $2`,
		idRun, idTask,
	).Scan(&phase, &status)
	s.Require().NoError(err)

	s.Equal("in_progress", phase, "the rejected call must not transition the run")
	s.Equal("active", status, "the rejected call must not close the task")
}

// Test_OwningBotStillWorks guards the other direction: the ownership check must
// not lock out the gateway it exists to protect.
func (s *AgentCallbackAuthzSuite) Test_OwningBotStillWorks() {
	idRun, idTask := s.insertRunWithActiveTask()

	beat := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/heartbeat", idTask), "", s.BotApiKey)
	s.Equal(http.StatusOK, beat.StatusCode, "the run's own bot must be able to heartbeat")

	stats := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/stats", idTask), `{"tokensUsed":42}`, s.BotApiKey)
	s.Equal(http.StatusOK, stats.StatusCode, "the run's own bot must be able to report stats")

	repo := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/repo", idRun), `{"repoPath":"owner/repo"}`, s.BotApiKey)
	s.Equal(http.StatusOK, repo.StatusCode, "the run's own bot must be able to report its repo")
}

func TestAgentCallbackAuthzSuite(t *testing.T) {
	suite.Run(t, new(AgentCallbackAuthzSuite))
}
