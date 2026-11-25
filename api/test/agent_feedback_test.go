package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// AgentFeedbackSuite: comments never start the agent; Send to agent hands it every
// comment it has not seen yet, and the agent is told which ones those are.
type AgentFeedbackSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	IdAgent   int64
	IdHuman   int64
	IdProject int64
	events    chan gatewayEvent
	gateway   *httptest.Server
}

type gatewayEvent struct {
	Event   string          `json:"event"`
	IdRun   int64           `json:"idRun"`
	Payload json.RawMessage `json:"payload"`
}

type feedbackStagePayload struct {
	IdTask        int64 `json:"idTask"`
	ContextBundle struct {
		IdMessageSeen *int64           `json:"idMessageSeen"`
		ReviewThread  []*model.Message `json:"reviewThread"`
	} `json:"contextBundle"`
}

func (s *AgentFeedbackSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)
	s.events = make(chan gatewayEvent, 64)
	s.gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var event gatewayEvent
		if json.Unmarshal(body, &event) == nil {
			s.events <- event
		}
		w.WriteHeader(http.StatusOK)
	}))

	res := Request(s.T(), s.App, "POST", "/api/private/admin/user", `{"name":"feedbackbot","isAgent":true}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var created struct {
		IdUser int64 `json:"idUser"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&created))
	s.IdAgent = created.IdUser

	createUserAsAdmin(s.T(), s.App, s.Token, `{"name":"feedbackhuman","email":"feedbackhuman@test.sk","password":"kreslo1"}`)
	s.IdHuman = idOfUser(s.T(), s.App, s.Token, "feedbackhuman@test.sk")

	s.IdProject = createProject(s.T(), s.App, s.Token, "agent-feedback-project")
	ensureGitIntegration(s.T(), s.App, s.Token, s.IdProject)
	for _, idUser := range []int64{s.IdAgent, s.IdHuman} {
		addMember(s.T(), s.App, s.Token, s.IdProject, idUser)
	}
	gw := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/admin/user/%d/gateway", s.IdAgent),
		fmt.Sprintf(`{"gatewayUrl":%q}`, s.gateway.URL), s.Token)
	s.Require().Equal(http.StatusOK, gw.StatusCode)
}

func (s *AgentFeedbackSuite) TearDownSuite() {
	ctx := context.Background()
	s.gateway.Close()
	s.App.Pool.Exec(ctx, "DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM agent.gateway WHERE id_user_agent = $1", s.IdAgent)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user IN ($1, $2)", s.IdAgent, s.IdHuman)
}

func (s *AgentFeedbackSuite) SetupTest() {
	for {
		select {
		case <-s.events:
		default:
			return
		}
	}
}

