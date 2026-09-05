package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type MergePollerSuite struct {
	suite.Suite
	App              *issue.Application
	Token            string
	BotToken         string
	UserID           int64
	BotUserID        int64
	IdProject        int64
	IdIssuePublic    int64
	IdGitIntegration int64
	poller           *agent.MergePoller
	mockServer       *httptest.Server
	mockState        string // "merged" | "closed" | "open"
}

func (s *MergePollerSuite) SetupSuite() {
	os.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	githost.ResetEncryptionKey()

	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.Token)
	var user model.User
	json.NewDecoder(userRes.Body).Decode(&user)
	s.UserID = user.IdUser

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"mergepollerbot","email":"mergepollerbot@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"mergepollerbot@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, loginRes.StatusCode)
	var tk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&tk)
	s.BotToken = tk.Token

	botUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.BotToken)
	var botUser model.User
	json.NewDecoder(botUserRes.Body).Decode(&botUser)
	s.BotUserID = botUser.IdUser

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE users.user SET is_bot = TRUE WHERE id_user = $1", s.BotUserID)
	s.Require().NoError(err)
	s.App.Cache.Del(context.Background(), s.BotToken)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"merge-poller-test-project","color":"#334455"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	addRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)
	s.Require().Equal(http.StatusOK, addRes.StatusCode)

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"merge poller test issue","description":"merge poller test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic

	// Mock server that serves GitHub API responses based on s.mockState.
	// GitHub host constructs apiBase as {baseUrl}/api/v3 for non-github.com URLs.
	s.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/reviews"):
			// Approval check — return empty list.
			fmt.Fprint(w, `[]`)

		case strings.Contains(path, "/commits/") && strings.HasSuffix(path, "/status"):
			// CI status — return success.
			fmt.Fprint(w, `{"state":"success"}`)

		default:
			// PR status endpoint.
			state := s.mockState
			if state == "" {
				state = "open"
			}
			switch state {
			case "merged":
				fmt.Fprint(w, `{"state":"closed","merged":true,"head":{"sha":"abc123"}}`)
			case "closed":
				fmt.Fprint(w, `{"state":"closed","merged":false,"head":{"sha":"abc123"}}`)
			default:
				fmt.Fprint(w, `{"state":"open","merged":false,"head":{"sha":"abc123"}}`)
			}
		}
	}))

	// Create git integration pointing at the mock server.
	intBody := fmt.Sprintf(
		`{"name":"mock-git","hostType":"github","baseUrl":%q,"repoPath":"org/repo","accessToken":"ghp_mock_token"}`,
		s.mockServer.URL,
	)
	intRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.IdProject),
		intBody, s.Token)
	s.Require().Equal(http.StatusCreated, intRes.StatusCode)
	var gitInt model.GitIntegrationRes
	json.NewDecoder(intRes.Body).Decode(&gitInt)
	s.IdGitIntegration = gitInt.IdGitIntegration

	s.poller = agent.NewMergePoller(
		injector.GetAgentRunRepository(),
		injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(),
		injector.GetGitIntegrationRepository(),
		injector.GetIssueRepository(),
		injector.GetStateRepository(),
		injector.GetPhaseStateTransitioner(),
		injector.GetNotifier(),
	)
}

func (s *MergePollerSuite) TearDownSuite() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *MergePollerSuite) insertPrOpenRun() int64 {
	return s.insertPrOpenRunWithIntegration(s.IdGitIntegration)
}

func (s *MergePollerSuite) insertPrOpenRunWithIntegration(idGitIntegration int64) int64 {
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, pr_id, pr_host_type, pr_url, branch_name, id_git_integration, stage_plan)
		SELECT id_issue, $1, $2, 'pr_open', '42', 'github', 'https://github.com/org/repo/pull/42',
		       'agent/b1/i1/123456', $3, '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $4 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, idGitIntegration, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)
	return idRun
}

func (s *MergePollerSuite) cleanupRuns() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

