package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type MergePollerManualSuite struct {
	suite.Suite
	App              *issue.Application
	OwnerToken       string
	IdProject        int64
	IdStateToDo      int64
	IdStateInProg    int64
	IdStateDone      int64
	IdGitIntegration int64
	BotUserID        int64
	poller           *agent.MergePoller
}

func (s *MergePollerManualSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.OwnerToken = Token(s.T(), s.App)

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"ps-manual-merge-project","color":"#445566"}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	s.IdStateToDo = s.createState("To Do", false, false)
	s.IdStateInProg = s.createState("In Progress", false, false)
	s.IdStateDone = s.createState("Done", false, true)

	var err error
	s.IdGitIntegration, err = s.insertGitIntegration()
	s.Require().NoError(err)

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"mpmbot","email":"mpmbot@test.sk","password":"kreslo"}`, s.OwnerToken)
	botLoginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"mpmbot@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, botLoginRes.StatusCode)
	var botTk struct{ Token string }
	json.NewDecoder(botLoginRes.Body).Decode(&botTk)
	var botUser model.User
	botRes := Request(s.T(), s.App, "GET", "/api/private/user", "", botTk.Token)
	json.NewDecoder(botRes.Body).Decode(&botUser)
	s.BotUserID = botUser.IdUser
	_, err = s.App.Pool.Exec(context.Background(),
		"UPDATE users.user SET is_bot = TRUE WHERE id_user = $1", s.BotUserID)
	s.Require().NoError(err)

	s.poller = agent.NewMergePoller(
		injector.GetAgentRunRepository(),
		injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(),
		injector.GetGitIntegrationRepository(),
		injector.GetIssueRepository(),
		injector.GetStateRepository(),
		injector.GetPhaseStateTransitioner(),
		injector.GetNotifier(),
	)
}

func (s *MergePollerManualSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE id_user = $1", s.BotUserID)
}

func (s *MergePollerManualSuite) createState(name string, start, final bool) int64 {
	body := fmt.Sprintf(`{"idProject":%d,"name":%q,"start":%v,"final":%v}`,
		s.IdProject, name, start, final)
	res := Request(s.T(), s.App, "POST", "/api/private/state", body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var st model.State
	json.NewDecoder(res.Body).Decode(&st)
	return st.IdState
}

func (s *MergePollerManualSuite) insertGitIntegration() (int64, error) {
	return s.insertGitIntegrationNamed("manual-merge-test", "org/repo")
}

func (s *MergePollerManualSuite) insertGitIntegrationNamed(name, repoPath string) (int64, error) {
	var idGitIntegration int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO projects.git_integration
			(id_project, name, host_type, base_url, repo_path, access_token_enc, token_nonce)
		VALUES ($1, $2, 'github', 'https://example.test', $3, $4, $5)
		RETURNING id_git_integration
	`, s.IdProject, name, repoPath, []byte("dummy-token-cipher"), []byte("dummy-nonce")).Scan(&idGitIntegration)
	return idGitIntegration, err
}

