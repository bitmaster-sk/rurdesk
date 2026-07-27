package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *IssueRelationSuite) Test_SelfRelation_IsRejected() {
	res := s.postRelation(s.IssueA, s.IssueA, "relates_to", "", nil)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *IssueRelationSuite) Test_CrossProject_IsRejected() {
	body := `{"idIssuePublicTo":999999,"relationType":"relates_to"}`
	res := Request(s.T(), s.App, "POST", s.relURL(s.IssueA), body, s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func (s *IssueRelationSuite) Test_DuplicateRelation_IsRejected() {
	res1 := s.postRelation(s.IssueA, s.IssueB, "relates_to", "", nil)
	s.Equal(http.StatusOK, res1.StatusCode)

	res2 := s.postRelation(s.IssueA, s.IssueB, "relates_to", "", nil)
	s.Equal(http.StatusConflict, res2.StatusCode)
}

func (s *IssueRelationSuite) Test_ReverseDuplicate_IsRejected() {
	res1 := s.postRelation(s.IssueA, s.IssueB, "duplicates", "", nil)
	s.Equal(http.StatusOK, res1.StatusCode)

	res2 := s.postRelation(s.IssueB, s.IssueA, "duplicates", "", nil)
	s.Equal(http.StatusConflict, res2.StatusCode)
}

func (s *IssueRelationSuite) Test_Schedule_MissingSubType_IsRejected() {
	body := fmt.Sprintf(`{"idIssuePublicTo":%d,"relationType":"schedule"}`, s.IssueB)
	res := Request(s.T(), s.App, "POST", s.relURL(s.IssueA), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *IssueRelationSuite) Test_ACL_IsRejected() {
	rb := `{"email":"stranger@test.sk","password":"kreslo","name":"stranger"}`
	Request(s.T(), s.App, "POST", "/api/private/admin/user", rb, Token(s.T(), s.App))
	lb := `{"email":"stranger@test.sk","password":"kreslo"}`
	lRes := Request(s.T(), s.App, "POST", "/api/public/login", lb, "")
	var tk struct {
		Token string `json:"token"`
	}
	json.NewDecoder(lRes.Body).Decode(&tk)
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'stranger@test.sk'")

	body := fmt.Sprintf(`{"idIssuePublicTo":%d,"relationType":"relates_to"}`, s.IssueB)
	res := Request(s.T(), s.App, "POST", s.relURL(s.IssueA), body, tk.Token)
	s.Equal(http.StatusForbidden, res.StatusCode)
}
