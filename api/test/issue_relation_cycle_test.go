package test

import "net/http"

func (s *IssueRelationSuite) Test_HierarchyCycle_IsRejected() {
	r1 := s.postRelation(s.IssueA, s.IssueB, "hierarchy", "", nil)
	s.Equal(http.StatusOK, r1.StatusCode)

	r2 := s.postRelation(s.IssueB, s.IssueC, "hierarchy", "", nil)
	s.Equal(http.StatusOK, r2.StatusCode)

	r3 := s.postRelation(s.IssueC, s.IssueA, "hierarchy", "", nil)
	s.Equal(http.StatusUnprocessableEntity, r3.StatusCode)
}

func (s *IssueRelationSuite) Test_SingleParent_IsRejected() {
	r1 := s.postRelation(s.IssueA, s.IssueB, "hierarchy", "", nil)
	s.Equal(http.StatusOK, r1.StatusCode)

	r2 := s.postRelation(s.IssueC, s.IssueB, "hierarchy", "", nil)
	s.Equal(http.StatusConflict, r2.StatusCode)
}

func (s *IssueRelationSuite) Test_ScheduleCycle_IsRejected() {
	r1 := s.postRelation(s.IssueA, s.IssueB, "schedule", "finish_to_start", nil)
	s.Equal(http.StatusOK, r1.StatusCode)

	r2 := s.postRelation(s.IssueB, s.IssueC, "schedule", "finish_to_start", nil)
	s.Equal(http.StatusOK, r2.StatusCode)

	r3 := s.postRelation(s.IssueC, s.IssueA, "schedule", "finish_to_start", nil)
	s.Equal(http.StatusUnprocessableEntity, r3.StatusCode)
}