func (s *MergePollerManualSuite) mapDoneTo(idState int64) {
	body := fmt.Sprintf(`{"mappings":[{"event":"done","idState":%d}]}`, idState)
	res := Request(s.T(), s.App, "PUT",
		fmt.Sprintf("/api/private/project/%d/workflow-event-state-map", s.IdProject),
		body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *MergePollerManualSuite) clearEventMap() {
	Request(s.T(), s.App, "PUT",
		fmt.Sprintf("/api/private/project/%d/workflow-event-state-map", s.IdProject),
		`{"mappings":[]}`, s.OwnerToken)
}

func (s *MergePollerManualSuite) createIssue() model.Issue {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"manual merge test issue","description":"manual merge test issue body","estimated":0}`,
		s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	json.NewDecoder(res.Body).Decode(&iss)
	return iss
}

func (s *MergePollerManualSuite) linkMr(idIssue int64, mrId string) {
	_, err := s.App.Pool.Exec(context.Background(),
		`UPDATE issues.issue SET id_git_integration = $1, mr_id = $2 WHERE id_issue = $3`,
		s.IdGitIntegration, mrId, idIssue)
	s.Require().NoError(err)
}

func (s *MergePollerManualSuite) setIssueState(idIssue, idState int64) {
	_, err := s.App.Pool.Exec(context.Background(),
		`UPDATE issues.issue SET id_state = $1 WHERE id_issue = $2`, idState, idIssue)
	s.Require().NoError(err)
}

func (s *MergePollerManualSuite) setMrState(idIssue int64, mrState string) {
	_, err := s.App.Pool.Exec(context.Background(),
		`UPDATE issues.issue SET mr_state = $1 WHERE id_issue = $2`, mrState, idIssue)
	s.Require().NoError(err)
}

func (s *MergePollerManualSuite) loadStateAndMrState(idIssue int64) (idState *int64, mrState *string) {
	err := s.App.Pool.QueryRow(context.Background(),
		`SELECT id_state, mr_state FROM issues.issue WHERE id_issue = $1`, idIssue,
	).Scan(&idState, &mrState)
	s.Require().NoError(err)
	return
}

func (s *MergePollerManualSuite) Test_ManualMerge_AppliesDoneMapping() {
	s.mapDoneTo(s.IdStateInProg)
	defer s.clearEventMap()

	iss := s.createIssue()
	s.linkMr(iss.IdIssue, "101")

	loaded := &model.Issue{
		IdIssue:          iss.IdIssue,
		IdProject:        s.IdProject,
		IdState:          nil,
		IdGitIntegration: &s.IdGitIntegration,
		MrId:             strPtr("101"),
	}
	s.poller.HandleManualMrStatus(context.Background(), loaded, &githost.Status{State: "merged"})

	idState, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Require().NotNil(idState)
	s.Equal(s.IdStateInProg, *idState)
	s.Require().NotNil(mrState)
	s.Equal("merged", *mrState)
}

func (s *MergePollerManualSuite) Test_ManualMerge_FinalState_OnlyStamps() {
	s.mapDoneTo(s.IdStateInProg)
	defer s.clearEventMap()

	iss := s.createIssue()
	s.linkMr(iss.IdIssue, "102")
	s.setIssueState(iss.IdIssue, s.IdStateDone)

	loaded := &model.Issue{
		IdIssue:          iss.IdIssue,
		IdProject:        s.IdProject,
		IdState:          &s.IdStateDone,
		IdGitIntegration: &s.IdGitIntegration,
		MrId:             strPtr("102"),
	}
	s.poller.HandleManualMrStatus(context.Background(), loaded, &githost.Status{State: "merged"})

	idState, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Require().NotNil(idState)
	s.Equal(s.IdStateDone, *idState)
	s.Require().NotNil(mrState)
	s.Equal("merged", *mrState)
}

func (s *MergePollerManualSuite) Test_ManualClose_StampsWithoutStateChange() {
	iss := s.createIssue()
	s.linkMr(iss.IdIssue, "103")

	idStateBefore, _ := s.loadStateAndMrState(iss.IdIssue)

	loaded := &model.Issue{
		IdIssue:          iss.IdIssue,
		IdProject:        s.IdProject,
		IdState:          idStateBefore,
		IdGitIntegration: &s.IdGitIntegration,
		MrId:             strPtr("103"),
	}
	s.poller.HandleManualMrStatus(context.Background(), loaded, &githost.Status{State: "closed"})

	idStateAfter, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Equal(idStateBefore, idStateAfter)
	s.Require().NotNil(mrState)
	s.Equal("closed", *mrState)
}

func (s *MergePollerManualSuite) Test_LoadIssuesWithOpenMr_SkipsProcessedAndRunOwned() {
	ctx := context.Background()

	issA := s.createIssue()
	s.linkMr(issA.IdIssue, "201")

	issB := s.createIssue()
	s.linkMr(issB.IdIssue, "202")
	s.setMrState(issB.IdIssue, "merged")

	issC := s.createIssue()
	s.linkMr(issC.IdIssue, "203")
	var idRun int64
	err := s.App.Pool.QueryRow(ctx, `
		INSERT INTO agent.run (id_issue, id_user_bot, id_project, phase, stage_plan)
		VALUES ($1, $2, $3, 'pr_open', '{"stages":[]}')
		RETURNING id_run
	`, issC.IdIssue, s.BotUserID, s.IdProject).Scan(&idRun)
	s.Require().NoError(err)
	defer func() {
		s.App.Pool.Exec(ctx, `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck
	}()

	issD := s.createIssue()
	s.linkMr(issD.IdIssue, "204")

	issues, err := injector.GetIssueRepository().LoadIssuesWithOpenMr(ctx, 1000, 0)
	s.Require().NoError(err)

	seen := map[int64]bool{}
	for _, iss := range issues {
		seen[iss.IdIssue] = true
	}
	s.True(seen[issA.IdIssue], "issue with open manual mr must be returned")
	s.False(seen[issB.IdIssue], "issue with already-processed mr_state must be skipped")
	s.False(seen[issC.IdIssue], "issue whose mr is still owned by a pr_open run must be skipped")
	s.True(seen[issD.IdIssue], "second open manual mr issue must be returned")

	firstBatch, err := injector.GetIssueRepository().LoadIssuesWithOpenMr(ctx, 1, issA.IdIssue-1)
	s.Require().NoError(err)
	s.Require().Len(firstBatch, 1)
	s.Equal(issA.IdIssue, firstBatch[0].IdIssue)

	secondBatch, err := injector.GetIssueRepository().LoadIssuesWithOpenMr(ctx, 1000, firstBatch[0].IdIssue)
	s.Require().NoError(err)
	secondSeen := map[int64]bool{}
	for _, iss := range secondBatch {
		secondSeen[iss.IdIssue] = true
	}
	s.True(secondSeen[issD.IdIssue],
		"issue with id greater than the first batch's max must be returned by a paginated call with afterId")
}

