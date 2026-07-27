package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type IssueRelationSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	IdProject int64
	IssueA    int64
	IssueB    int64
	IssueC    int64
}

func (s *IssueRelationSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"relation-test-project"}`, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
	var proj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(res.Body).Decode(&proj)
	s.IdProject = proj.IdProject

	s.IssueA = s.createIssue("Issue A")
	s.IssueB = s.createIssue("Issue B")
	s.IssueC = s.createIssue("Issue C")
}

func (s *IssueRelationSuite) SetupTest() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue_relation WHERE id_project = $1", s.IdProject)
}

func (s *IssueRelationSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *IssueRelationSuite) createIssue(title string) int64 {
	body := fmt.Sprintf(`{"title":%q,"description":"desc","estimated":0}`, title)
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
	var i model.Issue
	json.NewDecoder(res.Body).Decode(&i)
	return i.IdIssuePublic
}

func (s *IssueRelationSuite) relURL(idIssuePublic int64) string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d/relation", s.IdProject, idIssuePublic)
}

func (s *IssueRelationSuite) relDeleteURL(idIssuePublic, idRelation int64) string {
	return fmt.Sprintf("/api/private/project/%d/issue/%d/relation/%d", s.IdProject, idIssuePublic, idRelation)
}

func (s *IssueRelationSuite) postRelation(fromIssue, toIssue int64, relationType, subType string, lag *int64) *http.Response {
	body := fmt.Sprintf(`{"idIssuePublicTo":%d,"relationType":%q`, toIssue, relationType)
	if subType != "" {
		body += fmt.Sprintf(`,"relationSubType":%q`, subType)
	}
	if lag != nil {
		body += fmt.Sprintf(`,"lagMinutes":%d`, *lag)
	}
	body += "}"
	return Request(s.T(), s.App, "POST", s.relURL(fromIssue), body, s.Token)
}

func Test_RunIssueRelationSuite(t *testing.T) {
	suite.Run(t, new(IssueRelationSuite))
}
