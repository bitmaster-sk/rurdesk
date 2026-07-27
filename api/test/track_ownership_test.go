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

// TrackOwnershipSuite covers: editing/deleting a time entry is allowed only for the
// entry's author or a project owner. A project member must not be able to mutate another
// member's time entries even though they share project update access.
type TrackOwnershipSuite struct {
	suite.Suite
	App         *issue.Application
	OwnerToken  string
	M1Token     string
	M2Token     string
	ProjectID   int64
	IdIssue     int64
	TrackForM1  int64 // authored by M1, used for edit cases
	TrackForDel int64 // authored by M1, used for delete cases
}

func (s *TrackOwnershipSuite) SetupSuite() {
	s.App = Setup(s.T())
	adminToken := Token(s.T(), s.App)

	s.OwnerToken = createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"towner","email":"bto@test.sk","password":"kreslo"}`)
	s.M1Token = createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"track1","email":"btrack1@test.sk","password":"kreslo"}`)
	s.M2Token = createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"track2","email":"btrack2@test.sk","password":"kreslo"}`)
	m1ID := idOfUser(s.T(), s.App, adminToken, "btrack1@test.sk")
	m2ID := idOfUser(s.T(), s.App, adminToken, "btrack2@test.sk")

	s.ProjectID = createProject(s.T(), s.App, s.OwnerToken, "track-ownership-test-project")

	// Add both as members.
	for _, idUser := range []int64{m1ID, m2ID} {
		res := Request(s.T(), s.App, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, idUser), s.OwnerToken)
		s.Require().Equal(http.StatusOK, res.StatusCode)
	}

	// Issue to log time against.
	issRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.ProjectID),
		`{"title":"Track ownership issue","description":"Issue for track ownership tests."}`,
		s.OwnerToken)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issRes.Body).Decode(&iss)
	s.IdIssue = iss.IdIssue

	// M1 authors two tracks.
	s.TrackForM1 = s.createTrack(s.M1Token)
	s.TrackForDel = s.createTrack(s.M1Token)
}

func (s *TrackOwnershipSuite) createTrack(token string) int64 {
	res := Request(s.T(), s.App, "POST", "/api/private/track",
		fmt.Sprintf(`{"idIssue":%d,"tracked":3600}`, s.IdIssue), token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var t model.Track
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&t))
	return t.IdTrack
}

func (s *TrackOwnershipSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.track WHERE id_issue = $1", s.IdIssue)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1", s.ProjectID)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.ProjectID)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email IN ('bto@test.sk','btrack1@test.sk','btrack2@test.sk')")
}

func (s *TrackOwnershipSuite) editTrack(token string, idTrack int64) int {
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/track/%d", idTrack), `{"tracked":1800}`, token)
	return res.StatusCode
}

func (s *TrackOwnershipSuite) deleteTrack(token string, idTrack int64) int {
	res := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/track/%d", idTrack), "", token)
	return res.StatusCode
}

func (s *TrackOwnershipSuite) Test_01_Author_EditsOwn_OK() {
	s.Equal(http.StatusOK, s.editTrack(s.M1Token, s.TrackForM1))
}

func (s *TrackOwnershipSuite) Test_02_OtherMember_EditsForeign_Forbidden() {
	s.Equal(http.StatusForbidden, s.editTrack(s.M2Token, s.TrackForM1))
}

func (s *TrackOwnershipSuite) Test_03_OtherMember_DeletesForeign_Forbidden() {
	s.Equal(http.StatusForbidden, s.deleteTrack(s.M2Token, s.TrackForDel))
}

func (s *TrackOwnershipSuite) Test_04_Owner_EditsForeign_OK() {
	s.Equal(http.StatusOK, s.editTrack(s.OwnerToken, s.TrackForM1))
}

func (s *TrackOwnershipSuite) Test_05_Owner_DeletesForeign_OK() {
	s.Equal(http.StatusOK, s.deleteTrack(s.OwnerToken, s.TrackForDel))
}

func Test_RunTrackOwnershipSuite(t *testing.T) {
	suite.Run(t, new(TrackOwnershipSuite))
}
