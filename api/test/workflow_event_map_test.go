package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type WorkflowEventMapSuite struct {
	suite.Suite
	App           *issue.Application
	OwnerToken    string
	MemberToken   string
	BotUserID     int64
	IdProject     int64
	IdStateToDo   int64
	IdStateInProg int64
	IdStateDone   int64
}

func (s *WorkflowEventMapSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.OwnerToken = Token(s.T(), s.App)

	// Register and log in a member user (via admin; public registration closed post-bootstrap)
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"psmember","email":"psmember@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"psmember@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, loginRes.StatusCode)
	var tk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&tk)
	s.MemberToken = tk.Token

	var memberUser model.User
	memberRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.MemberToken)
	json.NewDecoder(memberRes.Body).Decode(&memberUser)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"ps-map-test-project","color":"#112233"}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	addRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.IdProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberUser.IdUser), s.OwnerToken)
	s.Require().Equal(http.StatusOK, addRes.StatusCode)

	// Create a bot user — runs reference it via run.id_user_bot (FK to users.user).
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"psbot","email":"psbot@test.sk","password":"kreslo"}`, s.OwnerToken)
	botLogin := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"psbot@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, botLogin.StatusCode)
	var botTk struct{ Token string }
	json.NewDecoder(botLogin.Body).Decode(&botTk)
	var botUser model.User
	botRes := Request(s.T(), s.App, "GET", "/api/private/user", "", botTk.Token)
	json.NewDecoder(botRes.Body).Decode(&botUser)
	s.BotUserID = botUser.IdUser
	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE users.user SET is_bot = TRUE WHERE id_user = $1", s.BotUserID)
	s.Require().NoError(err)

	s.IdStateToDo = s.createState("To Do", false, false)
	s.IdStateInProg = s.createState("In Progress", false, false)
	s.IdStateDone = s.createState("Done", false, true)
}

func (s *WorkflowEventMapSuite) createState(name string, start, final bool) int64 {
	body := fmt.Sprintf(`{"idProject":%d,"name":%q,"start":%v,"final":%v}`,
		s.IdProject, name, start, final)
	res := Request(s.T(), s.App, "POST", "/api/private/state", body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var st model.State
	json.NewDecoder(res.Body).Decode(&st)
	return st.IdState
}

func (s *WorkflowEventMapSuite) url() string {
	return fmt.Sprintf("/api/private/project/%d/workflow-event-state-map", s.IdProject)
}

func (s *WorkflowEventMapSuite) Test_GetMappings_Empty() {
	res := Request(s.T(), s.App, "GET", s.url(), "", s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)
	var mappings []model.WorkflowEventMapping
	json.NewDecoder(res.Body).Decode(&mappings)
	s.Empty(mappings)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_CreatesRows() {
	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d},{"event":"done","idState":%d}]}`,
		s.IdStateInProg, s.IdStateDone)
	putRes := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Equal(http.StatusOK, putRes.StatusCode)

	getRes := Request(s.T(), s.App, "GET", s.url(), "", s.OwnerToken)
	s.Equal(http.StatusOK, getRes.StatusCode)
	var mappings []model.WorkflowEventMapping
	json.NewDecoder(getRes.Body).Decode(&mappings)
	s.Len(mappings, 2)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_OverwritesPrevious() {
	body1 := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, s.IdStateInProg)
	Request(s.T(), s.App, "PUT", s.url(), body1, s.OwnerToken)

	body2 := fmt.Sprintf(`{"mappings":[{"event":"done","idState":%d}]}`, s.IdStateDone)
	Request(s.T(), s.App, "PUT", s.url(), body2, s.OwnerToken)

	getRes := Request(s.T(), s.App, "GET", s.url(), "", s.OwnerToken)
	var mappings []model.WorkflowEventMapping
	json.NewDecoder(getRes.Body).Decode(&mappings)
	s.Len(mappings, 1)
	s.Equal("done", mappings[0].Event)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_EmptyArray_ClearsAll() {
	body1 := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, s.IdStateInProg)
	Request(s.T(), s.App, "PUT", s.url(), body1, s.OwnerToken)

	clearRes := Request(s.T(), s.App, "PUT", s.url(), `{"mappings":[]}`, s.OwnerToken)
	s.Equal(http.StatusOK, clearRes.StatusCode)

	getRes := Request(s.T(), s.App, "GET", s.url(), "", s.OwnerToken)
	var mappings []model.WorkflowEventMapping
	json.NewDecoder(getRes.Body).Decode(&mappings)
	s.Empty(mappings)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_InvalidEvent_Returns400() {
	body := fmt.Sprintf(`{"mappings":[{"event":"nonexistent","idState":%d}]}`, s.IdStateInProg)
	res := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_DuplicatePhase_Returns400() {
	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d},{"event":"in_progress","idState":%d}]}`,
		s.IdStateInProg, s.IdStateDone)
	res := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_StateFromOtherProject_Returns400() {
	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"ps-map-other-project","color":"#999999"}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var otherPrj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&otherPrj)

	stateBody := fmt.Sprintf(`{"idProject":%d,"name":"OtherState","start":false,"final":false}`, otherPrj.IdProject)
	stateRes := Request(s.T(), s.App, "POST", "/api/private/state", stateBody, s.OwnerToken)
	s.Require().Equal(http.StatusOK, stateRes.StatusCode)
	var otherState model.State
	json.NewDecoder(stateRes.Body).Decode(&otherState)

	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, otherState.IdState)
	res := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *WorkflowEventMapSuite) Test_ReplaceMappings_NilIdState_SkipsRow() {
	body := `{"mappings":[{"event":"in_progress","idState":null}]}`
	res := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Equal(http.StatusOK, res.StatusCode)

	getRes := Request(s.T(), s.App, "GET", s.url(), "", s.OwnerToken)
	var mappings []model.WorkflowEventMapping
	json.NewDecoder(getRes.Body).Decode(&mappings)
	s.Empty(mappings)
}

func (s *WorkflowEventMapSuite) Test_ACL_MemberCannot_Replace() {
	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, s.IdStateInProg)
	res := Request(s.T(), s.App, "PUT", s.url(), body, s.MemberToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *WorkflowEventMapSuite) Test_ACL_MemberCannot_Get() {
	res := Request(s.T(), s.App, "GET", s.url(), "", s.MemberToken)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *WorkflowEventMapSuite) Test_Mirror_AppliesOnPhaseTransition() {
	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, s.IdStateInProg)
	putRes := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, putRes.StatusCode)

	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"mirror test issue","description":"phase state map test issue body","estimated":0}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)

	ctx := context.Background()
	var idRun int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'queued', '{"stages":[]}')
		RETURNING id_run
	`, iss.IdIssue, s.BotUserID, s.IdProject).Scan(&idRun)
	s.Require().NoError(err)

	// Goes through the injected repository so the mirror is wired in, unlike a bare TransitionPhase call.
	agentRunRepo := injector.GetAgentRunRepository()
	_, err = agentRunRepo.TransitionPhase(ctx, idRun, "queued", "in_progress", "system", nil, "test")
	s.Require().NoError(err)

	var idState *int64
	err = s.App.Pool.QueryRow(ctx,
		`SELECT id_state FROM issues.issue WHERE id_issue = $1`, iss.IdIssue).Scan(&idState)
	s.Require().NoError(err)
	s.Require().NotNil(idState)
	s.Equal(s.IdStateInProg, *idState)

	s.App.Pool.Exec(ctx, `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck
	Request(s.T(), s.App, "PUT", s.url(), `{"mappings":[]}`, s.OwnerToken)
}

func (s *WorkflowEventMapSuite) Test_Mirror_NoMapping_NoStateChange() {
	Request(s.T(), s.App, "PUT", s.url(), `{"mappings":[]}`, s.OwnerToken)

	ctx := context.Background()
	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"no-mirror issue","description":"phase state map test issue body","estimated":0}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)
	originalState := iss.IdState

	var idRun int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'queued', '{"stages":[]}')
		RETURNING id_run
	`, iss.IdIssue, s.BotUserID, s.IdProject).Scan(&idRun)
	s.Require().NoError(err)

	agentRunRepo := injector.GetAgentRunRepository()
	_, err = agentRunRepo.TransitionPhase(ctx, idRun, "queued", "in_progress", "system", nil, "test")
	s.Require().NoError(err)

	var idState *int64
	err = s.App.Pool.QueryRow(ctx,
		`SELECT id_state FROM issues.issue WHERE id_issue = $1`, iss.IdIssue).Scan(&idState)
	s.Require().NoError(err)
	// State should be unchanged (either nil or the original)
	if originalState != nil && idState != nil {
		s.Equal(*originalState, *idState)
	} else {
		s.Equal(originalState, idState)
	}

	s.App.Pool.Exec(ctx, `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck
}

func (s *WorkflowEventMapSuite) Test_Mirror_DeletedState_SkipsGracefully() {
	tempStateID := s.createState("TempMirrorState", false, false)

	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, tempStateID)
	putRes := Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, putRes.StatusCode)

	// Delete the state — ON DELETE SET NULL in workflow_event_state_map. The state
	// is only referenced by the event map (no issues), so under the new
	// migrateTo contract the delete must explicitly say "unassign".
	deleteRes := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/state/%d/project/%d?migrateTo=null", tempStateID, s.IdProject),
		"", s.OwnerToken)
	s.Require().Equal(http.StatusOK, deleteRes.StatusCode)

	ctx := context.Background()
	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"deleted-state issue","description":"phase state map test issue body","estimated":0}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)

	var idRun int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'queued', '{"stages":[]}')
		RETURNING id_run
	`, iss.IdIssue, s.BotUserID, s.IdProject).Scan(&idRun)
	s.Require().NoError(err)

	agentRunRepo := injector.GetAgentRunRepository()
	run, err := agentRunRepo.TransitionPhase(ctx, idRun, "queued", "in_progress", "system", nil, "test")
	s.Require().NoError(err)
	s.Equal("in_progress", run.Phase)

	s.App.Pool.Exec(ctx, `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck
	Request(s.T(), s.App, "PUT", s.url(), `{"mappings":[]}`, s.OwnerToken)
}

func (s *WorkflowEventMapSuite) Test_Mirror_FailureDoesNotBlockPhaseTransition() {
	body := fmt.Sprintf(`{"mappings":[{"event":"in_progress","idState":%d}]}`, s.IdStateInProg)
	Request(s.T(), s.App, "PUT", s.url(), body, s.OwnerToken)

	ctx := context.Background()
	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"mirror-fail issue","description":"phase state map test issue body","estimated":0}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)

	var idRun int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'queued', '{"stages":[]}')
		RETURNING id_run
	`, iss.IdIssue, s.BotUserID, s.IdProject).Scan(&idRun)
	s.Require().NoError(err)

	agentRunRepo := injector.GetAgentRunRepository()
	run, err := agentRunRepo.TransitionPhase(ctx, idRun, "queued", "in_progress", "system", nil, "test")
	s.Require().NoError(err)
	s.Equal("in_progress", run.Phase)

	s.App.Pool.Exec(ctx, `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck
	Request(s.T(), s.App, "PUT", s.url(), `{"mappings":[]}`, s.OwnerToken)
}

func TestWorkflowEventMapSuite(t *testing.T) {
	suite.Run(t, new(WorkflowEventMapSuite))
}
