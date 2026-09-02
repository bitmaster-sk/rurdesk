package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type AgentThinkingPersistenceSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	BotApiKey     string
	BotUserID     int64
	IdProject     int64
	IdIssuePublic int64
}

func (s *AgentThinkingPersistenceSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"persistbot","email":"persistbot@test.sk","password":"kreslo"}`, s.Token)
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"persistbot@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, loginRes.StatusCode)
	var tk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&tk)
	botToken := tk.Token

	botUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", botToken)
	var botUser model.User
	json.NewDecoder(botUserRes.Body).Decode(&botUser)
	s.BotUserID = botUser.IdUser

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE users.user SET is_bot = TRUE WHERE id_user = $1", s.BotUserID)
	s.Require().NoError(err)
	s.App.Cache.Del(context.Background(), botToken)

	keyRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/admin/user/%d/api-key", s.BotUserID),
		`{"name":"persist-bot-key"}`, s.Token)
	s.Require().Equal(http.StatusOK, keyRes.StatusCode)
	var apiKey model.CreateApiKeyRes
	json.NewDecoder(keyRes.Body).Decode(&apiKey)
	s.BotApiKey = apiKey.RawKey

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"persist-test-project","color":"#bbccdd"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.BotUserID), s.Token)

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"persist test issue","description":"persist test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *AgentThinkingPersistenceSuite) TearDownSuite() {
	s.setPersistence(true)
	s.purgeRuns()
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *AgentThinkingPersistenceSuite) purgeRuns() {
	_, err := s.App.Pool.Exec(context.Background(), `
		DELETE FROM agent.task
		WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)`,
		s.IdProject)
	s.Require().NoError(err)
	_, err = s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.Require().NoError(err)
}

func (s *AgentThinkingPersistenceSuite) setPersistence(isPersisted bool) {
	res := Request(s.T(), s.App, "PATCH", "/api/private/admin/settings",
		fmt.Sprintf(`{"isAgentThinkingPersisted":%t}`, isPersisted), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *AgentThinkingPersistenceSuite) insertRunWithActiveTask() (idRun int64, idTask int64) {
	s.purgeRuns()

	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, stage_plan)
		SELECT id_issue, $1, $2, 'in_progress', '{"stages":[{"name":"implementation","skippable":false,"skip":false}]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)

	err = s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.task(id_run, id_user_bot, stage, attempt_no, status, started_at)
		VALUES ($1, $2, 'implementation', 1, 'active', now())
		RETURNING id_task`,
		idRun, s.BotUserID,
	).Scan(&idTask)
	s.Require().NoError(err)

	return idRun, idTask
}

func (s *AgentThinkingPersistenceSuite) postThinking(idTask int64, seq int, text string) {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/thinking", idTask),
		fmt.Sprintf(`{"seq":%d,"events":[{"kind":"thinking","text":%q,"at":1}]}`, seq, text),
		s.BotApiKey)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *AgentThinkingPersistenceSuite) completeStage(idTask int64) {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/complete", idTask),
		`{"outcome":"output_submitted"}`, s.BotApiKey)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *AgentThinkingPersistenceSuite) chunkCount(idTask int64) int {
	var count int
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM agent.task_thinking WHERE id_task = $1", idTask).Scan(&count))
	return count
}

func (s *AgentThinkingPersistenceSuite) readStageThinking(idRun int64) (events []model.AgentThinkingEvent, isComplete bool) {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/agent/run/%d/thinking?stage=implementation", idRun), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	body, _ := io.ReadAll(res.Body)
	var readRes model.AgentThinkingRes
	s.Require().NoError(json.Unmarshal(body, &readRes))
	return readRes.Events, readRes.IsComplete
}

func (s *AgentThinkingPersistenceSuite) taskThinking(idTask int64) (hasBlob bool, tail string) {
	var blobLen int
	var storedTail *string
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT coalesce(length(thinking_blob), 0), thinking_tail FROM agent.task WHERE id_task = $1",
		idTask).Scan(&blobLen, &storedTail))
	if storedTail != nil {
		tail = *storedTail
	}
	return blobLen > 0, tail
}