func (s *MergePollerSuite) loadRun(idRun int64) *model.AgentRun {
	var run model.AgentRun
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT id_run, id_issue, id_user_bot, id_project, id_git_integration,
		       phase, stage_plan, queue_position,
		       pr_url, pr_host_type, pr_id, branch_name, error_message,
		       started_at, finished_at, created_at
		FROM agent.run WHERE id_run = $1`, idRun,
	).Scan(
		&run.IdRun, &run.IdIssue, &run.IdUserBot, &run.IdProject, &run.IdGitIntegration,
		&run.Phase, &run.StagePlan, &run.QueuePosition,
		&run.PrUrl, &run.PrHostType, &run.PrId, &run.BranchName, &run.ErrorMessage,
		&run.StartedAt, &run.FinishedAt, &run.CreatedAt,
	)
	s.Require().NoError(err)
	return &run
}

func (s *MergePollerSuite) countRunEvents(idRun int64) int {
	var count int
	s.App.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM agent.run_event WHERE id_run = $1", idRun,
	).Scan(&count)
	return count
}

func (s *MergePollerSuite) Test_Merged() {
	s.cleanupRuns()
	s.mockState = "merged"
	idRun := s.insertPrOpenRun()

	err := s.poller.PollOnce(context.Background())
	s.Require().NoError(err)

	run := s.loadRun(idRun)
	// A merged PR advances the run to the terminal "done" phase (merge_poller
	// transitions pr_open → PhaseDone with reason "PR merged"); there is no
	// separate "merged" phase.
	s.Equal("done", run.Phase)
	s.NotNil(run.FinishedAt)
	s.Greater(s.countRunEvents(idRun), 0)
}

func (s *MergePollerSuite) Test_ClosedWithoutMerge() {
	s.cleanupRuns()
	s.mockState = "closed"
	idRun := s.insertPrOpenRun()

	err := s.poller.PollOnce(context.Background())
	s.Require().NoError(err)

	run := s.loadRun(idRun)
	s.Equal("failed", run.Phase)
	s.Require().NotNil(run.ErrorMessage)
	s.Equal("PR closed without merge", *run.ErrorMessage)
	s.Greater(s.countRunEvents(idRun), 0)
}

func (s *MergePollerSuite) Test_StillOpen_NoChange() {
	s.cleanupRuns()
	s.mockState = "open"
	idRun := s.insertPrOpenRun()

	err := s.poller.PollOnce(context.Background())
	s.Require().NoError(err)

	run := s.loadRun(idRun)
	s.Equal("pr_open", run.Phase)
}

func (s *MergePollerSuite) Test_PollsOnlyPrOpen() {
	s.cleanupRuns()
	s.mockState = "merged"

	// Insert a run in 'implementing' phase with pr_id set.
	// LoadPrOpenRuns only fetches phase='pr_open', so this run must not be transitioned.
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, pr_id, pr_host_type, pr_url, branch_name, id_git_integration, stage_plan)
		SELECT id_issue, $1, $2, 'implementing', '42', 'github', 'https://github.com/org/repo/pull/42',
		       'agent/b1/i1/123456', $3, '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $4 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdGitIntegration, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)

	err = s.poller.PollOnce(context.Background())
	s.Require().NoError(err)

	run := s.loadRun(idRun)
	s.Equal("implementing", run.Phase, "poller must not transition non-pr_open runs")
}

func (s *MergePollerSuite) Test_NoGitIntegration_SkipsRun() {
	s.cleanupRuns()
	s.mockState = "merged"

	// Insert a run in pr_open with pr_id but NULL id_git_integration.
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, pr_id, pr_host_type, pr_url, branch_name, stage_plan)
		SELECT id_issue, $1, $2, 'pr_open', '42', 'github', 'https://github.com/org/repo/pull/42', 'agent/b1/i1/123456', '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)

	err = s.poller.PollOnce(context.Background())
	s.Require().NoError(err)

	run := s.loadRun(idRun)
	s.Equal("pr_open", run.Phase, "run with no git integration must remain in pr_open")
}

func Test_RunMergePollerSuite(t *testing.T) {
	suite.Run(t, new(MergePollerSuite))
}
