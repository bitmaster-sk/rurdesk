package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/suite"
)

// stubStageDispatcher records dispatched tasks instead of firing the real
// per-stage webhook, so a scheduler tick can be asserted without a gateway.
type stubStageDispatcher struct {
	dispatched []*model.AgentTask
}

func (d *stubStageDispatcher) DispatchStageExecute(_ context.Context, _ *model.AgentRun, task *model.AgentTask) {
	d.dispatched = append(d.dispatched, task)
}

// SchedulerBroadcastSuite covers the "timeline doesn't spin after approval" bug:
// when the scheduler dispatches the next stage of an already-in_progress run —
// the state right after a user approves a stage — it must broadcast a run
// snapshot so the client sees the new stage go active. Previously the broadcast
// fired only on the initial queued→in_progress transition, so every post-approval
// stage executed with the timeline stuck on the prior (pending) stage until it
// completed.
type SchedulerBroadcastSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	BotUserID int64
	IdProject int64
	IdIssue   int64
}

func (s *SchedulerBroadcastSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	botRes := Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"schedbroadcastbot","isBot":true}`, s.Token)
	s.Require().Equal(http.StatusOK, botRes.StatusCode)
	var bot struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(botRes.Body).Decode(&bot))
	s.BotUserID = bot.IdUser

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"sched-broadcast-project","color":"#445566"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	s.Require().NoError(json.NewDecoder(prjRes.Body).Decode(&prj))
	s.IdProject = prj.IdProject

	addRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)
	s.Require().Equal(http.StatusOK, addRes.StatusCode)

	// Create the issue unassigned (assigning a bot here would auto-start a run of
	// its own), then point assigned_to at the bot directly — LoadNextEligible
	// gates on i.assigned_to = r.id_user_bot.
	issRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"sched broadcast issue","description":"body body body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(issRes.Body).Decode(&iss))
	s.IdIssue = iss.IdIssue

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE issues.issue SET assigned_to = $1 WHERE id_issue = $2", s.BotUserID, s.IdIssue)
	s.Require().NoError(err)
}

func (s *SchedulerBroadcastSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

// wsRunSnapshot is the subset of the agent_run notice payload the assertion reads.
type wsRunSnapshot struct {
	Subject string `json:"subject"`
	Payload struct {
		IdRun  int64 `json:"idRun"`
		Stages []struct {
			Stage  string `json:"stage"`
			Status string `json:"status"`
		} `json:"stages"`
	} `json:"payload"`
}

// TestDispatchOnInProgressRunBroadcastsActiveStage builds a run that is already
// in_progress with its first stage completed (the post-approval shape), ticks the
// scheduler, and asserts the newly dispatched stage is broadcast to a connected
// client as active — which is what drives the timeline spinner.
func (s *SchedulerBroadcastSuite) TestDispatchOnInProgressRunBroadcastsActiveStage() {
	ctx := context.Background()

	// A real in-process server so we can hold a genuine websocket connection on the
	// app's shared notifier — the same instance the scheduler broadcasts through.
	server := httptest.NewServer(s.App)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/private/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {s.Token}})
	s.Require().NoError(err, "websocket connect")
	defer conn.Close()

	runRepo := repository.NewAgentRunRepository(s.App.Pool)
	taskRepo := repository.NewAgentTaskRepository(s.App.Pool)
	projectRepo := repository.NewProjectRepository(s.App.Pool)

	run, err := runRepo.Insert(ctx, s.IdIssue, s.BotUserID, s.IdProject, emptyStagePlan(s.T()))
	s.Require().NoError(err)

	// Move the run to in_progress and mark the first stage (pickup) completed, so
	// ResolveNextStage returns the next stage and the run is NOT in the queued
	// branch that used to be the only path that broadcast. (This DB setup also
	// gives the server time to register the websocket connection before the tick.)
	_, err = s.App.Pool.Exec(ctx,
		"UPDATE agent.run SET phase = $1 WHERE id_run = $2", constants.PhaseInProgress, run.IdRun)
	s.Require().NoError(err)

	pickup, err := taskRepo.Insert(ctx, run.IdRun, s.BotUserID, constants.StagePickup, 1)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx,
		"UPDATE agent.task SET status = $1, finished_at = now() WHERE id_task = $2",
		constants.TaskStatusCompleted, pickup.IdTask)
	s.Require().NoError(err)

	dispatcher := &stubStageDispatcher{}
	sched := agent.NewScheduler(runRepo, taskRepo, projectRepo, dispatcher, injector.GetNotifier())

	s.Require().NoError(sched.TickOnce(ctx))

	// The next stage for our run was dispatched. (The tick scans every active bot;
	// filter to our run so leftover runs from other suites can't fool the assert.)
	var dispatched *model.AgentTask
	for _, t := range dispatcher.dispatched {
		if t.IdRun == run.IdRun {
			dispatched = t
		}
	}
	s.Require().NotNil(dispatched, "scheduler must dispatch the next stage of the in_progress run")
	s.Equal(constants.StageBrainstorming, dispatched.Stage)

	// The connected client must receive a snapshot for our run with that stage
	// active. Without the fix no notice is sent on the in_progress path and this
	// read times out.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, raw, err := conn.ReadMessage()
		s.Require().NoError(err, "no agent_run broadcast with the dispatched stage active")

		var msg wsRunSnapshot
		if json.Unmarshal(raw, &msg) != nil || msg.Subject != "agent_run" || msg.Payload.IdRun != run.IdRun {
			continue
		}
		var brainstorming string
		for _, st := range msg.Payload.Stages {
			if st.Stage == constants.StageBrainstorming {
				brainstorming = st.Status
			}
		}
		s.Equal("active", brainstorming, "dispatched stage must be broadcast as active")
		return
	}
}

func TestSchedulerBroadcastSuite(t *testing.T) {
	suite.Run(t, new(SchedulerBroadcastSuite))
}
