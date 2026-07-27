package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/stretchr/testify/suite"
)

// AgentReconcileSuite covers: a still-live gateway reporting complete_stage
// for a task that an API restart (crash recovery) already blanket-failed must be
// reconciled (accepted) instead of 500'd, without creating a duplicate/orphaned PR.
type AgentReconcileSuite struct {
	suite.Suite
	App              *issue.Application
	Token            string
	BotToken         string
	BotUserID        int64
	IdProject        int64
	IdIssuePublic    int64
	IdGitIntegration int64
	mockServer       *httptest.Server

	createPRCalls atomic.Int64 // POST /pulls counter (atomic — hit concurrently in T4d)
	findReturnsPR bool         // GET /pulls returns an existing PR (reuse path) when true
	failCreatePR  bool         // POST /pulls returns 500 (host PR creation fails) when true
}

func (s *AgentReconcileSuite) SetupSuite() {
	os.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	githost.ResetEncryptionKey()

	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"reconcilebot","email":"reconcilebot@test.sk","password":"kreslo"}`, s.Token)
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"reconcilebot@test.sk","password":"kreslo"}`, "")
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
	// NOTE: do NOT delete the cached login token — the reconcile tests call
	// complete_stage over HTTP with s.BotToken, which the Auth middleware validates
	// against the cache. (Deleting it would 401 every call.)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"reconcile-test-project","color":"#334455"}`, s.Token)
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
		`{"title":"reconcile test issue","description":"reconcile test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic

	// Mock GitHub API. apiBase is {baseUrl}/api/v3 for non-github.com hosts.
	s.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/pulls") && r.Method == http.MethodPost:
			s.createPRCalls.Add(1)
			if s.failCreatePR {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"message":"host down"}`)
				return
			}
			fmt.Fprint(w, `{"number":7,"html_url":"https://github.com/org/repo/pull/7"}`)
		case strings.HasSuffix(p, "/pulls") && r.Method == http.MethodGet:
			if s.findReturnsPR {
				fmt.Fprint(w, `[{"number":5,"html_url":"https://github.com/org/repo/pull/5"}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case strings.HasSuffix(p, "/repos/org/repo"):
			fmt.Fprint(w, `{"default_branch":"main"}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))

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
}

func (s *AgentReconcileSuite) TearDownSuite() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.App.Pool.Exec(context.Background(), "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(), "DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *AgentReconcileSuite) SetupTest() {
	s.createPRCalls.Store(0)
	s.findReturnsPR = false
	s.failCreatePR = false
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.task WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)", s.IdProject)
	s.App.Pool.Exec(context.Background(), "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
}

// insertRun inserts a run for the suite issue in the given phase, no PR set.
// When failed=true it also stamps finished_at + error_message='crash_recovery',
// mirroring what RunCrashRecovery/FailRun leave behind.
func (s *AgentReconcileSuite) insertRun(phase string, failed bool) int64 {
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, id_git_integration, stage_plan,
		                      finished_at, error_message)
		SELECT id_issue, $1, $2, $3, $4, '{"stages":[]}',
		       CASE WHEN $5 THEN now() ELSE NULL END,
		       CASE WHEN $5 THEN 'crash_recovery' ELSE NULL END
		FROM issues.issue WHERE id_issue_public = $6 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, phase, s.IdGitIntegration, failed, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)
	return idRun
}

// insertPrOpenRunWithPr inserts a run already in pr_open with a pr_id (as a
// successful reconcile / normal completion would leave it).
func (s *AgentReconcileSuite) insertPrOpenRunWithPr() int64 {
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, pr_id, pr_host_type, pr_url, branch_name, id_git_integration, stage_plan)
		SELECT id_issue, $1, $2, 'pr_open', '7', 'github', 'https://github.com/org/repo/pull/7', 'agent/b1/i1/111', $3, '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $4 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdGitIntegration, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)
	return idRun
}

func (s *AgentReconcileSuite) insertTask(idRun int64, stage, status string, errorReason *string) int64 {
	var idTask int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.task(id_run, id_user_bot, stage, attempt_no, status, error_reason,
		                       finished_at)
		VALUES ($1, $2, $3, 1, $4, $5,
		        CASE WHEN $4 IN ('failed','completed','cancelled') THEN now() ELSE NULL END)
		RETURNING id_task`,
		idRun, s.BotUserID, stage, status, errorReason,
	).Scan(&idTask)
	s.Require().NoError(err)
	return idTask
}

func (s *AgentReconcileSuite) loadRun(idRun int64) *model.AgentRun {
	var run model.AgentRun
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT phase, pr_id, error_message, finished_at
		FROM agent.run WHERE id_run = $1`, idRun,
	).Scan(&run.Phase, &run.PrId, &run.ErrorMessage, &run.FinishedAt)
	s.Require().NoError(err)
	return &run
}

