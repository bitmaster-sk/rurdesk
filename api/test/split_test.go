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

// minimalSplitResponse returns a valid CompletionRes with two proposed children.
func minimalSplitResponse() *ai.CompletionRes {
	payload := `{"children":[
		{"title":"Child task one","description":"First child task description."},
		{"title":"Child task two","description":"Second child task description."}
	]}`
	return &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
}

type SplitSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	IdProject     int64
	IdIssuePublic int64
}

func (s *SplitSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	// Create a project.
	res := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"split-test-project"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var proj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(res.Body).Decode(&proj)
	s.IdProject = proj.IdProject

	// Create an issue within the project.
	body := `{"title":"Issue to split","description":"This issue will be used in split tests."}`
	issRes := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", s.IdProject), body, s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss struct {
		IdIssuePublic int64 `json:"idIssuePublic"`
	}
	json.NewDecoder(issRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *SplitSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *SplitSuite) SetupTest() {
	// Clear the split-related injector slots so mock AI is picked up fresh each test.
	injector.Clear("split-controller")
	injector.Clear("split-service")
	injector.Clear("ai-provider")

	// Clear the rate-limit key so tests are isolated.
	cache, err := injector.GetCache()
	if err == nil {
		// Delete any existing rate limit keys for preview — pattern-based clear is not
		// available; instead delete the test user's key by scanning possible IDs.
		// A simpler approach: delete keys matching the prefix using a wildcard scan.
		keys, _ := cache.Keys(context.Background(), "split:rate:*").Result()
		for _, k := range keys {
			cache.Del(context.Background(), k)
		}
	}
}

func (s *SplitSuite) injectMockProvider(mock *mockAIProvider) {
	s.App = RebuildWithProvider(s.T(), mock)
}

func (s *SplitSuite) previewURL() string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d/split", s.IdProject, s.IdIssuePublic)
}

func (s *SplitSuite) acceptURL() string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d/split/accept", s.IdProject, s.IdIssuePublic)
}

// --- Preview tests ---

func (s *SplitSuite) Test_SplitPreview_ValidIssue_Returns200WithChildren() {
	s.injectMockProvider(&mockAIProvider{response: minimalSplitResponse()})

	res := Request(s.T(), s.App, "POST", s.previewURL(), `{}`, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var resp model.SplitPreviewRes
	json.NewDecoder(res.Body).Decode(&resp)
	s.GreaterOrEqual(len(resp.Children), 2)
	s.LessOrEqual(len(resp.Children), 6)

	// Verify no new issues were created in the DB by the preview call.
	var count int
	s.App.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM issues.issue WHERE id_project = $1", s.IdProject).Scan(&count)
	// Only the original issue should exist.
	s.Equal(1, count)
}

func (s *SplitSuite) Test_SplitPreview_RateLimit_Returns429OnSecondCall() {
	s.injectMockProvider(&mockAIProvider{response: minimalSplitResponse()})

	// First call should succeed.
	res1 := Request(s.T(), s.App, "POST", s.previewURL(), `{}`, s.Token)
	s.Equal(http.StatusOK, res1.StatusCode)

	// Second call within the rate-limit window should be rejected.
	res2 := Request(s.T(), s.App, "POST", s.previewURL(), `{}`, s.Token)
	s.Equal(http.StatusTooManyRequests, res2.StatusCode)
}

func (s *SplitSuite) Test_SplitPreview_IssueNotFound_Returns404() {
	s.injectMockProvider(&mockAIProvider{response: minimalSplitResponse()})

	url := fmt.Sprintf("/api/private/project/%d/issue/999999/split", s.IdProject)
	res := Request(s.T(), s.App, "POST", url, `{}`, s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

// --- Accept tests ---

func (s *SplitSuite) Test_SplitAccept_CreatesChildren() {
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1 AND id_issue_public != $2",
		s.IdProject, s.IdIssuePublic)

	body := `{"children":[
		{"title":"First child task","description":"Description of first child."},
		{"title":"Second child task","description":"Description of second child."}
	]}`
	res := Request(s.T(), s.App, "POST", s.acceptURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var resp model.SplitAcceptRes
	json.NewDecoder(res.Body).Decode(&resp)
	s.Len(resp.Children, 2)
	for _, child := range resp.Children {
		s.Equal(s.IdProject, child.IdProject)
		s.Greater(child.IdIssuePublic, int64(0))
	}

	// Verify children appear in the issue list.
	listRes := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/issue", s.IdProject), "", s.Token)
	s.Equal(http.StatusOK, listRes.StatusCode)
	var page model.IssuesPageRes
	json.NewDecoder(listRes.Body).Decode(&page)
	s.GreaterOrEqual(len(page.Items), 3) // original + 2 children
}

func (s *SplitSuite) Test_SplitAccept_CreatesHierarchyRelations() {
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1 AND id_issue_public != $2",
		s.IdProject, s.IdIssuePublic)

	body := `{"children":[
		{"title":"Hierarchy child one","description":"First child for hierarchy test."},
		{"title":"Hierarchy child two","description":"Second child for hierarchy test."}
	]}`
	res := Request(s.T(), s.App, "POST", s.acceptURL(), body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	// Verify hierarchy relations were created.
	relURL := fmt.Sprintf("/api/private/project/%d/issue/%d/relation", s.IdProject, s.IdIssuePublic)
	relRes := Request(s.T(), s.App, "GET", relURL, "", s.Token)
	s.Equal(http.StatusOK, relRes.StatusCode)

	var relations []model.ReadIssueRelationRes
	json.NewDecoder(relRes.Body).Decode(&relations)

	hierarchyCount := 0
	for _, rel := range relations {
		if rel.RelationType == model.RelationTypeHierarchy {
			hierarchyCount++
		}
	}
	s.Equal(2, hierarchyCount)
}

func (s *SplitSuite) Test_SplitAccept_EmptyChildren_Returns400() {
	body := `{"children":[]}`
	res := Request(s.T(), s.App, "POST", s.acceptURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func Test_RunSplitSuite(t *testing.T) {
	suite.Run(t, new(SplitSuite))
}
