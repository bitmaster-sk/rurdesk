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

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestDispatchCarriesResolvedSkills(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	seedBuiltinSkills(t)
	ctx := context.Background()

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

	botRes := Request(t, app, "POST", "/api/private/admin/user",
		`{"name":"dispatchskillbot","isBot":true}`, token)
	require.Equal(t, http.StatusOK, botRes.StatusCode)
	var bot struct {
		IdUser int64 `json:"idUser"`
	}
	require.NoError(t, json.NewDecoder(botRes.Body).Decode(&bot))

	gwRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/gateway", bot.IdUser),
		fmt.Sprintf(`{"gatewayUrl":%q}`, gateway.URL), token)
	require.Equal(t, http.StatusOK, gwRes.StatusCode)

	idProject := createProject(t, app, token, "dispatch-skills-project")
	member := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, bot.IdUser), token)
	require.Equal(t, http.StatusOK, member.StatusCode)

	issRes := Request(t, app, "POST", fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"dispatch-skills","description":"body","estimated":0}`, token)
	require.Equal(t, http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	require.NoError(t, json.NewDecoder(issRes.Body).Decode(&iss))

	skill := skillByName(t, listSkills(t, app, token), "Verification rules")
	stagePlan, err := injector.GetStagePlanService().Build(map[string][]int64{"implementation": {skill.IdSkill}})
	require.NoError(t, err)
	run, err := injector.GetAgentRunRepository().Insert(ctx, iss.IdIssue, bot.IdUser, idProject, stagePlan)
	require.NoError(t, err)

	task, err := injector.GetAgentTaskRepository().Insert(ctx, run.IdRun, bot.IdUser, "implementation", 1)
	require.NoError(t, err)

	injector.GetDispatcher().DispatchStageExecute(ctx, run, task)

	var raw []byte
	select {
	case raw = <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("gateway never received the stage_execute event")
	}

	var event struct {
		Event   string `json:"event"`
		Payload struct {
			ContextBundle struct {
				Skills []model.Skill `json:"skills"`
			} `json:"contextBundle"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &event))
	require.Equal(t, "stage_execute", event.Event)
	require.Len(t, event.Payload.ContextBundle.Skills, 1)
	require.Equal(t, "Verification rules", event.Payload.ContextBundle.Skills[0].Name)
	require.Contains(t, event.Payload.ContextBundle.Skills[0].Content, "Never push red")
}