func (s *AgentReconcileSuite) loadTask(idTask int64) *model.AgentTask {
	var t model.AgentTask
	err := s.App.Pool.QueryRow(context.Background(), `
		SELECT status, id_output_message FROM agent.task WHERE id_task = $1`, idTask,
	).Scan(&t.Status, &t.IdOutputMessage)
	s.Require().NoError(err)
	return &t
}

func (s *AgentReconcileSuite) complete(idTask int64, stage, outcome, kind, branch string) *http.Response {
	body := fmt.Sprintf(
		`{"outcome":%q,"message":"done from %s","messageKind":%q,"branchName":%q,"prTitle":"t","prBody":"b"}`,
		outcome, stage, kind, branch,
	)
	return Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/complete", idTask), body, s.BotToken)
}

func strptr(s string) *string { return &s }

// Happy path (control): active task + in_progress run completes normally to pr_open.
func (s *AgentReconcileSuite) Test_00_ActiveHappyPath() {
	idRun := s.insertRun("in_progress", false)
	idTask := s.insertTask(idRun, "implementation", "active", nil)

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/200")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	run := s.loadRun(idRun)
	s.Equal("pr_open", run.Phase)
	s.Require().NotNil(run.PrId)
	s.Equal(1, int(s.createPRCalls.Load()))
	s.Equal("completed", s.loadTask(idTask).Status)
}

// T1: reconcile a crash-orphaned task on the implementation (PR) stage.
func (s *AgentReconcileSuite) Test_01_ReconcileCrashFailedWithPr() {
	idRun := s.insertRun("failed", true)
	idTask := s.insertTask(idRun, "implementation", "failed", strptr("crash_recovery"))

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/201")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	run := s.loadRun(idRun)
	s.Equal("pr_open", run.Phase, "reconcile must move the run out of failed")
	s.Require().NotNil(run.PrId)
	s.Nil(run.FinishedAt, "reconcile must clear the stale finished_at")
	s.Nil(run.ErrorMessage, "reconcile must clear the stale error_message")
	s.Equal(1, int(s.createPRCalls.Load()), "exactly one PR created/reused")
	task := s.loadTask(idTask)
	s.Equal("completed", task.Status)
	s.NotNil(task.IdOutputMessage, "agent message persisted")
}

// T2: a genuinely-failed task (agent error) is NOT reconciled and creates NO PR.
func (s *AgentReconcileSuite) Test_02_GenuineFailNoReconcileNoPr() {
	idRun := s.insertRun("failed", true)
	idTask := s.insertTask(idRun, "implementation", "failed", strptr("agent_reported_error"))

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/202")
	s.Require().Equal(http.StatusOK, res.StatusCode, "no-op success, not 500")

	s.Equal(0, int(s.createPRCalls.Load()), "must not create a host PR for a genuine failure")
	run := s.loadRun(idRun)
	s.Equal("failed", run.Phase, "genuine failure stays failed")
	s.Nil(run.PrId)
	s.Equal("failed", s.loadTask(idTask).Status)
}

// T3: a duplicate complete_stage after success is an idempotent no-op.
func (s *AgentReconcileSuite) Test_03_IdempotentDuplicateComplete() {
	idRun := s.insertPrOpenRunWithPr()
	idTask := s.insertTask(idRun, "implementation", "completed", nil)

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/203")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal(0, int(s.createPRCalls.Load()), "no second PR on a retried complete")
	run := s.loadRun(idRun)
	s.Equal("pr_open", run.Phase)
	s.Require().NotNil(run.PrId)
	s.Equal("7", *run.PrId, "existing PR id unchanged")
}

// T4a: when the user has Restarted (a newer run exists), a late reconcile no-ops
// and creates no PR on the old branch.
func (s *AgentReconcileSuite) Test_04a_ReconcileSkippedWhenNewerRunExists() {
	oldRun := s.insertRun("failed", true)
	idTask := s.insertTask(oldRun, "implementation", "failed", strptr("crash_recovery"))
	// Simulate Restart: a fresh, non-terminal run for the same issue.
	s.insertRun("queued", false)

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/204")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal(0, int(s.createPRCalls.Load()), "must not open a second PR when a newer run exists")
	run := s.loadRun(oldRun)
	s.Equal("failed", run.Phase, "old run stays failed")
	s.Nil(run.PrId)
}