// startRun assigns the agent and returns its run, in progress with the given stages completed.
func (s *AgentFeedbackSuite) startRun(title string, completedStages ...string) (model.Issue, *model.AgentRun) {
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"body","estimated":0}`, title), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&iss))

	assign := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/assign-agent", s.IdProject, iss.IdIssuePublic),
		fmt.Sprintf(`{"idUserAgent":%d}`, s.IdAgent), s.Token)
	s.Require().Equal(http.StatusOK, assign.StatusCode)
	var run model.AgentRun
	s.Require().NoError(json.NewDecoder(assign.Body).Decode(&run))
	for _, stage := range completedStages {
		task, err := injector.GetAgentTaskRepository().Insert(context.Background(), run.IdRun, s.IdAgent, stage, 1)
		s.Require().NoError(err)
		s.setTaskStatus(task.IdTask, constants.TaskStatusCompleted)
	}
	s.setPhase(run.IdRun, constants.PhaseInProgress)
	return iss, s.loadRun(run.IdRun)
}

func (s *AgentFeedbackSuite) loadRun(idRun int64) *model.AgentRun {
	run, err := injector.GetAgentRunRepository().LoadById(context.Background(), idRun)
	s.Require().NoError(err)
	return run
}

func (s *AgentFeedbackSuite) tasksOf(idRun int64) []*model.AgentTask {
	tasks, err := injector.GetAgentTaskRepository().LoadByRun(context.Background(), idRun)
	s.Require().NoError(err)
	return tasks
}

func (s *AgentFeedbackSuite) setPhase(idRun int64, phase string) {
	_, err := s.App.Pool.Exec(context.Background(), "UPDATE agent.run SET phase = $2 WHERE id_run = $1", idRun, phase)
	s.Require().NoError(err)
}

func (s *AgentFeedbackSuite) setTaskStatus(idTask int64, status string) {
	_, err := s.App.Pool.Exec(context.Background(), "UPDATE agent.task SET status = $2 WHERE id_task = $1", idTask, status)
	s.Require().NoError(err)
}

// dispatch sends the task to the gateway and returns the stage payload it received.
func (s *AgentFeedbackSuite) dispatch(task *model.AgentTask) feedbackStagePayload {
	injector.GetDispatcher().DispatchStage(context.Background(), &agent.Dispatch{Run: s.loadRun(task.IdRun), Task: task})
	for {
		select {
		case event := <-s.events:
			if event.Event != "stage_execute" {
				continue
			}
			var payload feedbackStagePayload
			s.Require().NoError(json.Unmarshal(event.Payload, &payload))
			if payload.IdTask == task.IdTask {
				return payload
			}
		case <-time.After(10 * time.Second):
			s.FailNow("the gateway never received the stage")
		}
	}
}

// dispatchStage starts the next attempt of the stage and returns it with its payload.
func (s *AgentFeedbackSuite) dispatchStage(idRun int64, stage string) (*model.AgentTask, feedbackStagePayload) {
	task, err := injector.GetAgentTaskRepository().Insert(context.Background(), idRun, s.IdAgent, stage,
		agent.ResolveNextAttemptNo(s.tasksOf(idRun), stage))
	s.Require().NoError(err)
	s.setTaskStatus(task.IdTask, constants.TaskStatusActive)
	return task, s.dispatch(task)
}

// finishAttempt completes the task and leaves the run waiting for a person in the phase.
func (s *AgentFeedbackSuite) finishAttempt(task *model.AgentTask, phase string) {
	s.setTaskStatus(task.IdTask, constants.TaskStatusCompleted)
	s.setPhase(task.IdRun, phase)
}

func (s *AgentFeedbackSuite) postMessage(idIssue int64, fields map[string]any) model.Message {
	payload := map[string]any{"idRecipient": idIssue, "idMessageRecipientType": 4}
	for key, value := range fields {
		payload[key] = value
	}
	body, err := json.Marshal(payload)
	s.Require().NoError(err)
	res := Request(s.T(), s.App, "POST", "/api/private/message", string(body), s.Token)
	resBody := readBody(s.T(), res)
	s.Require().Equal(http.StatusOK, res.StatusCode, resBody)
	var message model.Message
	s.Require().NoError(json.Unmarshal([]byte(resBody), &message))
	return message
}

func (s *AgentFeedbackSuite) comment(idIssue int64, text string) model.Message {
	return s.postMessage(idIssue, map[string]any{"message": text})
}

func (s *AgentFeedbackSuite) agentComment(idIssue int64, text string) *model.Message {
	agentUser, err := injector.GetUserRepository().LoadUser(context.Background(), s.IdAgent)
	s.Require().NoError(err)
	message, err := injector.GetMessageRepository().InsertIssueAgentMessage(context.Background(), text, agentUser, idIssue, constants.MessageKindComment)
	s.Require().NoError(err)
	return message
}

func (s *AgentFeedbackSuite) newestMessageId(idIssue int64) int64 {
	var idMessage int64
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT COALESCE(MAX(id_message), 0) FROM messages.issue_message WHERE id_issue_to = $1", idIssue).Scan(&idMessage))
	return idMessage
}

func (s *AgentFeedbackSuite) snapshot(idRun int64) model.AgentRunWithEvents {
	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/agent/run/%d", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var snapshot model.AgentRunWithEvents
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&snapshot))
	return snapshot
}

func (s *AgentFeedbackSuite) newFeedbackCount(idIssue, idRun int64) int {
	snapshot := s.snapshot(idRun)
	s.Require().NotNil(snapshot.IdMessageSeen)
	count, err := injector.GetMessageRepository().CountNewFeedback(context.Background(), idIssue, *snapshot.IdMessageSeen)
	s.Require().NoError(err)
	return count
}

func (s *AgentFeedbackSuite) sendFeedback(idRun int64, token string) *http.Response {
	return Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/agent/run/%d/feedback", idRun), "", token)
}

// waitingForReview returns a run whose plan waits for review, the plan's attempt and its payload.
func (s *AgentFeedbackSuite) waitingForReview(title string) (model.Issue, *model.AgentRun, *model.AgentTask) {
	iss, run := s.startRun(title, constants.StagePickup, constants.StageBrainstorming, constants.StageDesign)
	s.comment(iss.IdIssue, "Plan the history table.")
	plan, _ := s.dispatchStage(run.IdRun, constants.StageImplementationPlan)
	s.finishAttempt(plan, constants.PhaseAwaitingApproval)
	return iss, run, plan
}

// T1 · what the agent has seen

func (s *AgentFeedbackSuite) TestSnapshotHoldsTheConversationTheAttemptWasGiven() {
	iss, run := s.startRun("feedback-watermark", constants.StagePickup, constants.StageBrainstorming, constants.StageDesign)
	s.comment(iss.IdIssue, "Plan the history table.")
	newestAtDispatch := s.newestMessageId(iss.IdIssue)

	plan, _ := s.dispatchStage(run.IdRun, constants.StageImplementationPlan)
	s.finishAttempt(plan, constants.PhaseAwaitingApproval)

	snapshot := s.snapshot(run.IdRun)
	s.Require().NotNil(snapshot.IdMessageSeen)
	s.Equal(newestAtDispatch, *snapshot.IdMessageSeen)
	s.Zero(s.newFeedbackCount(iss.IdIssue, run.IdRun))
}

func (s *AgentFeedbackSuite) TestPeoplesCommentsAndRepliesCountAsNewAgentMessagesDoNot() {
	iss, run, _ := s.waitingForReview("feedback-count")
	parent := s.comment(iss.IdIssue, "line one\nline two")
	s.postMessage(iss.IdIssue, map[string]any{"message": "about line two", "idParentMessage": parent.IdMessage, "anchorLineStart": 2, "anchorLineEnd": 2})
	s.agentComment(iss.IdIssue, "Noted, waiting for the send.")

	s.Equal(2, s.newFeedbackCount(iss.IdIssue, run.IdRun), "the comment and the anchored reply, not the agent's note")
}

func (s *AgentFeedbackSuite) TestACommentWrittenWhileTheAgentWorksIsNewOnceItFinishes() {
	iss, run := s.startRun("feedback-during-attempt", constants.StagePickup, constants.StageBrainstorming, constants.StageDesign)
	plan, _ := s.dispatchStage(run.IdRun, constants.StageImplementationPlan)
	s.comment(iss.IdIssue, "One more requirement.")
	s.finishAttempt(plan, constants.PhaseAwaitingApproval)

	s.Equal(1, s.newFeedbackCount(iss.IdIssue, run.IdRun))
	s.Equal(http.StatusOK, s.sendFeedback(run.IdRun, s.Token).StatusCode)
}

func (s *AgentFeedbackSuite) TestIssueMessagesTellAgentAuthorsApart() {
	iss, _ := s.startRun("feedback-is-agent")
	human := s.comment(iss.IdIssue, "from a person")
	bot := s.agentComment(iss.IdIssue, "from the agent")

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/message?idRecipient=%d&idMessageRecipientType=4", iss.IdIssue), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var messages []*model.Message
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&messages))
	isAgentById := map[int64]bool{}
	for _, message := range messages {
		s.Require().NotNil(message.Creator)
		isAgentById[message.IdMessage] = message.Creator.IsAgent
	}
	s.False(isAgentById[human.IdMessage])
	s.True(isAgentById[bot.IdMessage])
}

// T2 · comments no longer start the agent; Send

func (s *AgentFeedbackSuite) TestACommentStartsNothing() {
	for _, phase := range []string{constants.PhaseAwaitingApproval, constants.PhaseAwaitingInput, constants.PhasePrOpen} {
		s.Run(phase, func() {
			iss, run, _ := s.waitingForReview("feedback-comment-" + phase)
			s.setPhase(run.IdRun, phase)
			tasksBefore := len(s.tasksOf(run.IdRun))

			s.comment(iss.IdIssue, "a thought")

			s.Equal(phase, s.loadRun(run.IdRun).Phase)
			s.Len(s.tasksOf(run.IdRun), tasksBefore)
		})
	}
}

func (s *AgentFeedbackSuite) TestACommentWhileAPersonHoldsTheIssueStartsNothing() {
	iss, run, _ := s.waitingForReview("feedback-uc7")
	res := Request(s.T(), s.App, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic),
		fmt.Sprintf(`{"idProject":%d,"assignedTo":%d}`, s.IdProject, s.IdHuman), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.comment(iss.IdIssue, "while a person holds it")

	s.Equal(constants.PhaseAwaitingApproval, s.loadRun(run.IdRun).Phase)
	s.Equal(1, s.newFeedbackCount(iss.IdIssue, run.IdRun))
}

func (s *AgentFeedbackSuite) TestSendQueuesARevisionThatSeesEveryNewComment() {
	iss, run, plan := s.waitingForReview("feedback-send")
	first := s.comment(iss.IdIssue, "Use option 1.")
	second := s.comment(iss.IdIssue, "And keep the filter by field.")

	res := s.sendFeedback(run.IdRun, s.Token)

	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.Equal(constants.PhaseQueued, s.loadRun(run.IdRun).Phase)
	var revision *model.AgentTask
	for _, task := range s.tasksOf(run.IdRun) {
		if task.Status == constants.TaskStatusPending {
			revision = task
		}
	}
	s.Require().NotNil(revision, "Send creates the next attempt")
	s.Equal(constants.StageImplementationPlan, revision.Stage)
	s.Equal(2, revision.AttemptNo)

	payload := s.dispatch(revision)
	idsInThread := map[int64]bool{}
	for _, message := range payload.ContextBundle.ReviewThread {
		idsInThread[message.IdMessage] = true
	}
	s.True(idsInThread[first.IdMessage] && idsInThread[second.IdMessage], "the attempt gets every comment written before the send")

	// T3: the revision knows what the previous attempt saw, so the gateway can mark the new comments.
	reloadedPlan, err := injector.GetAgentTaskRepository().LoadById(context.Background(), plan.IdTask)
	s.Require().NoError(err)
	s.Require().NotNil(payload.ContextBundle.IdMessageSeen)
	s.Require().NotNil(reloadedPlan.IdMessageSeen)
	s.Equal(*reloadedPlan.IdMessageSeen, *payload.ContextBundle.IdMessageSeen)
	s.Less(*payload.ContextBundle.IdMessageSeen, first.IdMessage)
}

func (s *AgentFeedbackSuite) TestAFirstAttemptCarriesNoWatermark() {
	_, run := s.startRun("feedback-first-attempt", constants.StagePickup)
	_, payload := s.dispatchStage(run.IdRun, constants.StageBrainstorming)

	s.Nil(payload.ContextBundle.IdMessageSeen)
}

func (s *AgentFeedbackSuite) TestSendRefusals() {
	s.Run("nothing new", func() {
		_, run, _ := s.waitingForReview("feedback-nothing-new")
		res := s.sendFeedback(run.IdRun, s.Token)
		s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
		s.Equal("NO_NEW_FEEDBACK", errorCode(s.T(), res))
	})
	s.Run("the agent is working", func() {
		iss, run := s.startRun("feedback-working", constants.StagePickup)
		s.comment(iss.IdIssue, "early thought")
		res := s.sendFeedback(run.IdRun, s.Token)
		s.Equal(http.StatusConflict, res.StatusCode)
		s.Equal("RUN_NOT_AWAITING_FEEDBACK", errorCode(s.T(), res))
	})
	s.Run("already sent", func() {
		iss, run, _ := s.waitingForReview("feedback-twice")
		s.comment(iss.IdIssue, "change it")
		s.Require().Equal(http.StatusOK, s.sendFeedback(run.IdRun, s.Token).StatusCode)
		res := s.sendFeedback(run.IdRun, s.Token)
		s.Equal(http.StatusConflict, res.StatusCode)
		s.Equal("RUN_NOT_AWAITING_FEEDBACK", errorCode(s.T(), res))
	})
	s.Run("a person holds the issue", func() {
		iss, run, _ := s.waitingForReview("feedback-parked")
		res := Request(s.T(), s.App, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic),
			fmt.Sprintf(`{"idProject":%d,"assignedTo":%d}`, s.IdProject, s.IdHuman), s.Token)
		s.Require().Equal(http.StatusOK, res.StatusCode)
		s.comment(iss.IdIssue, "for later")
		sent := s.sendFeedback(run.IdRun, s.Token)
		s.Equal(http.StatusConflict, sent.StatusCode)
		s.Equal("RUN_PARKED", errorCode(s.T(), sent))
	})
	s.Run("no right to update the issue", func() {
		iss, run, _ := s.waitingForReview("feedback-acl")
		s.comment(iss.IdIssue, "change it")
		outsider := createUserAndLogin(s.T(), s.App, s.Token, "feedback-outsider@test.sk")
		s.Equal(http.StatusForbidden, s.sendFeedback(run.IdRun, outsider).StatusCode)
		s.Equal(constants.PhaseAwaitingApproval, s.loadRun(run.IdRun).Phase)
	})
}

func (s *AgentFeedbackSuite) TestConcurrentSendsCreateOneAttempt() {
	iss, run, _ := s.waitingForReview("feedback-concurrent")
	s.comment(iss.IdIssue, "change it")
	tasksBefore := len(s.tasksOf(run.IdRun))

	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- s.sendFeedback(run.IdRun, s.Token).StatusCode
		}()
	}
	wg.Wait()
	close(statuses)

	var codes []int
	for code := range statuses {
		codes = append(codes, code)
	}
	s.ElementsMatch([]int{http.StatusOK, http.StatusConflict}, codes)
	s.Len(s.tasksOf(run.IdRun), tasksBefore+1)
}

// T5 · unassigning stops only the cancelled attempt

func (s *AgentFeedbackSuite) TestUnassignTellsTheGatewayWhichAttemptStopped() {
	iss, run := s.startRun("feedback-cancel-ids", constants.StagePickup)
	active, _ := s.dispatchStage(run.IdRun, constants.StageBrainstorming)

	res := Request(s.T(), s.App, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic),
		fmt.Sprintf(`{"idProject":%d,"assignedTo":%d}`, s.IdProject, s.IdHuman), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	for {
		select {
		case event := <-s.events:
			if event.Event != "cancelled" || event.IdRun != run.IdRun {
				continue
			}
			var payload struct {
				IdsTask []int64 `json:"idsTask"`
			}
			s.Require().NoError(json.Unmarshal(event.Payload, &payload))
			s.Equal([]int64{active.IdTask}, payload.IdsTask)
			return
		case <-time.After(10 * time.Second):
			s.FailNow("the gateway never received the cancelled event")
		}
	}
}

func Test_AgentFeedbackSuite(t *testing.T) {
	suite.Run(t, new(AgentFeedbackSuite))
}
