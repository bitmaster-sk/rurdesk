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

	"github.com/gorilla/websocket"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type AgentThinkingSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	BotApiKey     string
	BotUserID     int64
	IdProject     int64
	IdIssuePublic int64
}

func (s *AgentThinkingSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"thinkbot","email":"thinkbot@test.sk","password":"kreslo"}`, s.Token)
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"thinkbot@test.sk","password":"kreslo"}`, "")
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
		`{"name":"thinking-bot-key"}`, s.Token)
	s.Require().Equal(http.StatusOK, keyRes.StatusCode)
	var apiKey model.CreateApiKeyRes
	json.NewDecoder(keyRes.Body).Decode(&apiKey)
	s.BotApiKey = apiKey.RawKey

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"thinking-test-project","color":"#aabbcc"}`, s.Token)
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
		`{"title":"thinking test issue","description":"thinking test issue body","estimated":0}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *AgentThinkingSuite) TearDownSuite() {
	s.purgeRuns()
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *AgentThinkingSuite) purgeRuns() {
	_, err := s.App.Pool.Exec(context.Background(), `
		DELETE FROM agent.task
		WHERE id_run IN (SELECT id_run FROM agent.run WHERE id_project = $1)`,
		s.IdProject)
	s.Require().NoError(err)

	_, err = s.App.Pool.Exec(context.Background(),
		"DELETE FROM agent.run WHERE id_project = $1", s.IdProject)
	s.Require().NoError(err)
}

func (s *AgentThinkingSuite) insertRunWithActiveTask() (idRun int64, idTask int64) {
	s.purgeRuns()

	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run(id_issue, id_user_bot, id_project, phase, stage_plan)
		SELECT id_issue, $1, $2, 'in_progress', '{"stages":[]}'
		FROM issues.issue WHERE id_issue_public = $3 AND id_project = $2
		RETURNING id_run`,
		s.BotUserID, s.IdProject, s.IdIssuePublic,
	).Scan(&idRun)
	s.Require().NoError(err)

	err = s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.task(id_run, id_user_bot, stage, attempt_no, status, last_heartbeat_at)
		VALUES ($1, $2, 'implementation', 1, 'active', now() - interval '5 minutes')
		RETURNING id_task`,
		idRun, s.BotUserID,
	).Scan(&idTask)
	s.Require().NoError(err)

	return idRun, idTask
}

func (s *AgentThinkingSuite) postThinking(idTask int64, body, token string) *http.Response {
	return Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/thinking", idTask), body, token)
}

func (s *AgentThinkingSuite) chunkCount(idTask int64) int {
	var count int
	err := s.App.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM agent.task_thinking WHERE id_task = $1", idTask).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *AgentThinkingSuite) Test_ForeignUserRejected() {
	_, idTask := s.insertRunWithActiveTask()

	res := s.postThinking(idTask,
		`{"seq":1,"events":[{"kind":"thinking","text":"injected","at":1}]}`, s.Token)

	s.Equal(http.StatusForbidden, res.StatusCode)
	s.Equal(0, s.chunkCount(idTask))
}

func (s *AgentThinkingSuite) Test_BatchIsStored() {
	_, idTask := s.insertRunWithActiveTask()

	res := s.postThinking(idTask, `{"seq":1,"events":[
		{"kind":"thinking","text":"weighing the tokenizer options","at":1},
		{"kind":"tool","tool":"developer__shell","text":"npm run test:unit","at":2},
		{"kind":"tool","tool":"developer__text_editor","at":3}
	]}`, s.BotApiKey)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	rows, err := s.App.Pool.Query(context.Background(),
		"SELECT kind, tool, text FROM agent.task_thinking WHERE id_task = $1 ORDER BY seq, event_index", idTask)
	s.Require().NoError(err)
	defer rows.Close()

	var stored []string
	for rows.Next() {
		var kind, tool, text string
		s.Require().NoError(rows.Scan(&kind, &tool, &text))
		stored = append(stored, kind+"|"+tool+"|"+text)
	}
	s.Equal([]string{
		"thinking||weighing the tokenizer options",
		"tool|developer__shell|npm run test:unit",
		"tool|developer__text_editor|",
	}, stored)
}

func (s *AgentThinkingSuite) Test_RepeatedSeqIsIdempotent() {
	_, idTask := s.insertRunWithActiveTask()
	body := `{"seq":7,"events":[{"kind":"thinking","text":"same batch","at":1}]}`

	s.Require().Equal(http.StatusOK, s.postThinking(idTask, body, s.BotApiKey).StatusCode)
	s.Require().Equal(http.StatusOK, s.postThinking(idTask, body, s.BotApiKey).StatusCode)

	s.Equal(1, s.chunkCount(idTask))
}

// The gateway resends a batch under its original seq when the send failed but
// the tracker had already committed it. Rows dedup on their key; the tail has
// no key, so without a guard the reader sees the sentence twice.
func (s *AgentThinkingSuite) Test_RepeatedSeqDoesNotDuplicateTheTail() {
	_, idTask := s.insertRunWithActiveTask()
	body := `{"seq":1,"events":[{"kind":"thinking","text":"one thought","at":1}]}`

	s.Require().Equal(http.StatusOK, s.postThinking(idTask, body, s.BotApiKey).StatusCode)
	s.Require().Equal(http.StatusOK, s.postThinking(idTask, body, s.BotApiKey).StatusCode)

	completeRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/agent/task/%d/complete", idTask),
		`{"outcome":"output_submitted"}`, s.BotApiKey)
	s.Require().Equal(http.StatusOK, completeRes.StatusCode)

	var tail *string
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT thinking_tail FROM agent.task WHERE id_task = $1", idTask).Scan(&tail))
	s.Require().NotNil(tail)
	s.Equal("one thought", *tail)
}