// T4b: Restart is refused when the run already has a PR (post-reconcile guard).
func (s *AgentReconcileSuite) Test_04b_RestartRefusedWhenPrExists() {
	idRun := s.insertPrOpenRunWithPr()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/run/%d/restart", idRun), "", s.Token)
	s.Equal(http.StatusConflict, res.StatusCode, "restart must be blocked on a run with a PR")
}

// T6: heartbeat_stale is deliberately excluded from the reconcile allowlist.
func (s *AgentReconcileSuite) Test_06_HeartbeatStaleNotReconciled() {
	idRun := s.insertRun("failed", true)
	idTask := s.insertTask(idRun, "implementation", "failed", strptr("heartbeat_stale"))

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/206")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal(0, int(s.createPRCalls.Load()), "heartbeat_stale must not be reconciled")
	s.Equal("failed", s.loadRun(idRun).Phase)
	s.Equal("failed", s.loadTask(idTask).Status)
}

// T8: reconcile a crash-orphaned NON-PR stage advances the pipeline (no PR).
func (s *AgentReconcileSuite) Test_08_ReconcileNonPrStage() {
	idRun := s.insertRun("failed", true)
	idTask := s.insertTask(idRun, "brainstorming", "failed", strptr("crash_recovery"))

	res := s.complete(idTask, "brainstorming", "output_submitted", "brainstorming_complete", "")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal(0, int(s.createPRCalls.Load()), "non-implementation stage opens no PR")
	run := s.loadRun(idRun)
	s.Equal("in_progress", run.Phase, "reconcile advances the pipeline out of failed")
	s.Nil(run.FinishedAt)
	s.Nil(run.ErrorMessage)
	s.Equal("completed", s.loadTask(idTask).Status)
}

// T4c: reconcile is skipped (no-op, no PR) when the run is no longer `failed`
// (e.g. Continue already moved it to queued) even though the task is still a
// crash-failed one. Covers the pre-tx gate condition `run.Phase == failed`.
func (s *AgentReconcileSuite) Test_04c_ReconcileSkippedWhenRunNotFailed() {
	idRun := s.insertRun("queued", false) // Continue already moved the run out of failed
	idTask := s.insertTask(idRun, "implementation", "failed", strptr("crash_recovery"))

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/205")
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.Equal(0, int(s.createPRCalls.Load()), "no PR when the run already moved out of failed")
	run := s.loadRun(idRun)
	s.Equal("queued", run.Phase, "run must stay where Continue left it")
	s.Nil(run.PrId)
	s.Equal("failed", s.loadTask(idTask).Status)
}

// T4d: under concurrency — a Continue moving the run out of `failed` WHILE a late
// complete_stage reconciles — complete_stage must never 500 and must leave the run
// consistent (PR linked ⇒ pr_open; no PR ⇒ not pr_open). Stresses the in-tx CAS
// tolerance (errReconcileSuperseded) that Task 3 of the plan requires.
func (s *AgentReconcileSuite) Test_04d_ConcurrentReconcileVsMoveNever500() {
	const iterations = 25
	for i := 0; i < iterations; i++ {
		s.SetupTest()
		idRun := s.insertRun("failed", true)
		idTask := s.insertTask(idRun, "implementation", "failed", strptr("crash_recovery"))

		var wg sync.WaitGroup
		wg.Add(2)
		var completeStatus int
		go func() {
			defer wg.Done()
			res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed",
				fmt.Sprintf("agent/b1/i1/9%02d", i))
			completeStatus = res.StatusCode
		}()
		go func() {
			defer wg.Done()
			// Realistic Continue: CAS failed→queued (matches TransitionPhase, so it will
			// NOT clobber a run the reconcile already advanced to pr_open).
			s.App.Pool.Exec(context.Background(),
				"UPDATE agent.run SET phase='queued', finished_at=NULL, error_message=NULL WHERE id_run=$1 AND phase='failed'", idRun)
		}()
		wg.Wait()

		s.Require().NotEqual(http.StatusInternalServerError, completeStatus,
			"complete_stage must never 500 under a concurrent run move (iter %d)", i)
		s.LessOrEqual(int(s.createPRCalls.Load()), 1, "at most one host PR per attempt (iter %d)", i)
		run := s.loadRun(idRun)
		if run.PrId != nil {
			s.Equal("pr_open", run.Phase, "a linked PR must leave the run in pr_open (iter %d)", i)
		} else {
			s.NotEqual("pr_open", run.Phase, "no PR link → run must not be pr_open (iter %d)", i)
		}
	}
}

