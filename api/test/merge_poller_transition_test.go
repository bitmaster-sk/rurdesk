package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/stretchr/testify/suite"
)

type MergePollerTransitionSuite struct {
	suite.Suite
	App              *issue.Application
	OwnerToken       string
	IdProject        int64
	IdStateDone      int64
	IdStateFailed    int64
	IdStateFinal     int64
	IdGitIntegration int64
	AgentUserID      int64
	poller           *agent.MergePoller
	pollerNotifier   *notify.Notifier
	gitHub           *httptest.Server
	// prState is what the fake GitHub serves, keyed by pull request id.
	prState map[string]string
	// mutable fields protected by statusMu describe the currently served open PR status.
	statusMu         sync.Mutex
	lastStatusCi     string
	lastStatusHead   string
	lastStatusReview bool
}

func (s *MergePollerTransitionSuite) SetupSuite() {
	os.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	githost.ResetEncryptionKey()

	s.App = Setup(s.T())
	s.OwnerToken = Token(s.T(), s.App)
	s.prState = map[string]string{}

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"merge-transition-project","color":"#223344"}`, s.OwnerToken)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.IdProject = prj.IdProject

	s.IdStateDone = s.createState("Merged", false, false)
	s.IdStateFailed = s.createState("Rejected", false, false)
	s.IdStateFinal = s.createState("Closed", false, true)

	s.statusMu.Lock()
	s.lastStatusCi = constants.CiStatusPending
	s.lastStatusHead = "abc123"
	s.lastStatusReview = false
	s.statusMu.Unlock()

	s.gitHub = httptest.NewServer(http.HandlerFunc(s.serveGitHub))

	intBody := fmt.Sprintf(
		`{"name":"merge-transition","hostType":"github","baseUrl":%q,"repoPath":"org/repo","accessToken":"ghp_mock_token"}`,
		s.gitHub.URL)
	intRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.IdProject), intBody, s.OwnerToken)
	s.Require().Equal(http.StatusCreated, intRes.StatusCode)
	var gitInt model.GitIntegrationRes
	json.NewDecoder(intRes.Body).Decode(&gitInt)
	s.IdGitIntegration = gitInt.IdGitIntegration

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"mptransbot","email":"mptransbot@test.sk","password":"kreslo"}`, s.OwnerToken)
	agentLoginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"mptransbot@test.sk","password":"kreslo"}`, "")
	s.Require().Equal(http.StatusOK, agentLoginRes.StatusCode)
	var agentTk struct{ Token string }
	json.NewDecoder(agentLoginRes.Body).Decode(&agentTk)
	var agentUser model.User
	agentRes := Request(s.T(), s.App, "GET", "/api/private/user", "", agentTk.Token)
	json.NewDecoder(agentRes.Body).Decode(&agentUser)
	s.AgentUserID = agentUser.IdUser
	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE users.user SET is_agent = TRUE WHERE id_user = $1", s.AgentUserID)
	s.Require().NoError(err)

	// Use a dedicated notifier for the poller in tests. The global notifier's
	// background listener immediately drains its Send channel, making it
	// impossible to assert on queued notices. A plain channel lets the test
	// drain what the poller produced.
	s.pollerNotifier = &notify.Notifier{Send: make(chan *notify.Notice, 1024)}

	s.poller = agent.NewMergePoller(
		injector.GetAgentRunRepository(),
		injector.GetAgentTaskRepository(),
		injector.GetProjectRepository(),
		injector.GetGitIntegrationRepository(),
		injector.GetIssueRepository(),
		injector.GetStateRepository(),
		injector.GetPhaseStateTransitioner(),
		s.pollerNotifier,
	)
	s.Require().NotNil(s.poller)
}

func (s *MergePollerTransitionSuite) serveGitHub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	if strings.HasSuffix(path, "/reviews") {
		fmt.Fprint(w, `[]`)
		return
	}

	if strings.HasSuffix(path, "/status") {
		s.statusMu.Lock()
		defer s.statusMu.Unlock()
		fmt.Fprintf(w, `{"state":"%s","statuses":[{"state":"%s"}]}`, s.lastStatusCi, s.lastStatusCi)
		return
	}
	if strings.HasSuffix(path, "/check-runs") {
		s.statusMu.Lock()
		defer s.statusMu.Unlock()
		fmt.Fprintf(w, `{"total_count":0,"check_runs":[]}`)
		return
	}
	if strings.Contains(path, "/commits/") {
		s.statusMu.Lock()
		defer s.statusMu.Unlock()
		fmt.Fprintf(w, `{"state":"%s","statuses":[{"state":"%s"}]}`, s.lastStatusCi, s.lastStatusCi)
		return
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	prId := segments[len(segments)-1]
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	switch s.prState[prId] {
	case "merged":
		fmt.Fprint(w, `{"state":"closed","merged":true,"head":{"sha":"def"}}`)
	case "closed":
		fmt.Fprint(w, `{"state":"closed","merged":false,"head":{"sha":"def"}}`)
	default:
		fmt.Fprintf(w, `{"state":"open","merged":false,"head":{"sha":%q},"approved":%v}`, s.lastStatusHead, s.lastStatusReview)
	}
}

// drainMrStatusNotices pulls all currently queued notices and returns those whose subject is mr_status.
func (s *MergePollerTransitionSuite) drainMrStatusNotices() []*notify.Notice {
	var out []*notify.Notice
	for {
		select {
		case n := <-s.pollerNotifier.Send:
			if n.Subject == notify.SubjectMrStatus {
				out = append(out, n)
			}
		case <-time.After(500 * time.Millisecond):
			return out
		}
	}
}

func (s *MergePollerTransitionSuite) Test_OpenManualMr_CiChange_EmitsMrStatus() {
	s.statusMu.Lock()
	s.lastStatusCi = constants.CiStatusPending
	s.lastStatusHead = "abc123"
	s.lastStatusReview = false
	s.statusMu.Unlock()

	iss := s.createIssue("open manual mr ci change")
	s.linkManualMr(iss.IdIssue, "401")

	s.Require().NoError(s.poller.PollOnce(context.Background()))
	// First poll sees the new open MR; because the in-memory cache has no
	// previous value, it broadcasts the initial state.
	firstNotices := s.drainMrStatusNotices()
	s.Require().Len(firstNotices, 1, "initial open-manual-MR poll must seed the cache by broadcasting")

	s.statusMu.Lock()
	s.lastStatusCi = constants.CiStatusSuccess
	s.lastStatusHead = "def456"
	s.statusMu.Unlock()

	s.Require().NoError(s.poller.PollOnce(context.Background()))
	// Second poll: only ciStatus and headSha changed, so exactly one update.
	notices := s.drainMrStatusNotices()
	s.Require().Len(notices, 1)
	payload, ok := notices[0].Payload.(model.MrStatusNotice)
	s.Require().True(ok)
	s.Equal(iss.IdIssue, payload.IdIssue)
	s.Equal(s.IdGitIntegration, payload.IdGitIntegration)
	s.Equal("401", payload.IdMr)
	s.Equal(constants.CiStatusSuccess, payload.CiStatus)
}

func (s *MergePollerTransitionSuite) TearDownSuite() {
	if s.gitHub != nil {
		s.gitHub.Close()
	}
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject) //nolint:errcheck
	s.App.Pool.Exec(ctx, "DELETE FROM users.user WHERE id_user = $1", s.AgentUserID)        //nolint:errcheck
}

func (s *MergePollerTransitionSuite) createState(name string, start, final bool) int64 {
	body := fmt.Sprintf(`{"idProject":%d,"name":%q,"start":%v,"final":%v}`,
		s.IdProject, name, start, final)
	res := Request(s.T(), s.App, "POST", "/api/private/state", body, s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var st model.State
	json.NewDecoder(res.Body).Decode(&st)
	return st.IdState
}

func (s *MergePollerTransitionSuite) mapEvents(mappings string) {
	res := Request(s.T(), s.App, "PUT",
		fmt.Sprintf("/api/private/project/%d/workflow-event-state-map", s.IdProject),
		fmt.Sprintf(`{"mappings":[%s]}`, mappings), s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
}

func (s *MergePollerTransitionSuite) clearEventMap() {
	Request(s.T(), s.App, "PUT",
		fmt.Sprintf("/api/private/project/%d/workflow-event-state-map", s.IdProject),
		`{"mappings":[]}`, s.OwnerToken)
}

func (s *MergePollerTransitionSuite) createIssue(title string) model.Issue {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		fmt.Sprintf(`{"title":%q,"description":"merge poller transition test issue body","estimated":0}`, title),
		s.OwnerToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	json.NewDecoder(res.Body).Decode(&iss)
	return iss
}

func (s *MergePollerTransitionSuite) insertPrOpenRun(idIssue int64, prId string) int64 {
	var idRun int64
	err := s.App.Pool.QueryRow(context.Background(), `
		INSERT INTO agent.run (id_issue, id_user_agent, id_project, phase, stage_plan, pr_id, id_git_integration)
		VALUES ($1, $2, $3, 'pr_open', '{"stages":[]}', $4, $5)
		RETURNING id_run
	`, idIssue, s.AgentUserID, s.IdProject, prId, s.IdGitIntegration).Scan(&idRun)
	s.Require().NoError(err)
	return idRun
}

func (s *MergePollerTransitionSuite) loadIssueState(idIssue int64) *int64 {
	var idState *int64
	err := s.App.Pool.QueryRow(context.Background(),
		`SELECT id_state FROM issues.issue WHERE id_issue = $1`, idIssue).Scan(&idState)
	s.Require().NoError(err)
	return idState
}

func (s *MergePollerTransitionSuite) loadRunPhase(idRun int64) string {
	var phase string
	err := s.App.Pool.QueryRow(context.Background(),
		`SELECT phase FROM agent.run WHERE id_run = $1`, idRun).Scan(&phase)
	s.Require().NoError(err)
	return phase
}

func (s *MergePollerTransitionSuite) linkManualMr(idIssue int64, mrId string) {
	_, err := s.App.Pool.Exec(context.Background(),
		`UPDATE issues.issue SET id_git_integration = $1, mr_id = $2 WHERE id_issue = $3`,
		s.IdGitIntegration, mrId, idIssue)
	s.Require().NoError(err)
}

func (s *MergePollerTransitionSuite) loadMrState(idIssue int64) *string {
	var mrState *string
	err := s.App.Pool.QueryRow(context.Background(),
		`SELECT mr_state FROM issues.issue WHERE id_issue = $1`, idIssue).Scan(&mrState)
	s.Require().NoError(err)
	return mrState
}

func (s *MergePollerTransitionSuite) Test_MergedRun_TransitionsToDone() {
	s.mapEvents(fmt.Sprintf(`{"event":"done","idState":%d}`, s.IdStateDone))
	defer s.clearEventMap()

	iss := s.createIssue("merged run issue")
	s.prState["301"] = "merged"
	idRun := s.insertPrOpenRun(iss.IdIssue, "301")
	defer s.App.Pool.Exec(context.Background(), `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck

	s.Require().NoError(s.poller.PollOnce(context.Background()))

	s.Equal("done", s.loadRunPhase(idRun))
	idState := s.loadIssueState(iss.IdIssue)
	s.Require().NotNil(idState)
	s.Equal(s.IdStateDone, *idState)
}