// Once a stage is marked truncated the cap has to latch. A smaller batch that
// still fits under the cap would otherwise land behind the marker, and the
// replay would read as complete past the point where it stopped being so.
func (s *AgentThinkingSuite) Test_TruncationLatchesAgainstASmallerBatch() {
	_, idTask := s.insertRunWithActiveTask()

	for seq := 1; seq <= 12; seq++ {
		s.Require().Equal(http.StatusOK, s.postThinking(idTask,
			fmt.Sprintf(`{"seq":%d,"events":[{"kind":"thinking","text":%q,"at":1}]}`,
				seq, strings.Repeat("x", 100*1024)), s.BotApiKey).StatusCode)
	}
	countAfterCap := s.chunkCount(idTask)

	s.Require().Equal(http.StatusOK, s.postThinking(idTask,
		`{"seq":13,"events":[{"kind":"thinking","text":"tiny","at":1}]}`, s.BotApiKey).StatusCode)

	s.Equal(countAfterCap, s.chunkCount(idTask), "no batch may land behind the marker")
	var tailKind string
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(), `
		SELECT kind FROM agent.task_thinking WHERE id_task = $1
		ORDER BY seq DESC, event_index DESC LIMIT 1`, idTask).Scan(&tailKind))
	s.Equal("truncated", tailKind, "the marker must be the last thing the reader sees")
}

func (s *AgentThinkingSuite) Test_BatchRefreshesHeartbeat() {
	_, idTask := s.insertRunWithActiveTask()

	var before time.Time
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT last_heartbeat_at FROM agent.task WHERE id_task = $1", idTask).Scan(&before))

	s.Require().Equal(http.StatusOK, s.postThinking(idTask,
		`{"seq":1,"events":[{"kind":"thinking","text":"still working","at":1}]}`, s.BotApiKey).StatusCode)

	var after time.Time
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT last_heartbeat_at FROM agent.task WHERE id_task = $1", idTask).Scan(&after))

	s.True(after.After(before), "posting thinking must refresh the heartbeat")
}

func (s *AgentThinkingSuite) Test_PerTaskCapStopsGrowth() {
	_, idTask := s.insertRunWithActiveTask()

	chunk := strings.Repeat("x", 100*1024)
	for seq := 1; seq <= 12; seq++ {
		res := s.postThinking(idTask,
			fmt.Sprintf(`{"seq":%d,"events":[{"kind":"thinking","text":%q,"at":1}]}`, seq, chunk),
			s.BotApiKey)
		s.Require().Equal(http.StatusOK, res.StatusCode, "the cap must not turn into an error")
	}

	var totalBytes int
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT coalesce(sum(octet_length(text) + octet_length(tool)), 0) FROM agent.task_thinking WHERE id_task = $1 AND kind <> 'truncated'",
		idTask).Scan(&totalBytes))
	s.LessOrEqual(totalBytes, 1024*1024)

	var markers int
	s.Require().NoError(s.App.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM agent.task_thinking WHERE id_task = $1 AND kind = 'truncated'",
		idTask).Scan(&markers))
	s.Equal(1, markers, "exactly one truncation marker, however many batches follow")
}

type wsThinkingNotice struct {
	Subject string `json:"subject"`
	Payload struct {
		IdRun  int64  `json:"idRun"`
		IdTask int64  `json:"idTask"`
		Stage  string `json:"stage"`
		Seq    int    `json:"seq"`
		Events []struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
			Tool string `json:"tool"`
		} `json:"events"`
	} `json:"payload"`
}

func (s *AgentThinkingSuite) Test_BatchIsBroadcastAsDelta() {
	idRun, idTask := s.insertRunWithActiveTask()

	server := httptest.NewServer(s.App)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/private/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {s.Token}})
	s.Require().NoError(err, "websocket connect")
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)
	s.Require().Equal(http.StatusOK, s.postThinking(idTask,
		`{"seq":3,"events":[{"kind":"thinking","text":"measuring the tokenizer","at":1}]}`,
		s.BotApiKey).StatusCode)

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, raw, err := conn.ReadMessage()
		s.Require().NoError(err, "no agent_thinking notice for the posted batch")

		var notice wsThinkingNotice
		if json.Unmarshal(raw, &notice) != nil || notice.Subject != "agent_thinking" ||
			notice.Payload.IdTask != idTask {
			continue
		}
		s.Equal(idRun, notice.Payload.IdRun)
		s.Equal("implementation", notice.Payload.Stage)
		s.Equal(3, notice.Payload.Seq)
		s.Require().Len(notice.Payload.Events, 1)
		s.Equal("thinking", notice.Payload.Events[0].Kind)
		s.Equal("measuring the tokenizer", notice.Payload.Events[0].Text)
		return
	}
}

func TestAgentThinkingSuite(t *testing.T) {
	suite.Run(t, new(AgentThinkingSuite))
}