// T4g: two complete_stage calls racing on the SAME reconcilable task (gateway +
// MCP tool, or a retry) must never 500 — the loser is a superseded no-op. Exactly
// one wins (task completed, run pr_open).
func (s *AgentReconcileSuite) Test_04g_ConcurrentDoubleCompleteNever500() {
	const iterations = 25
	for i := 0; i < iterations; i++ {
		s.SetupTest()
		idRun := s.insertRun("failed", true)
		idTask := s.insertTask(idRun, "implementation", "failed", strptr("crash_recovery"))

		var wg sync.WaitGroup
		wg.Add(2)
		statuses := make([]int, 2)
		for g := 0; g < 2; g++ {
			g := g
			go func() {
				defer wg.Done()
				res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed",
					fmt.Sprintf("agent/b1/i1/8%02d", i))
				statuses[g] = res.StatusCode
			}()
		}
		wg.Wait()

		s.Require().NotContains(statuses, http.StatusInternalServerError,
			"neither concurrent complete may 500 (iter %d): %v", i, statuses)
		run := s.loadRun(idRun)
		s.Equal("pr_open", run.Phase, "one winner completes the run to pr_open (iter %d)", i)
		s.Require().NotNil(run.PrId)
		s.Equal("completed", s.loadTask(idTask).Status)
	}
}

// T4e: a reconcile whose host PR creation FAILS must record `errored` cleanly
// (200, task failed with pr_creation_failed) — not 500. Guards the SetError vs
// CompleteReconcilable ordering (SetError must not push the row out of the
// recoverable allowlist before the status CAS).
func (s *AgentReconcileSuite) Test_04e_ReconcileHostPrFailureRecordsErrored() {
	s.failCreatePR = true
	idRun := s.insertRun("failed", true)
	idTask := s.insertTask(idRun, "implementation", "failed", strptr("crash_recovery"))

	res := s.complete(idTask, "implementation", "output_submitted", "pull_request_pushed", "agent/b1/i1/207")
	s.Require().Equal(http.StatusOK, res.StatusCode, "host PR failure must record errored, not 500")

	run := s.loadRun(idRun)
	s.Equal("failed", run.Phase, "run stays failed when the PR could not be opened")
	s.Nil(run.PrId)
	s.Equal("failed", s.loadTask(idTask).Status)
}

// T4f (deterministic): the reconcile CAS repo methods must map a unique-constraint
// violation (agent_run_one_active_per_issue — a competing active run for the issue,
// e.g. a Restart landing mid-reconcile) to ErrPhaseMismatch, so the controller
// no-ops instead of 500 + orphaned PR. Exercised directly at the repo layer because
// the exact race window is non-deterministic over HTTP.
func (s *AgentReconcileSuite) Test_04f_ReconcileCasUniqueViolationIsPhaseMismatch() {
	runRepo := injector.GetAgentRunRepository()
	failedRun := s.insertRun("failed", true) // crash-orphaned run
	s.insertRun("queued", false)             // competing active run for the same issue (Restart)

	dto := model.SetRunPrReq{
		PrUrl: "https://github.com/org/repo/pull/7", PrId: "7", PrHostType: "github",
		BranchName: "agent/b1/i1/208", IdGitIntegration: s.IdGitIntegration,
	}
	_, err := runRepo.SetPrInfoFrom(context.Background(), failedRun, dto, "failed")
	s.Require().Error(err)
	s.True(errors.Is(err, repository.ErrPhaseMismatch),
		"SetPrInfoFrom must map the unique violation to ErrPhaseMismatch, got %v", err)

	_, err = runRepo.ReconcileToPhase(context.Background(), failedRun, "failed", "in_progress", "agent", "reconcile")
	s.Require().Error(err)
	s.True(errors.Is(err, repository.ErrPhaseMismatch),
		"ReconcileToPhase must map the unique violation to ErrPhaseMismatch, got %v", err)

	// The crash-orphaned run must be untouched by the rejected CAS.
	s.Equal("failed", s.loadRun(failedRun).Phase)
}

func Test_RunAgentReconcileSuite(t *testing.T) {
	suite.Run(t, new(AgentReconcileSuite))
}