func (s *AgentThinkingPersistenceSuite) Test_PersistenceOn_CompactsToBlob() {
	s.setPersistence(true)
	idRun, idTask := s.insertRunWithActiveTask()

	s.postThinking(idTask, 1, "first I read the formatter")
	s.postThinking(idTask, 2, "then I wrote the tokenizer")
	s.Require().Equal(2, s.chunkCount(idTask))

	s.completeStage(idTask)

	s.Equal(0, s.chunkCount(idTask), "chunks must be compacted away at complete")
	hasBlob, tail := s.taskThinking(idTask)
	s.True(hasBlob, "the full text must be kept as a blob")
	s.NotEmpty(tail, "the tail is written in both modes")

	events, isComplete := s.readStageThinking(idRun)
	s.True(isComplete)
	s.Equal([]model.AgentThinkingEvent{
		{Kind: model.ThinkingKindThinking, Text: "first I read the formatter", At: 1},
		{Kind: model.ThinkingKindThinking, Text: "then I wrote the tokenizer", At: 1},
	}, events)
}

func (s *AgentThinkingPersistenceSuite) Test_PersistenceOn_ReplaysToolCallsStructured() {
	s.setPersistence(true)
	idRun, idTask := s.insertRunWithActiveTask()

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/thinking", idTask), `{"seq":1,"events":[
			{"kind":"thinking","text":"→ shell returns nil here","at":1},
			{"kind":"tool","tool":"developer__shell","text":"rg --files src","at":2}
		]}`, s.BotApiKey)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.completeStage(idTask)

	events, _ := s.readStageThinking(idRun)
	s.Equal([]model.AgentThinkingEvent{
		{Kind: model.ThinkingKindThinking, Text: "→ shell returns nil here", At: 1},
		{Kind: model.ThinkingKindTool, Tool: "developer__shell", Text: "rg --files src", At: 2},
	}, events)
}

func (s *AgentThinkingPersistenceSuite) Test_PersistenceOff_KeepsOnlyTail() {
	s.setPersistence(false)
	idRun, idTask := s.insertRunWithActiveTask()

	s.postThinking(idTask, 1, strings.Repeat("a", 2000))
	s.postThinking(idTask, 2, "the tail that fits in the cap")
	s.Equal(0, s.chunkCount(idTask), "nothing is persisted per batch while the switch is off")

	s.completeStage(idTask)

	hasBlob, tail := s.taskThinking(idTask)
	s.False(hasBlob, "no full text is kept while the switch is off")
	s.LessOrEqual(len(tail), 1024, "the tail is capped at 1 KB")
	s.Contains(tail, "the tail that fits in the cap")

	// The tail travels as one thinking event, so the feed renders a stage the
	// same way whether the full thinking was kept or not.
	events, isComplete := s.readStageThinking(idRun)
	s.False(isComplete, "a tail must not pass as the full thinking")
	s.Require().Len(events, 1)
	s.Equal(model.ThinkingKindThinking, events[0].Kind)
	s.Contains(events[0].Text, "the tail that fits in the cap")
}

func (s *AgentThinkingPersistenceSuite) Test_SwitchAppliesToNextBatch() {
	s.setPersistence(false)
	_, idTask := s.insertRunWithActiveTask()

	s.postThinking(idTask, 1, "while off")
	s.Require().Equal(0, s.chunkCount(idTask))

	s.setPersistence(true)
	s.postThinking(idTask, 2, "after the flip")

	s.Equal(1, s.chunkCount(idTask), "the next batch must follow the new setting")
}

func (s *AgentThinkingPersistenceSuite) Test_StageWithoutThinkingCompletesCleanly() {
	s.setPersistence(true)
	_, idTask := s.insertRunWithActiveTask()

	s.completeStage(idTask)

	hasBlob, tail := s.taskThinking(idTask)
	s.False(hasBlob)
	s.Empty(tail)
}

func TestAgentThinkingPersistenceSuite(t *testing.T) {
	suite.Run(t, new(AgentThinkingPersistenceSuite))
}
