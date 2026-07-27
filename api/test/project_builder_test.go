package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// mockAIProvider is a test double for ai.Provider that returns a fixed response.
type mockAIProvider struct {
	response *ai.CompletionRes
	err      error
}

func (m *mockAIProvider) Complete(_ context.Context, _ ai.CompletionReq) (*ai.CompletionRes, error) {
	return m.response, m.err
}

// minimalBacklogResponse returns a valid JSON tool_use input with two issues and a schedule relation.
func minimalBacklogResponse() *ai.CompletionRes {
	payload := `{
		"summary": "A minimal test backlog.",
		"issues": [
			{
				"ref": "TASK-1",
				"title": "Design login flow",
				"description": "Design the user login flow including form validation and error handling states.",
				"estimated_hours": 20,
				"schedule_relations": []
			},
			{
				"ref": "TASK-2",
				"title": "Implement login",
				"description": "Implement the user login flow including form validation and error handling.",
				"estimated_hours": 4,
				"schedule_relations": [{"ref": "TASK-1", "type": "finish_to_start"}]
			}
		]
	}`
	return &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
}

type ProjectBuilderSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	IdProject int64
}

func (s *ProjectBuilderSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"pb-test-project"}`, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
	var proj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(res.Body).Decode(&proj)
	s.IdProject = proj.IdProject
}

func (s *ProjectBuilderSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *ProjectBuilderSuite) SetupTest() {
	// Inject the mock AI provider before each test so tests are isolated.
	injector.Clear("project-builder-controller")
	injector.Clear("ai-provider")

	// Clear the per-user generate rate-limit key (1 call / 30s) so consecutive
	// Generate tests are not throttled into 429s.
	if cache, err := injector.GetCache(); err == nil {
		keys, _ := cache.Keys(context.Background(), "ai:pb:ratelimit:*").Result()
		for _, k := range keys {
			cache.Del(context.Background(), k)
		}
	}
}

func (s *ProjectBuilderSuite) injectMockProvider(mock *mockAIProvider) {
	s.App = RebuildWithProvider(s.T(), mock)
}

func (s *ProjectBuilderSuite) generateURL() string {
	return fmt.Sprintf("/api/private/project/%d/project-builder/generate", s.IdProject)
}

func (s *ProjectBuilderSuite) acceptURL() string {
	return fmt.Sprintf("/api/private/project/%d/project-builder/accept", s.IdProject)
}

// --- Generate tests ---

func (s *ProjectBuilderSuite) Test_Generate_ValidDescription_Returns200() {
	s.injectMockProvider(&mockAIProvider{response: minimalBacklogResponse()})

	body := `{"description":"A project about building a web app with user authentication and dashboards."}`
	res := Request(s.T(), s.App, "POST", s.generateURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var resp model.ProjectBuilderGenerateRes
	json.NewDecoder(res.Body).Decode(&resp)
	s.Len(resp.Issues, 2)
	s.Equal("A minimal test backlog.", resp.Summary)
}

func (s *ProjectBuilderSuite) Test_Generate_DefaultsPopulatedOnIssues() {
	s.injectMockProvider(&mockAIProvider{response: minimalBacklogResponse()})

	// Get a valid state ID for this project.
	stateRes := Request(s.T(), s.App, "GET", "/api/private/state", "", s.Token)
	s.Equal(http.StatusOK, stateRes.StatusCode)
	var states []struct {
		IdState int64 `json:"idState"`
	}
	json.NewDecoder(stateRes.Body).Decode(&states)
	if len(states) == 0 {
		s.T().Skip("no states available — skip defaults test")
		return
	}
	idState := states[0].IdState

	body := fmt.Sprintf(`{"description":"A project about building something cool.","idState":%d}`, idState)
	res := Request(s.T(), s.App, "POST", s.generateURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var resp model.ProjectBuilderGenerateRes
	json.NewDecoder(res.Body).Decode(&resp)
	for _, iss := range resp.Issues {
		s.Require().NotNil(iss.IdState)
		s.Equal(idState, *iss.IdState)
	}
}

func (s *ProjectBuilderSuite) Test_Generate_ShortDescription_Returns400() {
	s.injectMockProvider(&mockAIProvider{response: minimalBacklogResponse()})

	body := `{"description":"too short"}`
	res := Request(s.T(), s.App, "POST", s.generateURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *ProjectBuilderSuite) Test_Generate_AIFailure_Returns503() {
	s.injectMockProvider(&mockAIProvider{err: fmt.Errorf("provider unavailable")})

	body := `{"description":"A project about building something cool with many features."}`
	res := Request(s.T(), s.App, "POST", s.generateURL(), body, s.Token)
	s.Equal(http.StatusServiceUnavailable, res.StatusCode)
}

// --- Accept tests ---

func (s *ProjectBuilderSuite) Test_Accept_ValidPayload_CreatesIssuesAndRelations() {
	// Clean up any previous issues from this test.
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1", s.IdProject)

	body := `{
		"issues": [
			{
				"ref": "A1",
				"title": "Parent issue",
				"description": "The parent issue description with sufficient length for acceptance.",
				"estimatedMinutes": 120,
				"hierarchyParentRef": ""
			},
			{
				"ref": "A2",
				"title": "Child issue",
				"description": "A child issue description that is long enough to meet requirements.",
				"estimatedMinutes": 60,
				"hierarchyParentRef": "A1"
			}
		]
	}`
	res := Request(s.T(), s.App, "POST", s.acceptURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var resp model.ProjectBuilderAcceptRes
	json.NewDecoder(res.Body).Decode(&resp)
	s.Len(resp.Issues, 2)
	for _, iss := range resp.Issues {
		s.Equal(s.IdProject, iss.IdProject)
		s.Greater(iss.IdIssuePublic, int64(0))
	}

	// Verify hierarchy relation was created.
	var relCount int
	s.App.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM issues.issue_relation WHERE id_project = $1 AND relation_type = 'hierarchy'",
		s.IdProject).Scan(&relCount)
	s.Equal(1, relCount)
}

func (s *ProjectBuilderSuite) Test_Accept_CyclicRelations_Returns422() {
	body := `{
		"issues": [
			{
				"ref": "C1",
				"title": "Issue one",
				"description": "desc",
				"estimatedMinutes": 60,
				"hierarchyParentRef": "C2"
			},
			{
				"ref": "C2",
				"title": "Issue two",
				"description": "desc",
				"estimatedMinutes": 60,
				"hierarchyParentRef": "C1"
			}
		]
	}`
	res := Request(s.T(), s.App, "POST", s.acceptURL(), body, s.Token)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func (s *ProjectBuilderSuite) Test_Accept_DanglingRef_Returns422() {
	body := `{
		"issues": [
			{
				"ref": "D1",
				"title": "Issue with bad parent",
				"description": "desc",
				"estimatedMinutes": 60,
				"hierarchyParentRef": "NONEXISTENT"
			}
		]
	}`
	res := Request(s.T(), s.App, "POST", s.acceptURL(), body, s.Token)
	s.Equal(http.StatusUnprocessableEntity, res.StatusCode)
}

func Test_RunProjectBuilderSuite(t *testing.T) {
	suite.Run(t, new(ProjectBuilderSuite))
}
