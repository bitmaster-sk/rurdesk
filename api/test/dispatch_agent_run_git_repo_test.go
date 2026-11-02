package test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

type runGitRepoFixture struct {
	app              *issue.Application
	token            string
	idAgent          int64
	gitHost          *httptest.Server
	repoLookups      atomic.Int32
	received         chan []byte
	stagePlan        json.RawMessage
	dispatchAttempts int
	webhookSecret    []byte
}

func newRunGitRepoFixture(t *testing.T) *runGitRepoFixture {
	t.Helper()
	t.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	githost.ResetEncryptionKey()
	t.Cleanup(githost.ResetEncryptionKey)

	fx := &runGitRepoFixture{received: make(chan []byte, 4)}
	fx.app = Setup(t)
	fx.token = Token(t, fx.app)

	fx.gitHost = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v3/repos/org/repo" {
			fx.repoLookups.Add(1)
			fmt.Fprint(w, `{"default_branch":"trunk"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(fx.gitHost.Close)

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fx.received <- body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(gateway.Close)

	agentRes := Request(t, fx.app, "POST", "/api/private/admin/user", `{"name":"runrepobot","isAgent":true}`, fx.token)
	require.Equal(t, http.StatusOK, agentRes.StatusCode)
	var agentUser struct {
		IdUser int64 `json:"idUser"`
	}
	require.NoError(t, json.NewDecoder(agentRes.Body).Decode(&agentUser))
	fx.idAgent = agentUser.IdUser

	gwRes := Request(t, fx.app, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", fx.idAgent),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gateway.URL), fx.token)
	require.Equal(t, http.StatusOK, gwRes.StatusCode)
	var gw struct {
		TrackerToGatewayToken string `json:"trackerToGatewayToken"`
	}
	require.NoError(t, json.NewDecoder(gwRes.Body).Decode(&gw))
	webhookSecret, err := hex.DecodeString(gw.TrackerToGatewayToken)
	require.NoError(t, err)
	fx.webhookSecret = webhookSecret

	stagePlan, err := injector.GetAgentStagePlanService().BuildFromChoices(constants.RunKindIssue, model.RunChoices{})
	require.NoError(t, err)
	fx.stagePlan = stagePlan
	return fx
}

func (fx *runGitRepoFixture) project(t *testing.T, name string) int64 {
	t.Helper()
	idProject := createProject(t, fx.app, fx.token, name)
	member := Request(t, fx.app, "POST", fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, fx.idAgent), fx.token)
	require.Equal(t, http.StatusOK, member.StatusCode)
	return idProject
}

func (fx *runGitRepoFixture) integration(t *testing.T, idProject int64, baseBranchJson string) model.GitIntegrationRes {
	t.Helper()
	res := Request(t, fx.app, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", idProject),
		fmt.Sprintf(`{"name":"mock-git","hostType":"github","baseUrl":%q,"repoPath":"org/repo","baseBranch":%s,"accessToken":"ghp_run_repo_token"}`,
			fx.gitHost.URL, baseBranchJson), fx.token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var integration model.GitIntegrationRes
	require.NoError(t, json.NewDecoder(res.Body).Decode(&integration))
	return integration
}

func (fx *runGitRepoFixture) run(t *testing.T, idProject int64) *model.AgentRun {
	t.Helper()
	issRes := Request(t, fx.app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"run-repo","description":"body","estimated":0}`, fx.token)
	require.Equal(t, http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	require.NoError(t, json.NewDecoder(issRes.Body).Decode(&iss))
	run, err := injector.GetAgentRunRepository().InsertForIssue(context.Background(), iss.IdIssue, fx.idAgent, idProject, fx.stagePlan)
	require.NoError(t, err)
	return run
}

func (fx *runGitRepoFixture) dispatch(t *testing.T, run *model.AgentRun) {
	t.Helper()
	fx.dispatchAttempts++
	task, err := injector.GetAgentTaskRepository().Insert(context.Background(), run.IdRun, fx.idAgent, "implementation", fx.dispatchAttempts)
	require.NoError(t, err)
	injector.GetDispatcher().DispatchStageExecute(context.Background(), run, task)
}

func (fx *runGitRepoFixture) receivedRepo(t *testing.T) model.AgentRunGitRepo {
	t.Helper()
	var raw []byte
	select {
	case raw = <-fx.received:
	case <-time.After(10 * time.Second):
		t.Fatal("gateway never received the stage_execute event")
	}
	var event struct {
		Payload struct {
			Repo model.AgentRunGitRepo `json:"repo"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &event))
	return event.Payload.Repo
}

func (fx *runGitRepoFixture) decryptGitToken(t *testing.T, encrypted string) string {
	t.Helper()
	mac := hmac.New(sha256.New, fx.webhookSecret)
	mac.Write([]byte("rurdesk git token v1"))
	block, err := aes.NewCipher(mac.Sum(nil))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	sealed, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	require.NoError(t, err)
	return string(plain)
}

func (fx *runGitRepoFixture) reloadRun(t *testing.T, idRun int64) *model.AgentRun {
	t.Helper()
	run, err := injector.GetAgentRunRepository().LoadById(context.Background(), idRun)
	require.NoError(t, err)
	return run
}

func TestDispatchSendsTheProjectRepo(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	idProject := fx.project(t, "run-repo-default-branch")
	integration := fx.integration(t, idProject, "null")
	run := fx.run(t, idProject)

	fx.dispatch(t, run)
	repo := fx.receivedRepo(t)

	gitHostUrl, err := url.Parse(fx.gitHost.URL)
	require.NoError(t, err)
	require.Equal(t, fx.gitHost.URL+"/org/repo.git", repo.CloneUrl)
	require.Equal(t, gitHostUrl.Host, repo.Host)
	require.Equal(t, "org/repo", repo.RepoPath)
	require.Equal(t, "trunk", repo.BaseBranch, "without a base branch on the integration the host default branch is used")
	require.NotContains(t, repo.EncryptedToken, "ghp_run_repo_token")
	require.Equal(t, "ghp_run_repo_token", fx.decryptGitToken(t, repo.EncryptedToken), "the gateway's webhook secret decrypts the token")

	stamped := fx.reloadRun(t, run.IdRun)
	require.NotNil(t, stamped.IdGitIntegration)
	require.Equal(t, integration.IdGitIntegration, *stamped.IdGitIntegration)
	require.NotNil(t, stamped.BaseBranch)
	require.Equal(t, "trunk", *stamped.BaseBranch)

	updateRes := Request(t, fx.app, "PUT",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", idProject, integration.IdGitIntegration),
		fmt.Sprintf(`{"name":"mock-git","hostType":"github","baseUrl":%q,"repoPath":"org/repo","baseBranch":"develop"}`, fx.gitHost.URL),
		fx.token)
	require.Equal(t, http.StatusOK, updateRes.StatusCode)

	fx.dispatch(t, stamped)
	require.Equal(t, "trunk", fx.receivedRepo(t).BaseBranch, "a later stage keeps the base branch the run started from")
}

func TestDispatchUsesTheIntegrationBaseBranch(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	idProject := fx.project(t, "run-repo-configured-branch")
	fx.integration(t, idProject, `"develop"`)
	run := fx.run(t, idProject)

	fx.dispatch(t, run)

	require.Equal(t, "develop", fx.receivedRepo(t).BaseBranch)
	require.Zero(t, fx.repoLookups.Load(), "a configured base branch must not ask the host for its default branch")
}

type singleRunDispatcher struct {
	idRun int64
	real  agent.StageDispatcher
}

func (d *singleRunDispatcher) DispatchStageExecute(ctx context.Context, run *model.AgentRun, task *model.AgentTask) {
	if run.IdRun == d.idRun {
		d.real.DispatchStageExecute(ctx, run, task)
	}
}

func TestSchedulerFailsTheRunWithoutGitIntegration(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	idProject := fx.project(t, "run-repo-no-integration")
	run := fx.run(t, idProject)
	_, err := fx.app.Pool.Exec(context.Background(),
		`UPDATE issues.issue SET assigned_to = $1 WHERE id_issue = $2`, fx.idAgent, run.IdIssue)
	require.NoError(t, err)

	sched := agent.NewScheduler(fx.app.Pool, injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(), injector.GetIssueRepository(),
		&singleRunDispatcher{idRun: run.IdRun, real: injector.GetDispatcher()}, injector.GetNotifier())
	require.NoError(t, sched.TickOnce(context.Background()))

	require.Eventually(t, func() bool {
		return fx.reloadRun(t, run.IdRun).Phase == "failed"
	}, 10*time.Second, 50*time.Millisecond, "a run in a project without a git integration must fail, not stay in progress")

	var errorDetail string
	require.NoError(t, fx.app.Pool.QueryRow(context.Background(),
		`SELECT error_detail FROM agent.task WHERE id_run = $1`, run.IdRun).Scan(&errorDetail))
	require.Contains(t, errorDetail, "no git integration")

	select {
	case <-fx.received:
		t.Fatal("no stage_execute may reach the gateway without a repo")
	default:
	}
}
