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
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/stretchr/testify/suite"
)

// countingAIProvider is a test double that counts how many times Complete is called.
type countingAIProvider struct {
	response *ai.CompletionRes
	err      error
	called   int
}

func (m *countingAIProvider) Complete(_ context.Context, _ ai.CompletionReq) (*ai.CompletionRes, error) {
	m.called++
	return m.response, m.err
}

// minimalQualityResponse returns a valid CompletionRes for a quality check.
func minimalQualityResponse() *ai.CompletionRes {
	payload := `{
		"score": 72,
		"dimensions": {"clarity": 80, "completeness": 65, "actionability": 75, "scope": 90, "metadata": 50},
		"problems": ["No acceptance criteria"],
		"suggestions": [{"type": "add_section", "explanation": "Add acceptance criteria", "new_value": "## Acceptance Criteria\n- ..."}]
	}`
	return &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
}

type QualitySuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	IdProject     int64
	IdIssuePublic int64
}

func (s *QualitySuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"quality-test-project"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var proj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(res.Body).Decode(&proj)
	s.IdProject = proj.IdProject

	body := `{"title":"Quality test issue","description":"This issue will be used in quality tests."}`
	issRes := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", s.IdProject), body, s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss struct {
		IdIssuePublic int64 `json:"idIssuePublic"`
	}
	json.NewDecoder(issRes.Body).Decode(&iss)
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *QualitySuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *QualitySuite) SetupTest() {
	injector.Clear("quality-controller")
	injector.Clear("quality-service")
	injector.Clear("ai-provider")

	cache, err := injector.GetCache()
	if err == nil {
		keys, _ := cache.Keys(context.Background(), "quality:rate:*").Result()
		for _, k := range keys {
			cache.Del(context.Background(), k)
		}
	}

	// Clear any existing quality rows for the test issue.
	s.App.Pool.Exec(context.Background(), `
		DELETE FROM issues.issue_quality
		WHERE id_issue = (
			SELECT id_issue FROM issues.issue
			WHERE id_project = $1 AND id_issue_public = $2
		)
	`, s.IdProject, s.IdIssuePublic)
}

// useProvider swaps the AI provider and rebuilds the HTTP engine. Routes are bound to
// controller instances at app-build time, so merely setting the provider mid-session does not
// reach the already-registered handlers — the quality controller/service plus the router and
// engine must be cleared and rebuilt so the routes bind to a controller backed by the mock.
func (s *QualitySuite) useProvider(p ai.Provider) {
	injector.Set("ai-provider", p)
	for _, key := range []string{"quality-service", "quality-controller", "router", "http-server"} {
		injector.Clear(key)
	}
	app, err := issue.New()
	s.Require().NoError(err)
	s.App = app
}

func (s *QualitySuite) injectMockProvider(mock *mockAIProvider) {
	s.useProvider(mock)
}

func (s *QualitySuite) previewURL() string {
	return fmt.Sprintf("/api/private/project/%d/quality", s.IdProject)
}

func (s *QualitySuite) checkURL() string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d/quality", s.IdProject, s.IdIssuePublic)
}

func (s *QualitySuite) getQualityURL() string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d/quality", s.IdProject, s.IdIssuePublic)
}

// --- Preview tests ---

func (s *QualitySuite) Test_Preview_ValidInput_Returns200WithReport() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"Fix login redirect on timeout","description":"When the session expires the user is not redirected to login."}`
	res := Request(s.T(), s.App, "POST", s.previewURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var report model.QualityCheckRes
	json.NewDecoder(res.Body).Decode(&report)
	s.Equal(72, report.Score)
	s.NotEmpty(report.Problems)
	s.NotEmpty(report.Suggestions)
}

func (s *QualitySuite) Test_Preview_EmptyTitle_Returns400() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"","description":"Some description here."}`
	res := Request(s.T(), s.App, "POST", s.previewURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *QualitySuite) Test_Preview_DoesNotPersistQualityRow() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"Preview test title","description":"Description for preview."}`
	res := Request(s.T(), s.App, "POST", s.previewURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var count int
	s.App.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM issues.issue_quality
		WHERE id_issue = (
			SELECT id_issue FROM issues.issue
			WHERE id_project = $1 AND id_issue_public = $2
		)
	`, s.IdProject, s.IdIssuePublic).Scan(&count)
	s.Equal(0, count)
}

func (s *QualitySuite) Test_Preview_RateLimit_Returns429OnSecondCall() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"Rate limit test","description":"Testing rate limiting behavior."}`
	res1 := Request(s.T(), s.App, "POST", s.previewURL(), body, s.Token)
	s.Equal(http.StatusOK, res1.StatusCode)

	res2 := Request(s.T(), s.App, "POST", s.previewURL(), body, s.Token)
	s.Equal(http.StatusTooManyRequests, res2.StatusCode)
}

// --- Check tests ---

func (s *QualitySuite) Test_Check_ValidIssue_Returns200WithReport() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"Quality test issue","description":"This issue will be used in quality tests."}`
	res := Request(s.T(), s.App, "POST", s.checkURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)

	var report model.QualityCheckRes
	json.NewDecoder(res.Body).Decode(&report)
	s.Equal(72, report.Score)
}