func (s *MergePollerManualSuite) Test_ManualMerge_NoMapping_StampsWithoutStateChange() {
	s.clearEventMap()

	iss := s.createIssue()
	s.linkMr(iss.IdIssue, "401")

	idStateBefore, _ := s.loadStateAndMrState(iss.IdIssue)

	loaded := &model.Issue{
		IdIssue:          iss.IdIssue,
		IdProject:        s.IdProject,
		IdState:          idStateBefore,
		IdGitIntegration: &s.IdGitIntegration,
		MrId:             strPtr("401"),
	}
	s.poller.HandleManualMrStatus(context.Background(), loaded, &githost.Status{State: "merged"})

	idStateAfter, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Equal(idStateBefore, idStateAfter)
	s.Require().NotNil(mrState)
	s.Equal("merged", *mrState)
}

func (s *MergePollerManualSuite) Test_LinkMr_ResetsMrState() {
	ctx := context.Background()
	iss := s.createIssue()
	s.linkMr(iss.IdIssue, "501")
	s.setMrState(iss.IdIssue, "merged")

	err := injector.GetIssueRepository().LinkMr(ctx, iss.IdIssue, s.IdGitIntegration, "502")
	s.Require().NoError(err)

	_, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Nil(mrState)
}

func (s *MergePollerManualSuite) Test_UnlinkViaIntegrationDelete_ResetsMrState() {
	ctx := context.Background()

	idGitIntegration, err := s.insertGitIntegrationNamed("manual-merge-unlink-test", "org/repo-unlink")
	s.Require().NoError(err)

	iss := s.createIssue()
	_, err = s.App.Pool.Exec(ctx,
		`UPDATE issues.issue SET id_git_integration = $1, mr_id = $2, mr_state = $3 WHERE id_issue = $4`,
		idGitIntegration, "601", "merged", iss.IdIssue)
	s.Require().NoError(err)

	_, err = injector.GetGitIntegrationRepository().Delete(ctx, idGitIntegration, s.IdProject)
	s.Require().NoError(err)

	_, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Nil(mrState)
}

func (s *MergePollerManualSuite) Test_Relink_ResetsMrState() {
	iss := s.createIssue()
	s.linkMr(iss.IdIssue, "301")
	s.setMrState(iss.IdIssue, "merged")

	body := fmt.Sprintf(`{"idGitIntegration":%d,"mrId":"302"}`, s.IdGitIntegration)
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.IdProject, iss.IdIssuePublic),
		body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	_, mrState := s.loadStateAndMrState(iss.IdIssue)
	s.Nil(mrState)
}

func strPtr(v string) *string {
	return &v
}

func TestMergePollerManualSuite(t *testing.T) {
	suite.Run(t, new(MergePollerManualSuite))
}