func (s *MergePollerTransitionSuite) Test_ClosedRun_TransitionsToFailed() {
	s.mapEvents(fmt.Sprintf(`{"event":"failed","idState":%d}`, s.IdStateFailed))
	defer s.clearEventMap()

	iss := s.createIssue("closed run issue")
	s.prState["302"] = "closed"
	idRun := s.insertPrOpenRun(iss.IdIssue, "302")
	defer s.App.Pool.Exec(context.Background(), `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck

	s.Require().NoError(s.poller.PollOnce(context.Background()))

	s.Equal("failed", s.loadRunPhase(idRun))
	idState := s.loadIssueState(iss.IdIssue)
	s.Require().NotNil(idState)
	s.Equal(s.IdStateFailed, *idState)
}

func (s *MergePollerTransitionSuite) Test_OpenRun_ChangesNothing() {
	s.mapEvents(fmt.Sprintf(`{"event":"done","idState":%d}`, s.IdStateDone))
	defer s.clearEventMap()

	iss := s.createIssue("open run issue")
	s.prState["303"] = "open"
	idRun := s.insertPrOpenRun(iss.IdIssue, "303")
	defer s.App.Pool.Exec(context.Background(), `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck

	stateBefore := s.loadIssueState(iss.IdIssue)
	s.Require().NoError(s.poller.PollOnce(context.Background()))

	s.Equal("pr_open", s.loadRunPhase(idRun))
	s.Equal(stateBefore, s.loadIssueState(iss.IdIssue))
}

func (s *MergePollerTransitionSuite) Test_ManualMr_Closed_TransitionsToFailed() {
	s.mapEvents(fmt.Sprintf(`{"event":"failed","idState":%d}`, s.IdStateFailed))
	defer s.clearEventMap()

	iss := s.createIssue("manual closed issue")
	s.prState["304"] = "closed"
	s.linkManualMr(iss.IdIssue, "304")

	s.Require().NoError(s.poller.PollOnce(context.Background()))

	idState := s.loadIssueState(iss.IdIssue)
	s.Require().NotNil(idState, "a manually linked PR closed without merging must apply the failed mapping")
	s.Equal(s.IdStateFailed, *idState)

	mrState := s.loadMrState(iss.IdIssue)
	s.Require().NotNil(mrState)
	s.Equal("closed", *mrState)
}

func (s *MergePollerTransitionSuite) Test_ManualMr_Closed_FinalState_OnlyStamps() {
	s.mapEvents(fmt.Sprintf(`{"event":"failed","idState":%d}`, s.IdStateFailed))
	defer s.clearEventMap()

	iss := s.createIssue("manual closed final issue")
	s.prState["305"] = "closed"
	s.linkManualMr(iss.IdIssue, "305")
	_, err := s.App.Pool.Exec(context.Background(),
		`UPDATE issues.issue SET id_state = $1 WHERE id_issue = $2`, s.IdStateFinal, iss.IdIssue)
	s.Require().NoError(err)

	s.Require().NoError(s.poller.PollOnce(context.Background()))

	idState := s.loadIssueState(iss.IdIssue)
	s.Require().NotNil(idState)
	s.Equal(s.IdStateFinal, *idState, "work already closed must not be reopened by a late PR outcome")

	mrState := s.loadMrState(iss.IdIssue)
	s.Require().NotNil(mrState)
	s.Equal("closed", *mrState)
}

func (s *MergePollerTransitionSuite) mergedMrStatusNotices(idIssue int64) []model.MrStatusNotice {
	var out []model.MrStatusNotice
	for _, notice := range s.drainMrStatusNotices() {
		payload, ok := notice.Payload.(model.MrStatusNotice)
		if ok && payload.IdIssue == idIssue {
			out = append(out, payload)
		}
	}
	return out
}

func (s *MergePollerTransitionSuite) Test_MergedRun_EmitsTerminalMrStatus() {
	s.mapEvents(fmt.Sprintf(`{"event":"done","idState":%d}`, s.IdStateDone))
	defer s.clearEventMap()

	iss := s.createIssue("merged run mr status issue")
	s.prState["306"] = "merged"
	s.linkManualMr(iss.IdIssue, "306")
	idRun := s.insertPrOpenRun(iss.IdIssue, "306")
	defer s.App.Pool.Exec(context.Background(), `DELETE FROM agent.run WHERE id_run = $1`, idRun) //nolint:errcheck

	s.Require().NoError(s.poller.PollOnce(context.Background()))

	notices := s.mergedMrStatusNotices(iss.IdIssue)
	s.Require().NotEmpty(notices, "a merged PR must reach the detail badge over the socket, not via a refetch")
	s.Equal("merged", notices[0].State)
	s.Equal("306", notices[0].IdMr)
	s.Equal(s.IdGitIntegration, notices[0].IdGitIntegration)
}

func (s *MergePollerTransitionSuite) Test_ManualMr_Merged_EmitsTerminalMrStatus() {
	s.mapEvents(fmt.Sprintf(`{"event":"done","idState":%d}`, s.IdStateDone))
	defer s.clearEventMap()

	iss := s.createIssue("manual merged mr status issue")
	s.prState["307"] = "merged"
	s.linkManualMr(iss.IdIssue, "307")

	s.Require().NoError(s.poller.PollOnce(context.Background()))

	notices := s.mergedMrStatusNotices(iss.IdIssue)
	s.Require().NotEmpty(notices)
	s.Equal("merged", notices[0].State)
	s.Equal("307", notices[0].IdMr)
}

func TestMergePollerTransitionSuite(t *testing.T) {
	suite.Run(t, new(MergePollerTransitionSuite))
}
