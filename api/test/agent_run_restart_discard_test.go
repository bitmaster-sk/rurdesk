package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// Restart replaces the run with a fresh one, so the gateway must drop the old
// run's worktree; Cancel keeps it for Continue and sends only `cancelled`.
func TestRestartSendsRunDiscardedForOldRun(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	ctx := context.Background()

	discarded := make(chan []byte, 1)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// The new run gets its own stage_execute; only the discard matters here.
		if r.Header.Get("X-Tracker-Event-Type") == "run_discarded" {
			select {
			case discarded <- body:
			default:
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer gateway.Close()

	agentRes := Request(t, app, "POST", "/api/private/admin/user",
		`{"name":"restartdiscardbot","isAgent":true}`, token)
	require.Equal(t, http.StatusOK, agentRes.StatusCode)
	var agent struct {
		IdUser int64 `json:"idUser"`
	}
	require.NoError(t, json.NewDecoder(agentRes.Body).Decode(&agent))
	gwRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", agent.IdUser),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gateway.URL), token)
	require.Equal(t, http.StatusOK, gwRes.StatusCode)

	idProject := createProject(t, app, token, "restart-discard-project")
	issRes := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"restart-discard","description":"body","estimated":0}`, token)
	require.Equal(t, http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	require.NoError(t, json.NewDecoder(issRes.Body).Decode(&iss))

	stagePlan, err := injector.GetAgentStagePlanService().BuildFromChoices(constants.RunKindIssue, model.RunChoices{})
	require.NoError(t, err)
	oldRun, err := injector.GetAgentRunRepository().InsertForIssue(ctx, iss.IdIssue, agent.IdUser, idProject, stagePlan)
	require.NoError(t, err)

	restartRes := Request(t, app, "POST", fmt.Sprintf("/api/private/agent/run/%d/restart", oldRun.IdRun), "", token)
	require.Equal(t, http.StatusOK, restartRes.StatusCode)

	select {
	case raw := <-discarded:
		var event struct {
			IdRun int64  `json:"idRun"`
			Event string `json:"event"`
		}
		require.NoError(t, json.Unmarshal(raw, &event))
		require.Equal(t, oldRun.IdRun, event.IdRun)
		require.Equal(t, "run_discarded", event.Event)
	case <-time.After(10 * time.Second):
		t.Fatal("the gateway never got run_discarded for the restarted run")
	}
}
