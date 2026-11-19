package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// queuedIssueRun queues an issue run of the suite agent on a fresh issue assigned
// to it.
func (s *AgentConversationDispatchSuite) queuedIssueRun(title string) int64 {
	s.T().Helper()
	ctx := context.Background()
	created := s.createIssue(title)
	_, err := s.App.Pool.Exec(ctx, "UPDATE issues.issue SET assigned_to = $1 WHERE id_issue = $2", s.Agent, created.IdIssue)
	s.Require().NoError(err)
	stagePlan, err := injector.GetAgentStagePlanService().BuildFromChoices(constants.RunKindIssue, model.RunChoices{})
	s.Require().NoError(err)
	var idRun int64
	s.Require().NoError(s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, kind, phase, stage_plan)
		VALUES ($1, $2, $3, 'issue', 'queued', $4) RETURNING id_run`,
		created.IdIssue, s.Agent, s.P1, stagePlan).Scan(&idRun))
	return idRun
}

func (s *AgentConversationDispatchSuite) runPhase(idRun int64) string {
	s.T().Helper()
	run, err := injector.GetAgentRunRepository().LoadById(context.Background(), idRun)
	s.Require().NoError(err)
	return run.Phase
}

func (s *AgentConversationDispatchSuite) runExists(idRun int64) bool {
	s.T().Helper()
	var exists bool
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM agent.run WHERE id_run = $1)", idRun).Scan(&exists))
	return exists
}

// awaitEvents reads gateway events until every wanted (event, idRun) pair arrived.
// (The suite's projects have no git integration, so an issue run's dispatch stops
// before the gateway; tests see it by the attempt the scheduler creates.)
func (s *AgentConversationDispatchSuite) awaitEvents(want map[string]int64) {
	s.T().Helper()
	deadline := time.After(15 * time.Second)
	for len(want) > 0 {
		select {
		case raw := <-s.Events:
			var event struct {
				IdRun int64  `json:"idRun"`
				Event string `json:"event"`
			}
			s.Require().NoError(json.Unmarshal(raw, &event))
			if idRun, ok := want[event.Event]; ok && idRun == event.IdRun {
				delete(want, event.Event)
			}
		case <-deadline:
			s.FailNow(fmt.Sprintf("the gateway never received %v", want))
		}
	}
}

// A deleted owner takes the conversation along; the chat run, its turns and its
// events go too, so the agent's issue runs keep dispatching and a user who chatted
// can still be deleted.
func (s *AgentConversationDispatchSuite) TestOwnerDeleted_ChatRunRemovedAndIssueRunStillDispatches() {
	ctx := context.Background()
	createUserAsAdmin(s.T(), s.App, s.Token,
		`{"name":"chat deleted owner","email":"chat-deleted-owner@test.sk","password":"kreslo"}`)
	owner, err := injector.GetUserRepository().LoadUserByEmail(ctx, "chat-deleted-owner@test.sk")
	s.Require().NoError(err)
	s.addMember(s.P1, owner.IdUser)

	conv, err := s.Svc.CreateConversation(ctx, owner, model.CreateAgentConversationReq{IdUserAgent: s.Agent, IdProject: ptr(s.P1)})
	s.Require().NoError(err)
	_, err = s.Svc.CreateMessage(ctx, owner, conv.IdConversation, "first turn")
	s.Require().NoError(err)
	s.tick()
	first := s.nextEvent()
	s.complete(first.Payload.IdTask, chatReplyBody, first.Payload.RunToken)
	// The second message wakes the idle run on the owner's behalf (a run event
	// with the owner as actor) and queues the next turn.
	_, err = s.Svc.CreateMessage(ctx, owner, conv.IdConversation, "second turn, never answered")
	s.Require().NoError(err)
	idRunChat := first.IdRun
	s.Require().Equal(constants.PhaseQueued, s.runPhase(idRunChat))
	idRunIssue := s.queuedIssueRun("chat owner deleted: issue run")

	s.Require().Equal(http.StatusOK, s.status("DELETE", fmt.Sprintf("/api/private/admin/user/%d", owner.IdUser), "", s.Token),
		"chatting does not make the owner undeletable")

	s.False(s.runExists(idRunChat), "the chat run goes with its conversation")
	s.awaitEvents(map[string]int64{"run_discarded": idRunChat})
	s.tick()
	s.NotEmpty(s.tasks(idRunIssue), "the scheduler dispatches the issue run")
}

// A chat run whose conversation vanished without the cleanup (e.g. before it
// existed) is cancelled by the scheduler instead of blocking the agent's queue.
func (s *AgentConversationDispatchSuite) TestOrphanedQueuedChatRun_CancelledAndQueueMovesOn() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	s.post(conv, "orphaned soon")
	idRunChat := s.chatRun(conv).IdRun
	idRunIssue := s.queuedIssueRun("orphaned chat: issue run")
	_, err := s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_conversation = $1", conv.IdConversation)
	s.Require().NoError(err)

	s.tick()
	s.Equal(constants.PhaseCancelled, s.runPhase(idRunChat))
	for _, task := range s.tasks(idRunChat) {
		s.Equal(constants.TaskStatusCancelled, task.Status)
	}
	s.Empty(s.tasks(idRunIssue))
	s.tick()
	s.NotEmpty(s.tasks(idRunIssue), "the next tick dispatches the issue run")
}

// A running turn whose conversation vanished ends its run when the turn is lost,
// and its run token stops working.
func (s *AgentConversationDispatchSuite) TestOrphanedRunningChatTurn_SweepCancelsRunAndToken() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "orphaned while running")
	s.Require().Equal(http.StatusOK, s.status("GET", s.issuesURL(s.P1), "", event.Payload.RunToken))
	_, err := s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_conversation = $1", conv.IdConversation)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(ctx,
		"UPDATE agent.task SET status = 'failed', finished_at = now(), error_reason = 'heartbeat_stale' WHERE id_task = $1",
		event.Payload.IdTask)
	s.Require().NoError(err)

	agent.FailRunOrChatTurn(ctx, injector.GetNotifier(), injector.GetProjectRepository(), injector.GetIssueRepository(),
		injector.GetAgentRunRepository(), injector.GetAgentTaskRepository(), injector.GetChatTurns(), event.IdRun, constants.FailReasonHeartbeatStale)

	s.Equal(constants.PhaseCancelled, s.runPhase(event.IdRun))
	s.Equal(http.StatusUnauthorized, s.status("GET", s.issuesURL(s.P1), "", event.Payload.RunToken))
}

// A reply that arrives for a turn whose conversation vanished is not an error: the
// run is cancelled and the turn with it.
func (s *AgentConversationDispatchSuite) TestOrphanedChatTurn_LateReplyCancelsRun() {
	ctx := context.Background()
	conv := s.create(ptr(s.P1))
	event := s.startTurn(conv, "orphaned before the reply")
	_, err := s.App.Pool.Exec(ctx, "DELETE FROM agent.conversation WHERE id_conversation = $1", conv.IdConversation)
	s.Require().NoError(err)

	s.complete(event.Payload.IdTask, chatReplyBody, s.AgentKey)

	s.Equal(constants.PhaseCancelled, s.runPhase(event.IdRun))
	hasActive, err := injector.GetAgentTaskRepository().BotHasActiveTask(ctx, s.Agent)
	s.Require().NoError(err)
	s.False(hasActive, "the turn no longer holds the agent")
}
