package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestDispatchBroadcastsTheModelTheStageRunsWith(t *testing.T) {
	fx := newRunGitRepoFixture(t)
	idProject := fx.project(t, "run-broadcast-model")
	fx.integration(t, idProject, `"main"`)
	run := fx.run(t, idProject)
	require.NoError(t, injector.GetAgentGatewayRepository().SetModels(context.Background(), fx.idAgent,
		model.GatewayRecoveredReq{Models: []string{"openai/kimi", "ollama/qwen3"}, ModelDefault: "openai/kimi"}))

	server := httptest.NewServer(fx.app)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/private/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {"Bearer " + fx.token}})
	require.NoError(t, err)
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)

	fx.dispatch(t, run)
	fx.receivedRepo(t)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		_, raw, err := conn.ReadMessage()
		require.NoError(t, err, "no agent_run broadcast carried the model of the dispatched stage")
		var msg struct {
			Subject string `json:"subject"`
			Payload struct {
				IdRun  int64 `json:"idRun"`
				Stages []struct {
					Stage string  `json:"stage"`
					Model *string `json:"model"`
				} `json:"stages"`
			} `json:"payload"`
		}
		if json.Unmarshal(raw, &msg) != nil || msg.Subject != "agent_run" || msg.Payload.IdRun != run.IdRun {
			continue
		}
		for _, stage := range msg.Payload.Stages {
			if stage.Stage == constants.StageImplementation && stage.Model != nil {
				require.Equal(t, "openai/kimi", *stage.Model, "a stage without a chosen model runs the gateway default")
				return
			}
		}
	}
}
