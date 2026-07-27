package test

import (
	"encoding/json"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

func (s *IssueRelationSuite) Test_GetRelations_ReturnsEmpty() {
	res := Request(s.T(), s.App, "GET", s.relURL(s.IssueA), "", s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
	var views []model.ReadIssueRelationRes
	json.NewDecoder(res.Body).Decode(&views)
	s.Len(views, 0)
}

func (s *IssueRelationSuite) Test_CreateHierarchy_Succeeds() {
	res := s.postRelation(s.IssueA, s.IssueB, "hierarchy", "", nil)
	s.Equal(http.StatusOK, res.StatusCode)

	// Create returns both views (outbound + inbound) of the same relation.
	var views []model.ReadIssueRelationRes
	json.NewDecoder(res.Body).Decode(&views)
	s.Require().NotEmpty(views)
	s.Equal("hierarchy", views[0].RelationType)
	s.Greater(views[0].IdIssueRelation, int64(0))
}

func (s *IssueRelationSuite) Test_GetRelations_ReturnsBothSides() {
	res := s.postRelation(s.IssueA, s.IssueB, "hierarchy", "", nil)
	s.Equal(http.StatusOK, res.StatusCode)

	resA := Request(s.T(), s.App, "GET", s.relURL(s.IssueA), "", s.Token)
	s.Equal(http.StatusOK, resA.StatusCode)
	var viewsA []model.ReadIssueRelationRes
	json.NewDecoder(resA.Body).Decode(&viewsA)
	s.Len(viewsA, 1)
	s.Equal("outbound", viewsA[0].Direction)
	// A is the parent; from A's side the linked issue B is the child.
	s.Equal("child", viewsA[0].Label)
	s.Equal("parent", viewsA[0].InverseLabel)

	resB := Request(s.T(), s.App, "GET", s.relURL(s.IssueB), "", s.Token)
	s.Equal(http.StatusOK, resB.StatusCode)
	var viewsB []model.ReadIssueRelationRes
	json.NewDecoder(resB.Body).Decode(&viewsB)
	s.Len(viewsB, 1)
	s.Equal("inbound", viewsB[0].Direction)
	// B is the child; from B's side the linked issue A is the parent.
	s.Equal("parent", viewsB[0].Label)
	s.Equal("child", viewsB[0].InverseLabel)
}

func (s *IssueRelationSuite) Test_CreateSchedule_WithLag_Succeeds() {
	lag := int64(120)
	res := s.postRelation(s.IssueA, s.IssueB, "schedule", "finish_to_start", &lag)
	s.Equal(http.StatusOK, res.StatusCode)

	var views []model.ReadIssueRelationRes
	json.NewDecoder(res.Body).Decode(&views)
	s.Require().NotEmpty(views)
	s.Equal("schedule", views[0].RelationType)
	s.Require().NotNil(views[0].RelationSubType)
	s.Equal("finish_to_start", *views[0].RelationSubType)
	s.Require().NotNil(views[0].LagMinutes)
	s.Equal(lag, *views[0].LagMinutes)
}

func (s *IssueRelationSuite) Test_DeleteRelation_Succeeds() {
	res := s.postRelation(s.IssueA, s.IssueC, "relates_to", "", nil)
	s.Equal(http.StatusOK, res.StatusCode)
	var created []model.ReadIssueRelationRes
	json.NewDecoder(res.Body).Decode(&created)
	s.Require().NotEmpty(created)
	idRelation := created[0].IdIssueRelation
	s.Require().Greater(idRelation, int64(0))

	delRes := Request(s.T(), s.App, "DELETE", s.relDeleteURL(s.IssueA, idRelation), "", s.Token)
	s.Equal(http.StatusOK, delRes.StatusCode)

	getRes := Request(s.T(), s.App, "GET", s.relURL(s.IssueA), "", s.Token)
	var views []model.ReadIssueRelationRes
	json.NewDecoder(getRes.Body).Decode(&views)
	for _, v := range views {
		s.NotEqual(idRelation, v.IdIssueRelation)
	}
}
