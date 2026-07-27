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

// RelationProjectScopeSuite covers: UpdateRelation must reject a relation that does
// not belong to the project in the URL, even when the caller has update rights on that
// project. Otherwise a user can edit a schedule relation of a foreign project (cross-project IDOR).
type RelationProjectScopeSuite struct {
	suite.Suite
	App        *issue.Application
	Token      string
	ProjectA   int64
	ProjectB   int64
	IssueAFrom int64 // public id, project A
	RelA       int64 // schedule relation id in project A
	RelB       int64 // schedule relation id in project B
	HierARel   int64 // non-schedule relation id in project A
	HierAFrom  int64 // public id of the hierarchy relation's from-issue
}

func (s *RelationProjectScopeSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	s.ProjectA = createProject(s.T(), s.App, s.Token, "rel-scope-project-a")
	s.ProjectB = createProject(s.T(), s.App, s.Token, "rel-scope-project-b")

	// Project A: schedule relation A1->A2 + a hierarchy relation A3->A4.
	a1 := s.createIssue(s.ProjectA, "A1")
	a2 := s.createIssue(s.ProjectA, "A2")
	a3 := s.createIssue(s.ProjectA, "A3")
	a4 := s.createIssue(s.ProjectA, "A4")
	s.IssueAFrom = a1
	s.HierAFrom = a3
	lag := int64(10)
	s.RelA = s.postRelation(s.ProjectA, a1, a2, "schedule", "finish_to_start", &lag)
	s.HierARel = s.postRelation(s.ProjectA, a3, a4, "hierarchy", "", nil)

	// Project B: schedule relation B1->B2.
	b1 := s.createIssue(s.ProjectB, "B1")
	b2 := s.createIssue(s.ProjectB, "B2")
	s.RelB = s.postRelation(s.ProjectB, b1, b2, "schedule", "finish_to_start", &lag)
}

func (s *RelationProjectScopeSuite) TearDownSuite() {
	for _, p := range []int64{s.ProjectA, s.ProjectB} {
		s.App.Pool.Exec(context.Background(),
			"DELETE FROM issues.issue_relation WHERE id_project = $1", p)
		s.App.Pool.Exec(context.Background(),
			"DELETE FROM issues.issue WHERE id_project = $1", p)
		s.App.Pool.Exec(context.Background(),
			"DELETE FROM projects.project WHERE id_project = $1", p)
	}
}

func (s *RelationProjectScopeSuite) createIssue(idProject int64, title string) int64 {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":%q,"description":"desc","estimated":0}`, title), s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var i model.Issue
	json.NewDecoder(res.Body).Decode(&i)
	return i.IdIssuePublic
}

func (s *RelationProjectScopeSuite) postRelation(idProject, from, to int64, relType, subType string, lag *int64) int64 {
	body := fmt.Sprintf(`{"idIssuePublicTo":%d,"relationType":%q`, to, relType)
	if subType != "" {
		body += fmt.Sprintf(`,"relationSubType":%q`, subType)
	}
	if lag != nil {
		body += fmt.Sprintf(`,"lagMinutes":%d`, *lag)
	}
	body += "}"
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/relation", idProject, from), body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	// Create returns both views (outbound + inbound) of the same relation.
	var views []model.ReadIssueRelationRes
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&views))
	s.Require().NotEmpty(views)
	return views[0].IdIssueRelation
}

func (s *RelationProjectScopeSuite) patchRelation(idProject, issuePublic, idRelation, lag int64) int {
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d/relation/%d", idProject, issuePublic, idRelation),
		fmt.Sprintf(`{"lagMinutes":%d}`, lag), s.Token)
	return res.StatusCode
}

func (s *RelationProjectScopeSuite) Test_01_UpdateOwnProjectRelation_OK() {
	s.Equal(http.StatusOK, s.patchRelation(s.ProjectA, s.IssueAFrom, s.RelA, 99))
}

func (s *RelationProjectScopeSuite) Test_02_UpdateForeignProjectRelation_NotFound() {
	// Project B's relation reached via Project A's URL — must be rejected.
	s.Equal(http.StatusNotFound, s.patchRelation(s.ProjectA, s.IssueAFrom, s.RelB, 99))
}

func (s *RelationProjectScopeSuite) Test_03_UpdateNonexistentRelation_NotFound() {
	s.Equal(http.StatusNotFound, s.patchRelation(s.ProjectA, s.IssueAFrom, 999999999, 99))
}

func (s *RelationProjectScopeSuite) Test_04_UpdateNonScheduleRelation_BadRequest() {
	s.Equal(http.StatusBadRequest, s.patchRelation(s.ProjectA, s.HierAFrom, s.HierARel, 99))
}

func Test_RunRelationProjectScopeSuite(t *testing.T) {
	suite.Run(t, new(RelationProjectScopeSuite))
}
