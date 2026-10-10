package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestDispatchCarriesBranchOfOpenPr(t *testing.T) {
	t.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	githost.ResetEncryptionKey()
	t.Cleanup(githost.ResetEncryptionKey)

	app := Setup(t)
	token := Token(t, app)
	ctx := context.Background()

	var prState atomic.Value
	prState.Store("open")
	gitHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/reviews"):
			fmt.Fprint(w, `[]`)
		case strings.Contains(r.URL.Path, "/commits/"):
			fmt.Fprint(w, `{"state":"success"}`)
		default:
			switch prState.Load().(string) {
			case "unreachable":
				w.WriteHeader(http.StatusNotFound)
			case "merged":
				fmt.Fprint(w, `{"state":"closed","merged":true,"head":{"sha":"abc123"}}`)
			case "closed":
				fmt.Fprint(w, `{"state":"closed","merged":false,"head":{"sha":"abc123"}}`)
			default:
				fmt.Fprint(w, `{"state":"open","merged":false,"head":{"sha":"abc123"}}`)
			}
		}
	}))
	defer gitHost.Close()

	received := make(chan []byte, 1)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	agentRes := Request(t, app, "POST", "/api/private/admin/user",
		`{"name":"prbranchbot","isAgent":true}`, token)
	require.Equal(t, http.StatusOK, agentRes.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	require.NoError(t, json.NewDecoder(agentRes.Body).Decode(&agent))

	gwRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", agent.IdUser),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gateway.URL), token)
	require.Equal(t, http.StatusOK, gwRes.StatusCode)

	idProject := createProject(t, app, token, "dispatch-pr-branch-project")
	member := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, agent.IdUser), token)
	require.Equal(t, http.StatusOK, member.StatusCode)

	intRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", idProject),
		fmt.Sprintf(`{"name":"mock-git","hostType":"github","baseUrl":%q,"repoPath":"org/repo","accessToken":"ghp_mock_token"}`, gitHost.URL),
		token)
	require.Equal(t, http.StatusCreated, intRes.StatusCode)
	var integration model.GitIntegrationRes
	require.NoError(t, json.NewDecoder(intRes.Body).Decode(&integration))

	issRes := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"dispatch-pr-branch","description":"body","estimated":0}`, token)
	require.Equal(t, http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	require.NoError(t, json.NewDecoder(issRes.Body).Decode(&iss))

	stagePlan, err := injector.GetStagePlanService().Build(nil)
	require.NoError(t, err)
	runRepo := injector.GetAgentRunRepository()

	attemptNo := 0
	dispatchPrBranch := func(t *testing.T, run *model.AgentRun) string {
		t.Helper()
		attemptNo++
		task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, agent.IdUser, "implementation", attemptNo)
		require.NoError(t, err)
		injector.GetDispatcher().DispatchStageExecute(ctx, run, task)

		var raw []byte
		select {
		case raw = <-received:
		case <-time.After(10 * time.Second):
			t.Fatal("gateway never received the stage_execute event")
		}
		var event struct {
			Payload struct {
				PrBranch string `json:"prBranch"`
			} `json:"payload"`
		}
		require.NoError(t, json.Unmarshal(raw, &event))
		return event.Payload.PrBranch
	}

	firstRun, err := runRepo.Insert(ctx, iss.IdIssue, agent.IdUser, idProject, stagePlan)
	require.NoError(t, err)
	require.Empty(t, dispatchPrBranch(t, firstRun), "an issue without a PR must get a new branch")

	const prBranch = "agent/a1/i48/1790053695"
	_, err = app.Pool.Exec(ctx, `
		UPDATE agent.run
		SET phase = 'cancelled', pr_id = '42', pr_host_type = 'github',
		    pr_url = 'https://github.com/org/repo/pull/42', branch_name = $2, id_git_integration = $3
		WHERE id_run = $1`,
		firstRun.IdRun, prBranch, integration.IdGitIntegration)
	require.NoError(t, err)

	secondRun, err := runRepo.Insert(ctx, iss.IdIssue, agent.IdUser, idProject, stagePlan)
	require.NoError(t, err)

	tests := []struct {
		name     string
		prState  string
		expected string
	}{
		{"a later run continues on the branch of the still-open PR", "open", prBranch},
		{"a merged PR gets a new branch", "merged", ""},
		{"a PR closed without merge gets a new branch", "closed", ""},
		{"a git host error keeps the last PR branch", "unreachable", prBranch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prState.Store(tc.prState)
			require.Equal(t, tc.expected, dispatchPrBranch(t, secondRun))
		})
	}
}