func (s *QualitySuite) Test_Check_PersistsScore() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"Quality test issue","description":"This issue will be used in quality tests."}`
	res := Request(s.T(), s.App, "POST", s.checkURL(), body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	// Score should appear in the issue list.
	listRes := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/issue", s.IdProject), "", s.Token)
	s.Equal(http.StatusOK, listRes.StatusCode)

	var page model.IssuesPageRes
	json.NewDecoder(listRes.Body).Decode(&page)
	s.Require().NotEmpty(page.Items)

	var found bool
	for _, iss := range page.Items {
		if iss.IdIssuePublic == s.IdIssuePublic {
			s.Require().NotNil(iss.QualityScore)
			s.Equal(72, *iss.QualityScore)
			found = true
		}
	}
	s.True(found, "test issue not found in list")
}

func (s *QualitySuite) Test_Check_CacheHit_SameHash_NoSecondAICall() {
	mock := &countingAIProvider{response: minimalQualityResponse()}
	s.useProvider(mock)

	body := `{"title":"Cache test issue","description":"Same content should hit cache on second call."}`

	res1 := Request(s.T(), s.App, "POST", s.checkURL(), body, s.Token)
	s.Require().Equal(http.StatusOK, res1.StatusCode)
	s.Equal(1, mock.called)

	// Clear rate limit to allow second call.
	cache, _ := injector.GetCache()
	keys, _ := cache.Keys(context.Background(), "quality:rate:*").Result()
	for _, k := range keys {
		cache.Del(context.Background(), k)
	}

	res2 := Request(s.T(), s.App, "POST", s.checkURL(), body, s.Token)
	s.Require().Equal(http.StatusOK, res2.StatusCode)

	s.Equal(1, mock.called, "AI should not be called again for same content")
}

// Adding metadata (here: an estimate) without changing title/description must invalidate the
// quality cache, so the re-check produces a fresh AI call instead of returning the stale
// low-metadata report.
//
// Built at the service layer with a direct mock provider: the HTTP routes are bound to a
// real provider at app setup and cannot be re-injected mid-session, so this exercises the
// cache-key behavior deterministically against the real DB.
func (s *QualitySuite) Test_Check_MetadataChange_SameContent_TriggersRecheck() {
	ctx := context.Background()

	var idIssue int64
	err := s.App.Pool.QueryRow(ctx,
		"SELECT id_issue FROM issues.issue WHERE id_project = $1 AND id_issue_public = $2",
		s.IdProject, s.IdIssuePublic).Scan(&idIssue)
	s.Require().NoError(err)

	var checkedBy int64
	err = s.App.Pool.QueryRow(ctx,
		"SELECT id_user FROM users.user WHERE email = 'test@test.sk'").Scan(&checkedBy)
	s.Require().NoError(err)

	// Clean slate: no estimate, no cached quality row.
	_, err = s.App.Pool.Exec(ctx, "UPDATE issues.issue SET estimated = 0 WHERE id_issue = $1", idIssue)
	s.Require().NoError(err)
	s.App.Pool.Exec(ctx, "DELETE FROM issues.issue_quality WHERE id_issue = $1", idIssue)
	defer s.App.Pool.Exec(ctx, "UPDATE issues.issue SET estimated = 0 WHERE id_issue = $1", idIssue)

	mock := &countingAIProvider{response: minimalQualityResponse()}
	svc := service.NewQualityService(mock,
		repository.NewQualityRepository(s.App.Pool),
		repository.NewIssueRepository(s.App.Pool))

	title, desc := "Meta cache issue", "Adding metadata must invalidate the quality cache."

	_, err = svc.Check(ctx, idIssue, title, desc, checkedBy)
	s.Require().NoError(err)
	s.Equal(1, mock.called)

	// Same content, same metadata → cache hit, no new AI call.
	_, err = svc.Check(ctx, idIssue, title, desc, checkedBy)
	s.Require().NoError(err)
	s.Equal(1, mock.called, "same content and metadata must hit the cache")

	// Add metadata (estimate) without changing title/description → cache miss → fresh AI call.
	_, err = s.App.Pool.Exec(ctx, "UPDATE issues.issue SET estimated = 3600 WHERE id_issue = $1", idIssue)
	s.Require().NoError(err)
	_, err = svc.Check(ctx, idIssue, title, desc, checkedBy)
	s.Require().NoError(err)
	s.Equal(2, mock.called, "adding metadata must invalidate the cache and trigger a fresh AI call")
}

func (s *QualitySuite) Test_Check_IssueNotFound_Returns404() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	url := fmt.Sprintf("/api/private/project/%d/issue/999999/quality", s.IdProject)
	body := `{"title":"Title","description":"Description."}`
	res := Request(s.T(), s.App, "POST", url, body, s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

// --- GetQuality tests ---

func (s *QualitySuite) Test_GetQuality_BeforeCheck_Returns404() {
	res := Request(s.T(), s.App, "GET", s.getQualityURL(), "", s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func (s *QualitySuite) Test_GetQuality_AfterCheck_Returns200WithReport() {
	s.injectMockProvider(&mockAIProvider{response: minimalQualityResponse()})

	body := `{"title":"Quality test issue","description":"This issue will be used in quality tests."}`
	checkRes := Request(s.T(), s.App, "POST", s.checkURL(), body, s.Token)
	s.Require().Equal(http.StatusOK, checkRes.StatusCode)

	getRes := Request(s.T(), s.App, "GET", s.getQualityURL(), "", s.Token)
	s.Equal(http.StatusOK, getRes.StatusCode)

	var report model.QualityCheckRes
	json.NewDecoder(getRes.Body).Decode(&report)
	s.Equal(72, report.Score)
	s.True(report.FromCache)
}

func Test_RunQualitySuite(t *testing.T) {
	suite.Run(t, new(QualitySuite))
}
